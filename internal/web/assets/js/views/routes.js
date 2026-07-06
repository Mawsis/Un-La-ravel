// Routes view: filterable, sortable route table with dead-route
// highlighting. Ported from app.js's renderRoutes/drawRouteRows; sorting
// (design.md a11y baseline: sortable headers are <button>s with aria-sort)
// was added in a later PR.
//
// Filter/sort state is owned by the CALLER (main.js), not this module — the
// router (router.js) is the source of truth for both (design.md "URL &
// state"), so this module takes the current { filter, sort } and an
// onStateChange callback rather than keeping its own state. That keeps this
// file router-agnostic and testable in isolation.
//
// The controller cell renders as a shared entity-chip cross-link (issue #24):
// the controller is a jump-to-able entity, so it carries the consistent resolved-green
// chip visual language via entityChip rather than being an in-view filter
// button. The delegated chip listener in main.js routes the click.

import { $, escapeHtml } from "../dom.js";
import { entityChip, dangerFlag } from "../chip.js";

export const SORTABLE_COLUMNS = [
  { key: "method", label: "Method" },
  { key: "uri", label: "URI" },
  { key: "action", label: "Controller@Action" },
];

// renderRoutes draws the table for the given routes/deadRoutes at the given
// { filter, sortKey, sortDir } state. onStateChange(newState) fires when the
// user types in the filter box, clicks a sortable header, or clicks a
// controller cell; the caller re-renders (typically by pushing the new
// state into the router, which re-invokes renderRoutes with the updated
// state).
export function renderRoutes(routes, deadRoutes, state, onStateChange) {
  const deadSet = new Set((deadRoutes || []).map(deadKey));
  $("#badge-routes").textContent = routes.length;

  renderHeader(state, (sortKey) => {
    const sortDir = state.sortKey === sortKey ? -(state.sortDir || 1) : 1;
    onStateChange({ ...state, sortKey, sortDir });
  });
  drawRouteRows(routes, deadSet, state);

  const filterInput = $("#route-filter");
  if (filterInput.value !== (state.filter || "")) filterInput.value = state.filter || "";
  filterInput.oninput = (e) => onStateChange({ ...state, filter: e.target.value });
}

function renderHeader(state, onSortClick) {
  const thead = $("table.routes thead tr");
  thead.innerHTML =
    SORTABLE_COLUMNS.map((col) => {
      const active = state.sortKey === col.key;
      const dir = state.sortDir || 1;
      const ariaSort = active ? (dir === 1 ? "ascending" : "descending") : "none";
      return (
        '<th scope="col" aria-sort="' + ariaSort + '">' +
        '<button type="button" data-sort-key="' + col.key + '">' + escapeHtml(col.label) +
        (active ? (dir === 1 ? " ▲" : " ▼") : "") +
        "</button></th>"
      );
    }).join("") + '<th scope="col">Middleware</th>';

  thead.querySelectorAll("button[data-sort-key]").forEach((btn) => {
    btn.onclick = () => onSortClick(btn.dataset.sortKey);
  });
}

function deadKey(r) {
  return [r.method, r.uri, r.controller, r.action].join("|");
}

function sortRoutes(routes, state) {
  if (!state.sortKey) return routes;
  const key = state.sortKey;
  const dir = state.sortDir || 1;
  return [...routes].sort((a, b) => {
    const av = routeSortValue(a, key);
    const bv = routeSortValue(b, key);
    return av < bv ? -dir : av > bv ? dir : 0;
  });
}

function routeSortValue(r, key) {
  if (key === "action") return (r.controller || "") + "@" + (r.action || "");
  return r[key] || "";
}

// routeRowHtml is the pure per-row template: one route (dead or live) to one
// <tr> string, exported so the row's markup contract — method color-class,
// mono column classes, the DEAD danger flag cross-linking to the dead_routes
// finding (issue #28) — is unit-testable without a DOM.
export function routeRowHtml(r, isDead) {
  // The controller is a jump-to-able entity, so it renders as a cross-link
  // chip (issue #24); the @action suffix stays plain escaped text. A route
  // with no controller (closure/view route) has nothing to link to.
  const action =
    r.controller || r.action
      ? (r.controller
          ? entityChip({ kind: "controller", name: r.controller })
          : "—") +
        "@" +
        escapeHtml(r.action || "—")
      : '<span class="mw">(closure / view route)</span>';
  const mw = (r.middleware || []).length
    ? '<span class="mw">' + escapeHtml(r.middleware.join(", ")) + "</span>"
    : '<span class="mw">—</span>';
  return (
    '<tr class="' + (isDead ? "dead" : "") + '">' +
    '<td class="method m-' + escapeHtml(r.method || "") + '">' + escapeHtml(r.method || "") + "</td>" +
    '<td class="uri">' + escapeHtml(r.uri || "") + (isDead ? " " + dangerFlag("dead_routes", "DEAD") : "") + "</td>" +
    '<td class="action">' + action + "</td>" +
    "<td>" + mw + "</td>" +
    "</tr>"
  );
}

function drawRouteRows(routes, deadSet, state) {
  const q = (state.filter || "").trim().toLowerCase();
  const filtered = routes.filter((r) => {
    if (!q) return true;
    return (
      (r.method || "").toLowerCase().includes(q) ||
      (r.uri || "").toLowerCase().includes(q) ||
      (r.controller || "").toLowerCase().includes(q) ||
      (r.action || "").toLowerCase().includes(q)
    );
  });

  const rows = sortRoutes(filtered, state)
    .map((r) => routeRowHtml(r, deadSet.has(deadKey(r))))
    .join("");
  $("#routes-body").innerHTML = rows || '<tr><td colspan="4" class="hint" style="padding:16px">No routes match.</td></tr>';
}
