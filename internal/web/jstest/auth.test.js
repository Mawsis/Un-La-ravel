// Unit tests for the Auth view (issue #50): a verdict sentence in the tool's
// voice, routes grouped by middleware stack, and a non-color severity channel
// (ADR 0011 — a blocker write reads as a labelled danger dot, never a bare hue
// or a side-stripe). Every function is pure (model data in, string/objects out),
// exported for no-DOM testability the same way findings.js and routes.js are.

import { test } from "node:test";
import assert from "node:assert/strict";

import {
  authVerdict,
  groupByStack,
  authRowHtml,
  isWriteMethod,
  STACK_NONE_LABEL,
} from "../assets/js/views/auth.js";

// A small fixture mirroring the contract shape: each route carries the `auth`
// field the server stamped (issue #50), so the view READS it rather than
// re-classifying middleware in the browser (ADR 0008).
const routes = [
  { method: "GET", uri: "/posts", middleware: [], auth: "unauthenticated" },
  { method: "GET", uri: "/legacy", middleware: [], auth: "unauthenticated" },
  { method: "POST", uri: "/webhooks", middleware: [], auth: "unauthenticated" },
  { method: "POST", uri: "/posts", middleware: ["auth"], auth: "authenticated" },
  { method: "GET", uri: "/admin/users", middleware: ["auth:sanctum", "throttle:api"], auth: "authenticated" },
  { method: "DELETE", uri: "/custom", middleware: ["verified"], auth: "unknown" },
];

test("the verdict sentence counts unauthenticated routes and how many write, in the tool's voice", () => {
  const v = authVerdict(routes);
  // 3 unauthenticated of 6 routes, 1 of them a write (POST /webhooks).
  assert.equal(v.unauthenticated, 3);
  assert.equal(v.total, 6);
  assert.equal(v.writes, 1);
  // The sentence names the counts plainly — a fact, not an alarm.
  assert.match(v.sentence, /3 of 6 routes have no auth middleware/);
  assert.match(v.sentence, /1 of them write/);
});

test("the verdict reads the contract's auth field, not a re-derived classification", () => {
  // A route the server marked authenticated is NOT counted even if we passed
  // empty middleware — the view trusts the stamped field (ADR 0008).
  const trusted = [
    { method: "GET", uri: "/x", middleware: [], auth: "authenticated" },
    { method: "GET", uri: "/y", middleware: [], auth: "unauthenticated" },
  ];
  const v = authVerdict(trusted);
  assert.equal(v.unauthenticated, 1);
});

test("a clean project (every route authenticated) yields an all-clear verdict", () => {
  const v = authVerdict([{ method: "GET", uri: "/x", middleware: [], auth: "authenticated" }]);
  assert.equal(v.unauthenticated, 0);
  assert.match(v.sentence, /every route/i);
});

test("unauthenticated routes group by their middleware stack; a no-middleware stack has its own label", () => {
  const groups = groupByStack(routes);
  // Only the three unauthenticated routes appear; the two no-middleware ones
  // share a group and the label reads as "(no middleware)".
  const none = groups.find((g) => g.stack === STACK_NONE_LABEL);
  assert.ok(none, "expected a no-middleware group");
  assert.equal(none.routes.length, 3); // GET /posts, GET /legacy, POST /webhooks
  // Authenticated / unknown routes are not in any unauthenticated group.
  assert.ok(!groups.some((g) => g.routes.some((r) => r.uri === "/admin/users")));
  assert.ok(!groups.some((g) => g.routes.some((r) => r.uri === "/custom")));
});

test("a write row reads as the danger dot with a spelled-out WRITE label — the non-color channel", () => {
  const html = authRowHtml({ method: "POST", uri: "/webhooks", auth: "unauthenticated" }, new URLSearchParams());
  // Blocker severity → red danger dot, but the meaning is ALSO in the text
  // ("write"), so a red/green-blind reader gets the signal without hue.
  assert.match(html, /class="status-dot danger" aria-hidden="true"/);
  assert.match(html, /write/i);
  // No side-stripe (ADR 0011 / issue #38): severity is the dot + text, not a
  // left border.
  assert.doesNotMatch(html, /border-left/);
});

test("a read row reads as the warn dot, not the danger dot", () => {
  const html = authRowHtml({ method: "GET", uri: "/posts", auth: "unauthenticated" }, new URLSearchParams());
  assert.match(html, /class="status-dot warn" aria-hidden="true"/);
});

test("a route row escapes attacker-shaped URIs", () => {
  const html = authRowHtml({ method: "GET", uri: '<img src=x onerror=alert(1)>', auth: "unauthenticated" }, new URLSearchParams());
  assert.doesNotMatch(html, /<img/);
  assert.match(html, /&lt;img/);
});

test("isWriteMethod treats the four mutating verbs as writes and GET as a read", () => {
  for (const m of ["POST", "PUT", "PATCH", "DELETE", "post"]) {
    assert.equal(isWriteMethod(m), true, `${m} should be a write`);
  }
  for (const m of ["GET", "HEAD", "OPTIONS"]) {
    assert.equal(isWriteMethod(m), false, `${m} should be a read`);
  }
});
