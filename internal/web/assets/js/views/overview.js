// Overview view: project meta line + the stat card row. Ported from the
// original renderMeta/renderCards in app.js with no behavior change.

import { $, escapeHtml } from "../dom.js";

export function renderOverview(model) {
  renderMeta(model);
  renderCards(model);
}

function renderMeta(model) {
  const name = model.project_name || "(unnamed project)";
  const ver = model.laravel_version || "?";
  $("#proj-meta").innerHTML =
    "Project <strong>" + escapeHtml(name) + "</strong> · Laravel <strong>" +
    escapeHtml(ver) + "</strong> · schema " + escapeHtml(model.schema_version || "?");
}

function renderCards(model) {
  const dead = (model.dead_routes || []).length;
  const disagree = (model.disagreements || []).length;
  const unguardedCount = (model.models || []).filter(
    (m) => Array.isArray(m.guarded) && m.guarded.length === 0
  ).length;
  const cards = [
    { n: (model.schemas || []).length, l: "Tables" },
    { n: (model.models || []).length, l: "Models" },
    { n: (model.controllers || []).length, l: "Controllers" },
    { n: (model.routes || []).length, l: "Routes" },
    { n: (model.form_requests || []).length, l: "Form Requests" },
    { n: dead, l: "Dead Routes", warn: dead > 0 },
    { n: disagree, l: "Disagreements", warn: disagree > 0 },
    { n: unguardedCount, l: "Unguarded", warn: unguardedCount > 0 },
  ];
  $("#cards").innerHTML = cards
    .map(
      (c) =>
        '<div class="card' + (c.warn ? " warn" : "") + '"><div class="n">' +
        c.n + '</div><div class="l">' + c.l + "</div></div>"
    )
    .join("");
}
