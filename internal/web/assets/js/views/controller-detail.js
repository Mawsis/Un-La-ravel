// Controller detail page (issue #69): a URL-addressable page at
// #/controllers/{fqn} composed entirely client-side over the already-loaded
// Project Model — no new server endpoint, the same posture as the Model (#51)
// and Route (#68) detail pages.
//
// It shows one controller's public actions and, per action, the Routes that
// dispatch to it. This page depends directly on the controller-FQN fix (#63):
// routes-per-action is only correct once sub-namespaced controllers resolve, so
// every route match here is on Route.FQN, never on the verbatim
// Route.Controller reference (two namespaces can declare the same short name).

import { $, escapeHtml } from "../dom.js";
import { entityChip } from "../chip.js";
import { routeKey } from "./route-detail.js";

// composeControllerDetail derives the detail view-model for the controller with
// the given reference — a fully-qualified name, or a short name when that name
// is unambiguous across namespaces. It never throws: an unknown or ambiguous
// reference yields a view-model whose `found` is false, so the renderer shows a
// defined state rather than a blank page.
export function composeControllerDetail(ref, projectModel) {
  const pm = projectModel || {};
  const controllers = pm.controllers || [];
  const routes = pm.routes || [];

  const resolution = resolveController(ref, controllers);
  if (!resolution.controller) {
    return {
      ref,
      found: false,
      ambiguous: resolution.ambiguous,
      candidates: resolution.candidates,
      name: "",
      fqn: "",
      actions: [],
      undeclaredActions: [],
      routeCount: 0,
    };
  }

  const self = resolution.controller;
  // Every route dispatching to THIS controller, matched on the resolved FQN.
  const mine = routes.filter((r) => r.fqn === self.fqn);

  const declared = self.actions || [];
  const actions = declared.map((name) => {
    const forAction = mine.filter((r) => r.action === name).map(routeRow);
    return { name, routes: forAction, dispatched: forAction.length > 0 };
  });

  return {
    ref,
    found: true,
    ambiguous: false,
    candidates: [],
    name: self.name || "",
    fqn: self.fqn || "",
    actions,
    // Routes naming a method the class does not declare — a real mismatch (a
    // renamed method, a typo) that would otherwise vanish, since no action row
    // claims it. Reporting it is the honest move (ADR 0002).
    undeclaredActions: undeclaredActionsFor(declared, mine),
    routeCount: mine.length,
  };
}

// resolveController finds the controller a reference names. An exact FQN match
// wins outright. A short name resolves ONLY when exactly one controller carries
// it: silently picking one of several same-named classes across namespaces is
// precisely the false attribution #63 existed to fix, so an ambiguous reference
// deliberately fails and reports its candidates instead.
function resolveController(ref, controllers) {
  const exact = controllers.find((c) => c.fqn === ref);
  if (exact) return { controller: exact, ambiguous: false, candidates: [] };

  const byShortName = controllers.filter((c) => c.name === ref);
  if (byShortName.length === 1) {
    return { controller: byShortName[0], ambiguous: false, candidates: [] };
  }
  if (byShortName.length > 1) {
    return {
      controller: null,
      ambiguous: true,
      candidates: byShortName.map((c) => c.fqn),
    };
  }
  return { controller: null, ambiguous: false, candidates: [] };
}

// undeclaredActionsFor groups the routes whose action is absent from the
// controller's declared method list, keyed by that action name, in route order.
function undeclaredActionsFor(declared, routes) {
  const known = new Set(declared);
  const order = [];
  const byAction = new Map();
  routes.forEach((r) => {
    const name = r.action || "";
    if (known.has(name)) return;
    if (!byAction.has(name)) {
      byAction.set(name, []);
      order.push(name);
    }
    byAction.get(name).push(routeRow(r));
  });
  // Built from `order` (an array) rather than by iterating the Map, so no Map
  // iteration order reaches the output — determinism is a hard convention.
  return order.map((name) => ({ name, routes: byAction.get(name) }));
}

// routeRow reduces a contract Route to what a per-action row draws, plus the
// key that links straight through to that route's own detail page (#68).
function routeRow(r) {
  return {
    key: routeKey(r),
    method: r.method || "",
    uri: r.uri || "",
    auth: r.auth || "",
    name: r.name || "",
  };
}

// AUTH_PILL maps a route's auth state to its pill variant. Per DESIGN.md §1 the
// non-color channel is the label text the pill carries.
const AUTH_PILL = { authenticated: "ok", unauthenticated: "warn", unknown: "dim" };

// controllerDetailHtml is the pure per-page template: composed view-model in,
// HTML string out. Pure (no DOM) so the markup contract is unit-testable.
export function controllerDetailHtml(d) {
  if (!d || !d.found) {
    if (d && d.ambiguous) {
      // An ambiguous short name gets the candidates, not a guess: the user
      // picks, and the page never silently attributes routes to the wrong class.
      const rows = (d.candidates || [])
        .map((fqn) => '<div class="route-row">' + entityChip({ kind: "controller", name: fqn }) + "</div>")
        .join("");
      return (
        '<div class="detail-page">' +
        '<div class="detail-head"><h2 tabindex="-1">Ambiguous controller name</h2></div>' +
        '<div class="empty-note">More than one controller is named "' + escapeHtml(d.ref || "") +
        '". Pick the one you meant:</div>' + rows +
        "</div>"
      );
    }
    return (
      '<div class="detail-page">' +
      '<div class="detail-head"><h2 tabindex="-1">Controller not found</h2></div>' +
      '<div class="empty-note">No controller named "' + escapeHtml((d && d.ref) || "") +
      '" was found in this analysis. <a href="#/routes" data-view="routes">Back to routes</a>.</div>' +
      "</div>"
    );
  }

  return (
    '<div class="detail-page">' +
    '<div class="detail-head"><h2 tabindex="-1">' + escapeHtml(d.name) + "</h2>" +
    '<div class="empty-note"><code>' + escapeHtml(d.fqn) + "</code></div>" +
    '<div class="empty-note">' + d.actions.length + " public action" + (d.actions.length === 1 ? "" : "s") +
    ", " + d.routeCount + " route" + (d.routeCount === 1 ? "" : "s") + " dispatching here</div>" +
    "</div>" +
    actionsSection(d) +
    undeclaredSection(d) +
    "</div>"
  );
}

// actionsSection lists each public action with the routes that reach it. An
// action no route reaches is still listed and said to be undispatched — that is
// information (dead code, or a method called internally), not an empty row.
function actionsSection(d) {
  if (d.actions.length === 0) {
    return (
      '<section class="detail-section"><div class="subhead">Actions</div>' +
      '<div class="empty-note">This controller declares no public actions.</div></section>'
    );
  }
  const blocks = d.actions
    .map((a) => {
      const rows = a.dispatched
        ? a.routes.map(routeRowHtml).join("")
        : '<div class="empty-note">No route dispatches to this action.</div>';
      return (
        '<div class="fr-row"><span class="fr-name">' + escapeHtml(a.name) + "</span>" + rows + "</div>"
      );
    })
    .join("");
  return '<section class="detail-section"><div class="subhead">Actions</div>' + blocks + "</section>";
}

// undeclaredSection surfaces routes pointing at methods the class doesn't
// declare — shown apart from the real actions so the two are never confused.
function undeclaredSection(d) {
  if ((d.undeclaredActions || []).length === 0) return "";
  const blocks = d.undeclaredActions
    .map(
      (a) =>
        '<div class="fr-row"><span class="fr-name">' + escapeHtml(a.name) +
        ' <span class="pill warn">not declared</span></span>' +
        a.routes.map(routeRowHtml).join("") +
        "</div>"
    )
    .join("");
  return (
    '<section class="detail-section"><div class="subhead">Routes naming an undeclared method</div>' +
    '<div class="empty-note">These routes dispatch to a method this class does not declare &mdash; ' +
    "a renamed method or a typo.</div>" +
    blocks +
    "</section>"
  );
}

// routeRowHtml draws one dispatching route, linked to its own detail page.
function routeRowHtml(r) {
  const pill = AUTH_PILL[r.auth] || "dim";
  return (
    '<div class="route-row">' +
    // Uppercase "m-<METHOD>" — the class the color rules key on, same as the
    // routes table and the Route detail page.
    '<span class="method m-' + escapeHtml(r.method) + '">' + escapeHtml(r.method) + "</span> " +
    '<a href="#/routes/' + encodeURIComponent(r.key) + '" data-view="routes" data-entity-id="' +
    escapeHtml(r.key) + '"><span class="uri">' + escapeHtml(r.uri) + "</span></a> " +
    '<span class="pill ' + pill + '">' + escapeHtml(r.auth || "unknown") + "</span>" +
    "</div>"
  );
}

// renderControllerDetail is the thin DOM wrapper, mirroring renderModelDetail /
// renderRouteDetail. A null/empty ref closes the detail page.
export function renderControllerDetail(ref, projectModel) {
  const body = $("#controller-detail-body");
  if (!body) return;
  if (!ref) {
    body.hidden = true;
    body.innerHTML = "";
    return;
  }
  body.innerHTML = controllerDetailHtml(composeControllerDetail(ref, projectModel));
  body.hidden = false;
}
