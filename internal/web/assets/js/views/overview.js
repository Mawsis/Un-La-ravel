// Overview view: project meta line, the health verdict (issue #27), then the
// demoted neutral inventory grid. The verdict leads so problems out-shout
// inventory; the counts below are reference numbers, not findings.

import { $, escapeHtml } from "../dom.js";
import { healthVerdict } from "../verdict.js";

export function renderOverview(model) {
  renderMeta(model);
  renderVerdict(model);
  renderCards(model);
}

function renderMeta(model) {
  const name = model.project_name || "(unnamed project)";
  const ver = model.laravel_version || "?";
  $("#proj-meta").innerHTML =
    "Project <strong>" + escapeHtml(name) + "</strong> · Laravel <strong>" +
    escapeHtml(ver) + "</strong> · schema " + escapeHtml(model.schema_version || "?");
}

// renderVerdict draws the health verdict computed by healthVerdict: a calm
// "No issues found." for a clean project, or an itemized breakdown where each
// problem is an <a data-view="findings"> so the delegated chip handler in
// main.js navigates it into the Findings view. No score, no letter grade.
function renderVerdict(model) {
  const v = healthVerdict(model);
  if (v.clean) {
    $("#verdict").innerHTML =
      '<p class="verdict-clean">No issues found.</p>';
    return;
  }
  $("#verdict").innerHTML =
    '<ul class="verdict-problems">' +
    v.problems
      .map(
        (p) =>
          '<li class="verdict-item"><a href="#/' + escapeHtml(p.view) + '"' +
          ' data-view="' + escapeHtml(p.view) + '" class="entity-chip verdict-link">' +
          escapeHtml(p.label) + "</a></li>"
      )
      .join("") +
    "</ul>";
}

// renderCards draws only the neutral inventory counts, demoted below the
// verdict. The dead-route/disagreement/unguarded counts live in the verdict
// now, so they are deliberately absent here.
function renderCards(model) {
  const cards = [
    { n: (model.schemas || []).length, l: "Tables" },
    { n: (model.models || []).length, l: "Models" },
    { n: (model.controllers || []).length, l: "Controllers" },
    { n: (model.routes || []).length, l: "Routes" },
    { n: (model.form_requests || []).length, l: "Form Requests" },
  ];
  $("#cards").innerHTML = cards
    .map(
      (c) =>
        '<div class="card"><div class="n">' +
        c.n + '</div><div class="l">' + c.l + "</div></div>"
    )
    .join("");
}
