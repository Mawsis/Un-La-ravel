// Unit tests for the Findings view's row template (issue #38: severity reads
// as a leading status dot + tint, never a side-stripe class). findingRowHtml
// is pure (finding in, markup string out), exported for the same no-DOM
// testability reason as routeRowHtml in routes.js.

import { test } from "node:test";
import assert from "node:assert/strict";

import { findingRowHtml } from "../assets/js/views/findings.js";

test("a dead-route finding carries the danger status dot and links its subject", () => {
  const html = findingRowHtml({
    severity: "danger",
    title: "Dead route:",
    subject: "GET /old/endpoint",
    href: "#/routes?filter=%2Fold%2Fendpoint",
    reason: "controller method not found",
  });
  assert.match(html, /class="finding dead"/);
  // The dot is decorative — the row's own text carries the meaning — so it
  // must be hidden from assistive tech.
  assert.match(html, /class="status-dot danger" aria-hidden="true"/);
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
