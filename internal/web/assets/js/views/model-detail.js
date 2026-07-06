// Model detail page (issue #51): a URL-addressable page at #/models/{name}
// composed entirely client-side over the already-loaded Project Model — no new
// server endpoint. It shows a model's relations in both directions, the routes
// that touch it, the form requests validating its writes, its table schema and
// indexes, and the findings scoped to it, every reference an entity-chip so the
// user keeps following the thread without dead ends.
//
// composeModelDetail is the pure compose function at the page's heart: model
// name + whole Project Model in, everything the page renders out. Kept pure (no
// DOM) so it is unit-testable on its own and can't drift from the markup that
// consumes it — the same split as findingsList in findings.js.

import { $, escapeHtml } from "../dom.js";
import { entityChip } from "../chip.js";
import { severityFor } from "./findings.js";

// DOT_CLASS bridges the contract severity to the status-dot CSS class, matching
// findings.js exactly (blocker → the red danger dot; warn/info → the amber warn
// dot) so a finding reads the same on the detail page as in the Findings view.
const DOT_CLASS = { blocker: "danger", warn: "warn", info: "warn" };
function dotClass(severity) {
  return DOT_CLASS[severity] || "warn";
}

// modelDetailHtml is the pure per-page template: the composed detail view-model
// (composeModelDetail) in, the page's HTML string out. Every model / table /
// controller reference is an entity-chip (degrading to inert is-plain text when
// the target is absent), so the user keeps following the thread without dead
// ends. Chips carry their own in-app target via entityChip (no router params
// needed — main.js's delegated click handler injects the current path at click
// time), so this template needs nothing but the view-model. Pure (no DOM) so the
// markup contract is unit-testable, the same split as findingRowHtml.
export function modelDetailHtml(d) {
  if (!d || !d.found) {
    // A defined not-found state, never a blank page: the name the user asked
    // for, said plainly, with a calm route back to the full list.
    const name = escapeHtml((d && d.name) || "");
    return (
      '<div class="detail-page">' +
      '<div class="detail-head"><h2 tabindex="-1">Model not found</h2></div>' +
      '<div class="empty-note">No model named "' + name + '" was found in this analysis. ' +
      '<a href="#/models" data-view="models">Back to all models</a>.</div>' +
      "</div>"
    );
  }

  return (
    '<div class="detail-page">' +
    '<div class="detail-head"><h2 tabindex="-1">' + escapeHtml(d.name) + "</h2></div>" +
    relationsSection(d) +
    routesSection(d) +
    formRequestsSection(d) +
    tableSection(d) +
    findingsSection(d) +
    "</div>"
  );
}

// renderModelDetail is the thin DOM wrapper (mirrors renderModels/renderFindings):
// it composes the detail view-model, renders it into #model-detail-body, and
// toggles the models panel between its flat list and the detail page. Passing a
// null/empty name closes the detail and restores the list — the state a plain
// #/models route (no detail segment) lands in. Kept thin: all logic lives in the
// pure composeModelDetail / modelDetailHtml above, which the jstests cover.
export function renderModelDetail(name, projectModel) {
  const list = $("#models-list");
  const body = $("#model-detail-body");
  if (!name) {
    body.hidden = true;
    body.innerHTML = "";
    if (list) list.hidden = false;
    return;
  }
  const d = composeModelDetail(name, projectModel);
  body.innerHTML = modelDetailHtml(d);
  body.hidden = false;
  if (list) list.hidden = true;
  // Focus is NOT moved here: activateView (main.js) runs right after this and is
  // the single focus authority — it detects the now-visible detail body and
  // focuses its heading, so the screen reader announces the model just opened.
  // Setting focus here too would only be immediately overridden, a wasted flash.
}

// relationsSection lists forward and reverse relations. Forward names the method
// and chips the target model; reverse names the source model (chipped) and the
// method on it that reaches this model. Each direction gets a calm empty note.
function relationsSection(d) {
  const forward = d.forwardRelations || [];
  const reverse = d.reverseRelations || [];

  const forwardRows = forward.length
    ? forward
        .map(
          (r) =>
            '<div class="rel-row"><span class="rel-kind">' + escapeHtml(r.kind || "") + "</span> " +
            escapeHtml(r.method || "") + " &rarr; " +
            entityChip({ kind: "model", name: r.target }) + "</div>"
        )
        .join("")
    : '<div class="empty-note">No outbound relations declared.</div>';

  const reverseRows = reverse.length
    ? reverse
        .map(
          (r) =>
            '<div class="rel-row">' + entityChip({ kind: "model", name: r.from }) +
            ' &rarr; ' + escapeHtml(r.method || "") +
            ' <span class="rel-kind">' + escapeHtml(r.kind || "") + "</span></div>"
        )
        .join("")
    : '<div class="empty-note">No inbound relations point at this model.</div>';

  return (
    '<section class="detail-section"><div class="subhead">Relations &mdash; outbound</div>' + forwardRows + "</section>" +
    '<section class="detail-section"><div class="subhead">Relations &mdash; inbound</div>' + reverseRows + "</section>"
  );
}

// MATCH_LABEL turns the internal match kind into a short, honest confidence tag
// so the user can see WHY a route was attributed to this model (a controller-name
// match is near-certain; a URI-segment guess is a heuristic).
const MATCH_LABEL = {
  controller: "controller",
  form_request: "form request",
  uri: "uri match",
};

// routesSection lists the routes that touch the model, each with its method,
// URI, a controller entity-chip, and the confidence tag for why it matched.
function routesSection(d) {
  const routes = d.routes || [];
  const rows = routes.length
    ? routes
        .map(
          (r) =>
            '<div class="route-row">' +
            '<span class="method ' + escapeHtml((r.method || "").toLowerCase()) + '">' + escapeHtml(r.method || "") + "</span> " +
            '<span class="uri">' + escapeHtml(r.uri || "") + "</span> " +
            entityChip({ kind: "controller", name: r.controller }, r.controller ? r.controller + "::" + (r.action || "") : "") +
            ' <span class="match-tag">' + escapeHtml(MATCH_LABEL[r.match] || r.match || "") + "</span></div>"
        )
        .join("")
    : '<div class="empty-note">No routes were correlated to this model.</div>';
  return '<section class="detail-section"><div class="subhead">Routes</div>' + rows + "</section>";
}

// formRequestsSection lists the FormRequests validating the model's writes, each
// with its parsed field names. FormRequests have no dedicated view, so the class
// name is plain text (not a chip) — the fields ARE the payload here.
function formRequestsSection(d) {
  const frs = d.formRequests || [];
  const rows = frs.length
    ? frs
        .map((fr) => {
          const fields = (fr.fields || []).map((f) => '<span class="chip">' + escapeHtml(f.name || "") + "</span>").join("");
          return (
            '<div class="fr-row"><span class="fr-name">' + escapeHtml(fr.name || "") + "</span>" +
            (fields ? '<div class="chips">' + fields + "</div>" : '<div class="empty-note">No fields parsed.</div>') +
            "</div>"
          );
        })
        .join("")
    : '<div class="empty-note">No form requests validate this model\'s writes.</div>';
  return '<section class="detail-section"><div class="subhead">Form requests</div>' + rows + "</section>";
}

// tableSection renders the model's table schema: the table name as a chip to the
// ER view, its columns, and its indexes. An absent table gets the same calm
// "not found in schema" note models.js gives.
function tableSection(d) {
  const t = d.table;
  if (!t) {
    return '<section class="detail-section"><div class="subhead">Table</div>' +
      '<div class="empty-note">This model\'s table was not found in the schema.</div></section>';
  }
  const columns = (t.columns || []).length
    ? (t.columns || [])
        .map((c) => {
          const flags = [];
          if (c.is_primary_key) flags.push("PK");
          if (c.is_foreign_key) flags.push("FK");
          if (c.nullable) flags.push("nullable");
          const tag = flags.length ? ' <span class="col-flags">' + escapeHtml(flags.join(", ")) + "</span>" : "";
          return '<div class="col-row"><span class="col-name">' + escapeHtml(c.name || "") + "</span> " +
            '<span class="col-type">' + escapeHtml(c.type || "") + "</span>" + tag + "</div>";
        })
        .join("")
    : '<div class="empty-note">No columns.</div>';
  const indexes = (t.indexes || []).length
    ? (t.indexes || [])
        .map((idx) => {
          const kind = idx.unique ? "UNIQUE" : "INDEX";
          const name = idx.name ? escapeHtml(idx.name) + " " : "";
          const cols = (idx.columns || []).map((c) => escapeHtml(c)).join(", ");
          return '<div class="idx-row"><span class="idx-kind ' + (idx.unique ? "unique" : "plain") + '">' + kind + "</span> " +
            name + '<span class="cols">(' + cols + ")</span></div>";
        })
        .join("")
    : '<div class="empty-note">No indexes declared.</div>';
  return (
    '<section class="detail-section"><div class="subhead">Table &mdash; ' +
    entityChip({ kind: "table", name: t.name }) + "</div>" +
    '<div class="col-list">' + columns + "</div>" +
    '<div class="idx-list">' + indexes + "</div></section>"
  );
}

// findingsSection lists the findings scoped to this model, each led by its
// severity status dot (blocker → danger, warn/info → warn), matching the Findings
// view. An all-clear model says so plainly.
function findingsSection(d) {
  const findings = d.findings || [];
  const rows = findings.length
    ? findings
        .map((f) => {
          const dot = dotClass(f.severity);
          return (
            '<div class="finding' + (dot === "danger" ? " dead" : "") + '">' +
            '<div class="h"><span class="status-dot ' + dot + '" aria-hidden="true"></span>' +
            escapeHtml(f.title || "") + " " + escapeHtml(f.subject || "") + "</div>" +
            '<div class="r">' + escapeHtml(f.reason || "") + "</div></div>"
          );
        })
        .join("")
    : '<div class="empty-note">No findings scoped to this model. All clear.</div>';
  return '<section class="detail-section"><div class="subhead">Findings</div>' + rows + "</section>";
}

// composeModelDetail derives the detail view-model for the named model from the
// whole Project Model. It never throws on a missing model or absent sections —
// an unknown name yields a view-model whose `found` is false, so the renderer
// shows a defined not-found state rather than a blank page.
export function composeModelDetail(name, projectModel) {
  const pm = projectModel || {};
  const models = pm.models || [];
  const schemas = pm.schemas || [];
  const self = models.find((m) => m.name === name) || null;
  const table = self && self.table ? schemas.find((t) => t.name === self.table) || null : null;
  const routes = owningRoutesFor(name, self, pm.routes || []);

  return {
    name,
    found: self !== null,
    forwardRelations: self ? self.relationships || [] : [],
    reverseRelations: reverseRelationsFor(name, models),
    table,
    findings: findingsFor(name, self, pm.disagreements || []),
    routes,
    formRequests: formRequestsFor(routes, pm.form_requests || []),
  };
}

// formRequestsFor composes the FormRequests that validate the model's writes:
// the FormRequest linked to each owning route (Route.form_request → the
// matching FormRequest object), deduped and in owning-route order. Resolving to
// the full object (not just the name) makes each request's Fields available on
// the page, so validation coverage is auditable in one place. A route's
// form_request that resolves to no known FormRequest is skipped, never a dangling
// name.
function formRequestsFor(routes, formRequests) {
  const seen = new Set();
  const result = [];
  routes.forEach((r) => {
    if (!r.form_request) return;
    const short = lastSegment(r.form_request);
    if (seen.has(short)) return;
    const fr =
      formRequests.find((f) => f.fqn === r.form_request) ||
      formRequests.find((f) => f.name === short) ||
      null;
    if (fr) {
      seen.add(short);
      result.push(fr);
    }
  });
  return result;
}

// MATCH_RANK orders the route→model correlation signals by confidence, strongest
// first. There is no direct route→model edge in the contract, so the page casts
// a wide net (owner's choice) but leads with the strongest signal and demotes
// the weakest — honoring precision-over-coverage (ADR 0002) without dropping the
// coverage: a URI-segment guess never outranks a controller-name match.
const MATCH_RANK = { controller: 0, form_request: 1, uri: 2 };

// owningRoutesFor finds the routes that "touch" a model via three signals:
//   controller   — the route's controller is {Model}Controller (Laravel's
//                  resource convention); the strongest, near-certain signal.
//   form_request — the route's linked FormRequest name references the model
//                  (StorePostRequest → Post); strong, it validates the writes.
//   uri          — a URI path segment equals the model's table (/posts → Post);
//                  the weakest, a heuristic guess.
// A route matched by more than one signal appears ONCE at its strongest rank.
// The result is ordered strongest-match-first; within a rank, source order.
function owningRoutesFor(name, self, routes) {
  const table = self ? self.table : "";
  const matched = [];
  routes.forEach((r) => {
    const match = matchKindFor(r, name, table);
    if (match) matched.push({ ...r, match });
  });
  // Stable sort by rank: equal ranks keep source order ([].sort is stable).
  return matched.sort((a, b) => MATCH_RANK[a.match] - MATCH_RANK[b.match]);
}

// matchKindFor returns the STRONGEST match kind between a route and a model, or
// null when the route does not touch the model. Checked strongest-first so a
// route touched by several signals is attributed to its most certain one.
function matchKindFor(route, name, table) {
  if (route.controller && route.controller === name + "Controller") return "controller";
  if (route.form_request && formRequestNamesModel(lastSegment(route.form_request), name)) return "form_request";
  if (table && uriHasSegment(route.uri, table)) return "uri";
  return null;
}

// FORM_REQUEST_VERBS are the CRUD verb prefixes Laravel's {Verb}{Model}Request
// FormRequest convention uses. Matching the verb prefix (rather than a bare
// substring) is what keeps the form-request signal precise: StorePowerUserRequest
// must NOT be attributed to "User" just because "User" is a substring of it —
// its model is PowerUser, per the convention (ADR 0002, precision over coverage).
const FORM_REQUEST_VERBS = ["Store", "Update", "Create", "Delete", "Save", "Patch", "Put", "Upsert"];

// formRequestNamesModel reports whether a FormRequest class name names the given
// model under the {Verb}{Model}Request convention: an optional CRUD verb prefix,
// then EXACTLY the model name, then the "Request" suffix. "PostRequest",
// "StorePostRequest", "UpdatePostRequest" all name Post; "StorePowerUserRequest"
// names PowerUser, not User, because "User" is not the whole {Model} token.
function formRequestNamesModel(className, name) {
  if (!className.endsWith("Request")) return false;
  const core = className.slice(0, -"Request".length); // "StorePost" | "Post" | "StorePowerUser"
  if (core === name) return true;
  return FORM_REQUEST_VERBS.some((verb) => core === verb + name);
}

// lastSegment reduces a possibly fully-qualified class reference to its final
// name segment (App\Http\Requests\StorePostRequest → StorePostRequest), so the
// form-request match works whether the contract carried the FQN or a short name.
function lastSegment(ref) {
  const s = String(ref || "");
  const slash = s.lastIndexOf("\\");
  return slash === -1 ? s : s.slice(slash + 1);
}

// uriHasSegment reports whether the model's table appears as a whole path
// segment of the URI (/posts, /posts/{post}), never a substring of a larger
// word (/blogposts must NOT match "posts") — the whole-segment rule is what
// keeps the weak URI signal from being noise.
function uriHasSegment(uri, table) {
  return String(uri || "")
    .split("/")
    .some((seg) => seg === table);
}

// findingsFor scopes the project's findings to the ONE model the page is about.
// The top-level model.findings array is category-level (count + label, no
// per-item reference), so per-model findings are re-derived here from the same
// underlying facts the server counts: the model's own mass-assignment state and
// the disagreements that name it. Severity comes from the shared severityFor
// table (findings.js), so the detail page and the Findings view can never
// disagree on a finding's level.
function findingsFor(name, self, disagreements) {
  const findings = [];

  // Unguarded (blocker): `$guarded = []` is the escape hatch — a non-nil, empty
  // Guarded. The nil-vs-[] distinction is load-bearing (eloquent.go): a nil
  // Guarded is guarded-by-omission and NOT a finding.
  if (self && Array.isArray(self.guarded) && self.guarded.length === 0) {
    findings.push({
      kind: "unguarded",
      severity: severityFor("unguarded"),
      title: "Unguarded:",
      subject: name,
      reason: "$guarded = [] — every column is mass-assignable",
    });
  }

  // Disagreements (warn): the ones whose Model is this model.
  disagreements
    .filter((d) => d.model === name)
    .forEach((d) => {
      findings.push({
        kind: "disagreements",
        severity: severityFor("disagreements"),
        title: "Disagreement:",
        subject: name + "::" + (d.relationship || ""),
        reason: d.reason || d.kind || "",
      });
    });

  return findings;
}

// reverseRelationsFor inverts every OTHER model's relationship list: any
// relationship whose target is `name` becomes a reverse relation naming its
// source model, the method that reaches `name`, and that method's kind. Source/
// discovery order is preserved (models in discovery order, relations in
// source-declaration order) — determinism is a hard convention (CLAUDE.md).
function reverseRelationsFor(name, models) {
  const reverse = [];
  models.forEach((m) => {
    // A model's OWN relations are shown forward — but a self-referential relation
    // (Category belongsTo Category as parent) is genuinely inbound too, so it is
    // NOT skipped: the r.target === name test below admits exactly the self-
    // targeting ones from this model and none of its other (forward-only) ones.
    (m.relationships || []).forEach((r) => {
      if (r.target === name) {
        reverse.push({ from: m.name, method: r.method, kind: r.kind, target: name });
      }
    });
  });
  return reverse;
}
