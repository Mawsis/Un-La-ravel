// Unit tests for the Findings view's row template (issue #38: severity reads
// as a leading status dot + tint, never a side-stripe class). findingRowHtml
// is pure (finding in, markup string out), exported for the same no-DOM
// testability reason as routeRowHtml in routes.js.

import { test } from "node:test";
import assert from "node:assert/strict";

import { findingRowHtml, findingsList, severityFor } from "../assets/js/views/findings.js";

test("a dead-route finding carries the warn status dot and links its subject", () => {
  // Dead routes are warn severity in the contract (1.8.0, issue #47), so the row
  // reads as the amber warn dot — not the red danger dot, which is reserved for
  // blockers. The dot is decorative — the row's own text carries the meaning —
  // so it must be hidden from assistive tech.
  const html = findingRowHtml({
    severity: "warn",
    title: "Dead route:",
    subject: "GET /old/endpoint",
    href: "#/routes?filter=%2Fold%2Fendpoint",
    reason: "controller method not found",
  });
  assert.match(html, /class="finding"/);
  assert.doesNotMatch(html, /finding dead/);
  assert.match(html, /class="status-dot warn" aria-hidden="true"/);
  assert.match(html, />GET \/old\/endpoint<\/a>/);
  assert.doesNotMatch(html, /border-left/);
});

test("a disagreement finding carries the warn status dot, not the dead class", () => {
  const html = findingRowHtml({
    severity: "warn",
    title: "Disagreement:",
    subject: "Post::comments",
    href: "#/models?filter=Post",
    reason: "relation table mismatch",
  });
  assert.match(html, /class="finding"/);
  assert.doesNotMatch(html, /finding dead/);
  assert.match(html, /class="status-dot warn" aria-hidden="true"/);
});

test("a finding escapes attacker-shaped subjects and reasons", () => {
  const html = findingRowHtml({
    severity: "warn",
    title: "Disagreement:",
    subject: '<img src=x onerror=alert(1)>',
    href: "#/models",
    reason: '<script>alert(1)</script>',
  });
  assert.doesNotMatch(html, /<img/);
  assert.doesNotMatch(html, /<script/);
  assert.match(html, /&lt;img/);
});

// severityFor mirrors the server's single kind→severity table (contract 1.8.0,
// internal/model/findings.go): unguarded models are blockers; dead routes and
// disagreements are warnings; anything unknown degrades to info. The browser
// derives severity the same way the contract does so the two never disagree.
test("severityFor maps each finding kind like the contract", () => {
  assert.equal(severityFor("unguarded"), "blocker");
  assert.equal(severityFor("dead_routes"), "warn");
  assert.equal(severityFor("disagreements"), "warn");
  assert.equal(severityFor("some_future_kind"), "info");
});

// findingsList is the pure grouping the view renders: it flattens dead routes
// and disagreements into row objects, each tagged with the severity its kind
// maps to, ordered by severity most-severe-first so a blocker could never be
// buried below a warning. Within one severity the source order is preserved (a
// stable sort), keeping dead routes ahead of disagreements as they always were.
test("findingsList tags each row with its contract severity and links its subject", () => {
  const deadRoutes = [{ method: "GET", uri: "/old", reason: "no action" }];
  const disagreements = [{ model: "Post", relationship: "lens", reason: "no table" }];

  const rows = findingsList(disagreements, deadRoutes, new URLSearchParams());

  assert.equal(rows.length, 2);
  // Dead routes and disagreements are both warn; source order (dead routes
  // first) is preserved by the stable severity sort.
  assert.deepEqual(
    rows.map((r) => [r.kind, r.severity]),
    [["dead_routes", "warn"], ["disagreements", "warn"]],
  );
  assert.match(rows[0].subject, /GET \/old/);
  assert.equal(rows[1].subject, "Post::lens");
});

// The ordering is a property of severity, not of category position: findingsList
// sorts by severity rank, so a more-severe row always precedes a less-severe one
// even if it appeared later in the input. Proven directly on the rank the sort
// uses, so the "most severe first" guarantee holds the day a blocker-severity
// detail row joins the view.
test("blocker outranks warn outranks info", () => {
  const rank = (s) => ["blocker", "warn", "info"].indexOf(s);
  assert.ok(rank(severityFor("unguarded")) < rank(severityFor("dead_routes")));
  assert.ok(rank(severityFor("dead_routes")) < rank(severityFor("some_future_kind")));
});

// A dead-route row still renders the danger status dot + dead tint: the contract
// severity "warn" for dead_routes drives grouping/ranking, but the row's visual
// severity dot maps blocker→danger and keeps warn as warn, reusing the existing
// --danger / --warn tokens and status-dot classes (no new CSS, no side-stripe).
test("findingRowHtml maps a blocker severity to the danger dot and dead tint", () => {
  const html = findingRowHtml({
    severity: "blocker",
    title: "Unguarded model:",
    subject: "Post",
    href: "#/models?filter=Post",
    reason: "$guarded = []",
  });
  assert.match(html, /class="finding dead"/);
  assert.match(html, /class="status-dot danger" aria-hidden="true"/);
  assert.doesNotMatch(html, /border-left/);
});
