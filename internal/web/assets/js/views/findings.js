// Findings view: Disagreements + Dead Routes, or an "all clear" state.
// Ported from app.js's renderFindings with no behavior change. Findings are
// facts, not alarms (design.md principle 4) — the copy stays specific and
// calm rather than panicked.

import { $, escapeHtml } from "../dom.js";

export function renderFindings(disagreements, deadRoutes) {
  const total = (disagreements || []).length + (deadRoutes || []).length;
  const badge = $("#badge-findings");
  badge.textContent = total;
  badge.classList.toggle("danger", total > 0);

  const body = $("#findings-body");
  if (total === 0) {
    body.innerHTML = '<div class="all-clear">No dead routes, no Model↔Schema disagreements. All clear.</div>';
    return;
  }

  let html = "";
  (deadRoutes || []).forEach((d) => {
    html +=
      '<div class="finding dead"><div class="h">Dead route: ' +
      escapeHtml((d.method || "") + " " + (d.uri || "")) + "</div>" +
      '<div class="r">' + escapeHtml(d.reason || d.kind || "") + "</div></div>";
  });
  (disagreements || []).forEach((d) => {
    html +=
      '<div class="finding"><div class="h">Disagreement: ' +
      escapeHtml((d.model || "") + "::" + (d.relationship || "")) + "</div>" +
      '<div class="r">' + escapeHtml(d.reason || d.kind || "") + "</div></div>";
  });
  body.innerHTML = html;
}
