// Findings view: Disagreements + Dead Routes, grouped by severity, or an "all
// clear" state. Ported from app.js's renderFindings; each finding links to its
// subject (design.md "Cross-navigation": "A Disagreement or Dead Route finding
// → links to the Model/Route it's about"). Findings are facts, not alarms
// (design.md principle 4) — the copy stays specific and calm rather than
// panicked.

import { $, escapeHtml } from "../dom.js";
import { hrefFor } from "../links.js";

// severityFor mirrors the server's single kind→severity table (contract 1.8.0,
// internal/model/findings.go): unguarded models are blockers; dead routes and
// disagreements are warnings; an unknown kind degrades to info. The browser
// derives severity the SAME way the contract does, so the grouped view and the
// server-computed model.findings can never disagree on a finding's level.
const SEVERITY_BY_KIND = {
  unguarded: "blocker",
  dead_routes: "warn",
  disagreements: "warn",
};
export function severityFor(kind) {
  return SEVERITY_BY_KIND[kind] || "info";
}

// SEVERITY_RANK orders severities most-severe-first (blocker < warn < info) so a
// stable sort on this rank surfaces blockers above warnings above info. An
// unranked severity sorts last, never above a known one.
const SEVERITY_RANK = { blocker: 0, warn: 1, info: 2 };
function severityRank(severity) {
  const r = SEVERITY_RANK[severity];
  return r === undefined ? Number.MAX_SAFE_INTEGER : r;
}

// DOT_CLASS bridges the contract severity to the existing status-dot CSS class
// (issue #47): the contract speaks blocker/warn/info, but the design tokens and
// the .status-dot classes are --danger/--warn, so a blocker reads as the red
// danger dot. Reuses existing tokens — no new CSS, no side-stripe.
const DOT_CLASS = { blocker: "danger", warn: "warn", info: "warn" };
function dotClass(severity) {
  return DOT_CLASS[severity] || "warn";
}

// findingRowHtml is the pure per-finding template (issue #38): severity reads as
// a leading status dot + the card's tint, never a side-stripe. The dot is
// aria-hidden because the row's own text ("Dead route:" / "Disagreement:")
// already says what the color means — the non-color channel. Exported so the
// row's markup contract is unit-testable without a DOM, mirroring routeRowHtml.
export function findingRowHtml(f) {
  // blocker → the red danger dot + "dead" tint; warn/info → the amber warn dot.
  const dot = dotClass(f.severity);
  const dead = dot === "danger";
  return (
    '<div class="finding' + (dead ? " dead" : "") + '">' +
    '<div class="h">' +
    '<span class="status-dot ' + dot + '" aria-hidden="true"></span>' +
    escapeHtml(f.title) + ' <a href="' + escapeHtml(f.href) + '">' + escapeHtml(f.subject) + "</a></div>" +
    '<div class="r">' + escapeHtml(f.reason || "") + "</div></div>"
  );
}

// findingsList flattens the two detail arrays into row objects, tags each with
// the severity its kind maps to, and orders them by severity most-severe-first
// (issue #47). The sort is stable, so within one severity the source order is
// preserved — dead routes stay ahead of disagreements as they always were.
// Pure (arrays in, ordered rows out) so the grouping is unit-testable without a
// DOM, the same split as routeRowHtml/renderRoutes in routes.js.
export function findingsList(disagreements, deadRoutes, currentParams) {
  const rows = [];
  (deadRoutes || []).forEach((d) => {
    // hrefFor's output is already percent-encoded by URLSearchParams and can't
    // contain a literal '"' — escaping in findingRowHtml is defense-in-depth
    // consistency with search.js, not a fix for a live gap.
    rows.push({
      kind: "dead_routes",
      severity: severityFor("dead_routes"),
      title: "Dead route:",
      subject: (d.method || "") + " " + (d.uri || ""),
      href: hrefFor("route", d.uri || "", currentParams),
      reason: d.reason || d.kind || "",
    });
  });
  (disagreements || []).forEach((d) => {
    rows.push({
      kind: "disagreements",
      severity: severityFor("disagreements"),
      title: "Disagreement:",
      subject: (d.model || "") + "::" + (d.relationship || ""),
      href: hrefFor("model", d.model || "", currentParams),
      reason: d.reason || d.kind || "",
    });
  });
  // Stable sort by severity rank: [].sort is stable in modern engines, so equal
  // severities keep their push order (dead routes before disagreements).
  return rows.sort((a, b) => severityRank(a.severity) - severityRank(b.severity));
}

// The Findings count badge is NOT set here: sidebar.js owns it (issue #23),
// driven by the server-computed model.findings so the badge and the health chip
// agree by construction. This view renders only the panel body — the severity-
// grouped detail rows or the all-clear state.
export function renderFindings(disagreements, deadRoutes, currentParams) {
  const rows = findingsList(disagreements, deadRoutes, currentParams);
  const body = $("#findings-body");
  if (rows.length === 0) {
    body.innerHTML = '<div class="all-clear">No dead routes, no Model↔Schema disagreements. All clear.</div>';
    return;
  }
  body.innerHTML = rows.map(findingRowHtml).join("");
}
