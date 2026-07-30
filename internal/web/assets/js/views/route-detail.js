// Route detail page (issue #68): a URL-addressable page at #/routes/{key}
// composed entirely client-side over the already-loaded Project Model — no new
// server endpoint, the same posture as the Model detail page (issue #51).
//
// It shows ONE route's complete binding: method + URI, the resolved Controller
// and Action, the Middleware applied to it (each linked to its node), the Auth
// state, and any bound FormRequest. composeRouteDetail is the pure compose
// function at its heart, kept free of the DOM so it is unit-testable and can't
// drift from the markup that consumes it.

import { $, escapeHtml } from "../dom.js";
import { entityChip } from "../chip.js";

// routeKey addresses one route within the Routes view. A route is identified by
// method + URI (there is no id in the contract), and the key has to survive a
// single URL path segment — but the raw URI is full of slashes, which would
// otherwise split the segment.
//
// The slash is therefore swapped for a "~" rather than percent-encoded. Both
// round-trip correctly, but percent-encoding here means the key still contains
// "%2F" AFTER the router's one decode layer, so the href ends up double-encoded
// ("GET%20%252Fposts") — correct, yet unreadable in the address bar and
// unpleasant to copy-paste. "~" is unreserved in a URL path, so the key stays
// legible: "GET ~posts~{post}".
//
// "~" is not otherwise legal in a Laravel route URI, so the mapping is
// injective in practice; encodeURIComponent at the link site still handles the
// space and the braces.
//
// Note the key is NOT unique in general: the contract permits two entries with
// the same method+URI (the diff module documents exactly this — route keys
// aren't unique). Composing picks the FIRST match, which is why the page also
// shows the binding rather than relying on the key alone to disambiguate.
export function routeKey(route) {
  const method = String((route && route.method) || "").toUpperCase();
  const uri = String((route && route.uri) || "").replace(/\//g, "~");
  return method + " " + uri;
}

// composeRouteDetail derives the detail view-model for the route with the given
// key. It never throws on an unknown key or an absent section — an unmatched key
// yields a view-model whose `found` is false, so the renderer shows a defined
// not-found state rather than a blank page.
export function composeRouteDetail(key, projectModel) {
  const pm = projectModel || {};
  const routes = pm.routes || [];
  const self = routes.find((r) => routeKey(r) === key) || null;

  if (!self) {
    return {
      key,
      found: false,
      method: "",
      uri: "",
      auth: "",
      dead: false,
      controller: null,
      middleware: [],
      direct: [],
      viaGroup: [],
      formRequest: null,
    };
  }

  const middleware = middlewareFor(self, pm.middlewares || []);
  const { direct, viaGroup } = partitionByProvenance(middleware);

  return {
    key,
    found: true,
    method: self.method || "",
    uri: self.uri || "",
    name: self.name || "",
    // Auth comes from the CONTRACT, computed server-side by the classifier
    // (issue #50, ADR 0008). Re-deriving it from the middleware list here would
    // let this page and the Auth view disagree about the same route.
    auth: self.auth || "",
    dead: isDead(self, pm.dead_routes || []),
    controller: controllerFor(self),
    middleware,
    direct,
    viaGroup,
    formRequest: formRequestFor(self, pm.form_requests || []),
  };
}

// controllerFor composes the route's controller binding. Route.Controller is the
// reference recorded VERBATIM at the route site and Route.FQN is the phase-two
// resolution (issue #63); both are surfaced, because the verbatim string is what
// the source literally wrote while the FQN is what it actually means. An absent
// FQN means resolution failed — the state also recorded as a DeadRoute.
function controllerFor(route) {
  const fqn = route.fqn || "";
  return {
    reference: route.controller || "",
    fqn,
    action: route.action || "",
    resolved: fqn !== "",
  };
}

// isDead reports whether this route is among the analysis's dead routes —
// matched on method + URI, the same identity routeKey uses.
function isDead(route, deadRoutes) {
  return deadRoutes.some((d) => d.method === route.method && d.uri === route.uri);
}

// middlewareFor links each APPLIED middleware name to its Middleware node.
//
// Applied names arrive parameterized — "auth:sanctum", "throttle:api" — while a
// node's alias is the bare "auth"/"throttle", so the match is on the BASE alias
// with the parameter split off. Matching on the full applied string would fail
// to link exactly the routes that carry the most information.
//
// The parameter is preserved rather than discarded: "auth:sanctum" says WHICH
// guard authenticates the route, and dropping it would erase that. A name with
// no matching node is kept and marked unresolved — an applied-but-undeclared
// middleware is real information about the project (ADR 0002), not noise to
// silently drop.
//
// Order follows the route's applied order; determinism is a hard convention.
function middlewareFor(route, nodes) {
  const byAlias = new Map(nodes.map((n) => [n.alias, n]));
  // The set of group names this route applies directly, needed to decide
  // provenance below. A group is applied by naming it ("web", "api").
  const appliedNames = new Set((route.middleware || []).map((m) => baseAlias(m)));

  return (route.middleware || []).map((applied) => {
    const alias = baseAlias(applied);
    const node = byAlias.get(alias) || null;
    const groups = (node && node.groups) || [];
    // The group this middleware arrived through: one it belongs to that is ALSO
    // applied on this route. Without that second condition a middleware would
    // be reported as group-applied merely for belonging to a group the route
    // never applies.
    const group = groups.find((g) => appliedNames.has(g)) || "";
    return {
      applied,
      alias,
      parameter: parameterOf(applied),
      resolved: node !== null,
      class: (node && node.class) || "",
      origin: (node && node.origin) || "unknown",
      groups,
      group,
    };
  });
}

// partitionByProvenance splits the applied middleware into the two relations the
// page shows separately: those applied DIRECTLY at the route site, and those
// inherited VIA a group the route applies.
//
// The contract records no per-name provenance — Route.Middleware is a flat list
// that already merges both (see the field doc in internal/model/route.go) — so
// the split is derived from the middleware node's own Groups. A name whose node
// belongs to a group this route also applies came in through that group;
// everything else was applied directly.
//
// A group name itself ("web") lands in `direct`: applying it IS a direct act at
// the route site, even though its effect is to pull a group in. Every applied
// name lands in exactly one list — the two are a partition, never overlapping
// and never dropping a name.
function partitionByProvenance(middleware) {
  const direct = [];
  const viaGroup = [];
  middleware.forEach((m) => (m.group ? viaGroup : direct).push(m));
  return { direct, viaGroup };
}

// baseAlias strips a middleware parameter: "auth:sanctum" → "auth". Laravel's
// parameter syntax is a single colon, and everything after the FIRST colon is
// the argument list ("throttle:60,1"), so the split is on the first colon only.
function baseAlias(applied) {
  const s = String(applied || "");
  const colon = s.indexOf(":");
  return colon === -1 ? s : s.slice(0, colon);
}

// parameterOf returns a middleware's argument string, or "" when it takes none
// — "" rather than undefined so the renderer tests one falsy value.
function parameterOf(applied) {
  const s = String(applied || "");
  const colon = s.indexOf(":");
  return colon === -1 ? "" : s.slice(colon + 1);
}

// formRequestFor resolves the route's bound FormRequest to its full object, so
// the validated fields are available on the page. A binding naming a class that
// was never extracted still yields an object marked unresolved: the binding is a
// real fact even when the class wasn't found, and reporting "no form request"
// instead would be a different — and false — claim.
function formRequestFor(route, formRequests) {
  const ref = route.form_request || "";
  if (!ref) return null;
  const short = lastSegment(ref);
  const fr =
    formRequests.find((f) => f.fqn === ref) ||
    formRequests.find((f) => f.name === short) ||
    null;
  if (!fr) {
    return { name: short, fqn: ref, fields: [], resolved: false };
  }
  return { ...fr, resolved: true };
}

// lastSegment reduces a fully-qualified class reference to its final name
// segment, so a match works whether the contract carried the FQN or a short name.
function lastSegment(ref) {
  const s = String(ref || "");
  const slash = s.lastIndexOf("\\");
  return slash === -1 ? s : s.slice(slash + 1);
}

// AUTH_LABEL turns the contract's auth state into the page's wording. The states
// are the classifier's (issue #50); "unknown" is a real, distinct answer — the
// route applies middleware we could not classify — and must not be presented as
// "unauthenticated", which would be a stronger claim than the analysis supports.
const AUTH_LABEL = {
  authenticated: "Authenticated",
  unauthenticated: "Unauthenticated",
  unknown: "Unknown",
};

// AUTH_PILL maps the auth state to its pill variant. Per DESIGN.md §1 the
// non-color channel is the pill's own TEXT LABEL, so the state is fully
// readable with the hue removed.
const AUTH_PILL = {
  authenticated: "ok",
  unauthenticated: "warn",
  unknown: "dim",
};

// routeDetailHtml is the pure per-page template: composed view-model in, HTML
// string out. Pure (no DOM) so the markup contract is unit-testable, the same
// split as modelDetailHtml.
export function routeDetailHtml(d) {
  if (!d || !d.found) {
    return (
      '<div class="detail-page">' +
      '<div class="detail-head"><h2 tabindex="-1">Route not found</h2></div>' +
      '<div class="empty-note">No route matching this link was found in this analysis. ' +
      '<a href="#/routes" data-view="routes">Back to all routes</a>.</div>' +
      "</div>"
    );
  }

  const method = escapeHtml(d.method);
  // "m-<METHOD>" uppercase is what the color rules key on (.m-GET, .m-POST …),
  // matching the routes table; a lowercased variant matches no rule at all.
  const heading =
    '<span class="method m-' + method + '">' + method + "</span> " +
    '<span class="uri">' + escapeHtml(d.uri) + "</span>";

  return (
    '<div class="detail-page">' +
    '<div class="detail-head"><h2 tabindex="-1">' + heading + "</h2>" +
    (d.name ? '<div class="empty-note">name: ' + escapeHtml(d.name) + "</div>" : "") +
    "</div>" +
    controllerSection(d) +
    authSection(d) +
    middlewareSection(d) +
    formRequestSection(d) +
    "</div>"
  );
}

// controllerSection shows the binding the route dispatches to: the resolved FQN
// (chipped through to the controller's own page) plus the action. An unresolved
// controller says so plainly and names the verbatim reference the source wrote,
// which is the string a reader greps for.
function controllerSection(d) {
  const c = d.controller || {};
  if (!c.resolved) {
    return (
      '<section class="detail-section"><div class="subhead">Controller</div>' +
      '<div class="empty-note">' +
      '<span class="status-dot danger" aria-hidden="true"></span> Unresolved: the reference ' +
      '<code>' + escapeHtml(c.reference || "") + "</code> could not be resolved to a class" +
      (d.dead ? " — this route is reported dead." : ".") +
      "</div></section>"
    );
  }
  return (
    '<section class="detail-section"><div class="subhead">Controller</div>' +
    '<div class="route-row">' +
    entityChip({ kind: "controller", name: c.fqn }, c.fqn + "::" + c.action) +
    ' <span class="col-type">' + escapeHtml(c.action) + "</span></div>" +
    (c.reference && c.reference !== c.fqn
      ? '<div class="empty-note">written at the route site as <code>' + escapeHtml(c.reference) + "</code></div>"
      : "") +
    "</section>"
  );
}

// authSection shows the contract's auth verdict for this route.
function authSection(d) {
  const label = AUTH_LABEL[d.auth] || d.auth || "Unknown";
  const variant = AUTH_PILL[d.auth] || "dim";
  return (
    '<section class="detail-section"><div class="subhead">Auth</div>' +
    '<span class="pill ' + variant + '">' + escapeHtml(label) + "</span></section>"
  );
}

// middlewareSection lists the applied middleware as TWO labeled relations —
// directly applied, and inherited via a group — never merged into one list. The
// distinction is what tells a reader whether removing a group would drop the
// protection, so collapsing it would lose the point of the section.
function middlewareSection(d) {
  if ((d.middleware || []).length === 0) {
    return (
      '<section class="detail-section"><div class="subhead">Middleware</div>' +
      '<div class="empty-note">No middleware is applied to this route.</div></section>'
    );
  }
  return (
    '<section class="detail-section"><div class="subhead">Middleware &mdash; applied directly</div>' +
    (d.direct.length ? d.direct.map(middlewareRow).join("") : '<div class="empty-note">None applied directly at the route site.</div>') +
    "</section>" +
    '<section class="detail-section"><div class="subhead">Middleware &mdash; via a group</div>' +
    (d.viaGroup.length
      ? d.viaGroup.map(middlewareRow).join("")
      : '<div class="empty-note">None inherited from an applied group.</div>') +
    "</section>"
  );
}

// middlewareRow renders one applied middleware: its alias, the parameter it
// carries (which guard, which rate limit), the group it arrived through when
// any, and its resolved class. An unresolved name is flagged rather than hidden.
function middlewareRow(m) {
  const param = m.parameter ? '<span class="pill dim">' + escapeHtml(m.parameter) + "</span>" : "";
  const group = m.group ? '<span class="pill warn">via ' + escapeHtml(m.group) + "</span>" : "";
  const unresolved = !m.resolved
    ? '<span class="pill warn">not declared</span>'
    : "";
  const cls = m.class ? '<span class="col-type">' + escapeHtml(m.class) + "</span>" : "";
  return (
    '<div class="col-row overlay-row">' +
    '<span class="col-name">' + escapeHtml(m.alias) + "</span> " +
    param + group + unresolved + " " + cls +
    "</div>"
  );
}

// formRequestSection shows the FormRequest bound to the route's action and the
// fields it validates — the payload contract for this endpoint in one place.
function formRequestSection(d) {
  const fr = d.formRequest;
  if (!fr) {
    return (
      '<section class="detail-section"><div class="subhead">Form request</div>' +
      '<div class="empty-note">This route\'s action takes no FormRequest parameter.</div></section>'
    );
  }
  if (!fr.resolved) {
    return (
      '<section class="detail-section"><div class="subhead">Form request</div>' +
      '<div class="empty-note">Bound to <code>' + escapeHtml(fr.name) +
      "</code>, but that class was not found in this analysis.</div></section>"
    );
  }
  const fields = (fr.fields || []).length
    ? '<div class="chips">' +
      (fr.fields || []).map((f) => '<span class="chip">' + escapeHtml(f.name || "") + "</span>").join("") +
      "</div>"
    : '<div class="empty-note">No fields parsed.</div>';
  return (
    '<section class="detail-section"><div class="subhead">Form request</div>' +
    '<div class="fr-row"><span class="fr-name">' + escapeHtml(fr.name || "") + "</span>" +
    fields +
    "</div></section>"
  );
}

// renderRouteDetail is the thin DOM wrapper, mirroring renderModelDetail: it
// composes the view-model, renders it into #route-detail-body, and toggles the
// routes panel between its table and the detail page. A null/empty key closes
// the detail and restores the table — the state a plain #/routes route lands in.
export function renderRouteDetail(key, projectModel) {
  const table = $("#routes-list");
  const body = $("#route-detail-body");
  if (!body) return;
  if (!key) {
    body.hidden = true;
    body.innerHTML = "";
    if (table) table.hidden = false;
    return;
  }
  body.innerHTML = routeDetailHtml(composeRouteDetail(key, projectModel));
  body.hidden = false;
  if (table) table.hidden = true;
  // Focus is NOT moved here: activateView (main.js) is the single focus
  // authority and focuses the now-visible detail heading right after this.
}
