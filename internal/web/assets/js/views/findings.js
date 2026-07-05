// Findings view: Disagreements + Dead Routes, or an "all clear" state.
// Ported from app.js's renderFindings; each finding now links to its
// subject (design.md "Cross-navigation": "A Disagreement or Dead Route
// finding → links to the Model/Route it's about"). Findings are facts, not
// alarms (design.md principle 4) — the copy stays specific and calm rather
// than panicked.

import { $, escapeHtml } from "../dom.js";
import { hrefFor } from "../links.js";

// findingRowHtml is the pure per-finding template (issue #38): severity reads
// as a leading status dot + the card's tint, never a side-stripe. The dot is
// aria-hidden because the row's own text ("Dead route:" / "Disagreement:")
// already says what the color means. Exported so the row's markup contract is
// unit-testable without a DOM, mirroring routeRowHtml in routes.js.
export function findingRowHtml(f) {
  const dead = f.severity === "danger";
  return (
    '<div class="finding' + (dead ? " dead" : "") + '">' +
    '<div class="h">' +
    '<span class="status-dot ' + (dead ? "danger" : "warn") + '" aria-hidden="true"></span>' +
    escapeHtml(f.title) + ' <a href="' + escapeHtml(f.href) + '">' + escapeHtml(f.subject) + "</a></div>" +
    '<div class="r">' + escapeHtml(f.reason || "") + "</div></div>"
  );
}

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
    // hrefFor's output is already percent-encoded by URLSearchParams and
    // can't contain a literal '"' — escaping in findingRowHtml is
    // defense-in-depth consistency with search.js, not a fix for a live gap.
    html += findingRowHtml({
      severity: "danger",
      title: "Dead route:",
      subject: (d.method || "") + " " + (d.uri || ""),
      href: hrefFor("route", d.uri || "", currentParams),
      reason: d.reason || d.kind || "",
    });
  });
  (disagreements || []).forEach((d) => {
    html += findingRowHtml({
      severity: "warn",
      title: "Disagreement:",
      subject: (d.model || "") + "::" + (d.relationship || ""),
      href: hrefFor("model", d.model || "", currentParams),
      reason: d.reason || d.kind || "",
    });
  });
  body.innerHTML = html;
}
