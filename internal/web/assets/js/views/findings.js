// Findings view: Disagreements + Dead Routes, or an "all clear" state.
// Ported from app.js's renderFindings; each finding now links to its
// subject (design.md "Cross-navigation": "A Disagreement or Dead Route
// finding → links to the Model/Route it's about"). Findings are facts, not
// alarms (design.md principle 4) — the copy stays specific and calm rather
// than panicked.

import { $, escapeHtml } from "../dom.js";
import { hrefFor } from "../links.js";

// The Findings count badge is NOT set here: sidebar.js owns it (issue #23),
// driven by the server-computed model.findings so the badge and the health
// chip agree by construction. This view renders only the panel body.
export function renderFindings(disagreements, deadRoutes, currentParams) {
  const total = (disagreements || []).length + (deadRoutes || []).length;
  const body = $("#findings-body");
  if (total === 0) {
    body.innerHTML = '<div class="all-clear">No dead routes, no Model↔Schema disagreements. All clear.</div>';
    return;
  }

  let html = "";
  (deadRoutes || []).forEach((d) => {
    const subject = (d.method || "") + " " + (d.uri || "");
    // hrefFor's output is already percent-encoded by URLSearchParams and
    // can't contain a literal '"' — escapeHtml here is defense-in-depth
    // consistency with search.js, not a fix for a live gap.
    const href = escapeHtml(hrefFor("route", d.uri || "", currentParams));
    html +=
      '<div class="finding dead"><div class="h">Dead route: <a href="' + href + '">' +
      escapeHtml(subject) + "</a></div>" +
      '<div class="r">' + escapeHtml(d.reason || d.kind || "") + "</div></div>";
  });
  (disagreements || []).forEach((d) => {
    const subject = (d.model || "") + "::" + (d.relationship || "");
    const href = escapeHtml(hrefFor("model", d.model || "", currentParams));
    html +=
      '<div class="finding"><div class="h">Disagreement: <a href="' + href + '">' +
      escapeHtml(subject) + "</a></div>" +
      '<div class="r">' + escapeHtml(d.reason || d.kind || "") + "</div></div>";
  });
  body.innerHTML = html;
}
