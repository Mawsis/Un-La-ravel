// Routes view: filterable route table with dead-route highlighting. Ported
// from app.js's renderRoutes/drawRouteRows. Column headers are <button>
// elements inside <th> (design.md a11y baseline "sortable headers are
// <button> elements... with aria-sort") — sorting itself is wired up here
// since the markup and behavior are one unit; it was not present before.

import { $, escapeHtml } from "../dom.js";

const SORTABLE_COLUMNS = [
  { key: "method", label: "Method" },
  { key: "uri", label: "URI" },
  { key: "action", label: "Controller@Action" },
];

let sortState = { key: null, dir: 1 };

export function renderRoutes(routes, deadRoutes) {
  const deadSet = new Set((deadRoutes || []).map(deadKey));
  $("#badge-routes").textContent = routes.length;

  renderHeader();
  drawRouteRows(routes, deadSet, "");
  $("#route-filter").oninput = (e) => drawRouteRows(routes, deadSet, e.target.value);
}

function renderHeader() {
  const thead = $("table.routes thead tr");
  thead.innerHTML =
    SORTABLE_COLUMNS.map((col) => {
      const active = sortState.key === col.key;
      const ariaSort = active ? (sortState.dir === 1 ? "ascending" : "descending") : "none";
      return (
        '<th scope="col" aria-sort="' + ariaSort + '">' +
        '<button type="button" data-sort-key="' + col.key + '">' + escapeHtml(col.label) +
        (active ? (sortState.dir === 1 ? " ▲" : " ▼") : "") +
        "</button></th>"
      );
    }).join("") + '<th scope="col">Middleware</th>';
}

function deadKey(r) {
  return [r.method, r.uri, r.controller, r.action].join("|");
}

function sortRoutes(routes) {
  if (!sortState.key) return routes;
  const key = sortState.key;
  const dir = sortState.dir;
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

function drawRouteRows(routes, deadSet, filter) {
  const q = filter.trim().toLowerCase();
  const filtered = routes.filter((r) => {
    if (!q) return true;
    return (
      (r.method || "").toLowerCase().includes(q) ||
      (r.uri || "").toLowerCase().includes(q) ||
      (r.controller || "").toLowerCase().includes(q) ||
      (r.action || "").toLowerCase().includes(q)
    );
  });

  const rows = sortRoutes(filtered)
    .map((r) => {
      const isDead = deadSet.has(deadKey(r));
      const action =
        r.controller || r.action
          ? escapeHtml((r.controller || "—") + "@" + (r.action || "—"))
          : '<span class="mw">(closure / view route)</span>';
      const mw = (r.middleware || []).length
        ? '<span class="mw">' + escapeHtml(r.middleware.join(", ")) + "</span>"
        : '<span class="mw">—</span>';
      return (
        '<tr class="' + (isDead ? "dead" : "") + '">' +
        '<td class="method m-' + escapeHtml(r.method || "") + '">' + escapeHtml(r.method || "") + "</td>" +
        '<td class="uri">' + escapeHtml(r.uri || "") + (isDead ? '<span class="dead-tag">DEAD</span>' : "") + "</td>" +
        '<td class="action">' + action + "</td>" +
        "<td>" + mw + "</td>" +
        "</tr>"
      );
    })
    .join("");
  $("#routes-body").innerHTML = rows || '<tr><td colspan="4" class="hint" style="padding:16px">No routes match.</td></tr>';

  wireSortButtons(routes, deadSet, filter);
}

function wireSortButtons(routes, deadSet, filter) {
  document.querySelectorAll("table.routes thead button[data-sort-key]").forEach((btn) => {
    btn.onclick = () => {
      const key = btn.dataset.sortKey;
      sortState = sortState.key === key ? { key, dir: -sortState.dir } : { key, dir: 1 };
      renderHeader();
      drawRouteRows(routes, deadSet, filter);
    };
  });
}
