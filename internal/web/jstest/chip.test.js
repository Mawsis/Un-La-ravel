// Unit tests for the shared entity-chip / cross-link helper (issue #24).
//
// Run with the zero-dependency Node built-in test runner (no build step, no
// node_modules — matches the "no external CDN / no build step" architecture
// constraint the rest of the web assets follow):
//
//   node --test 'internal/web/jstest/**/*.test.js'
//
// These live outside assets/ on purpose: assets/ is embedded verbatim into the
// Go binary (//go:embed assets), so shipping *.test.js there would bloat the
// binary with dead test code — the same "no stray files in the embedded tree"
// discipline TestHandler_OldAppJS_Gone enforces. The runner test in
// internal/web/js_test.go invokes this under `go test`.
//
// The target-computation (chipTarget) is a pure function of a reference, so it
// is tested directly with no DOM. entityChip's markup is asserted as a string.

import { test } from "node:test";
import assert from "node:assert/strict";

import { chipTarget, entityChip, dangerFlag } from "../assets/js/chip.js";

test("a model reference targets its detail page, keyed and sub-routed by name", () => {
  // The detail carries the name into the #/models/{name} sub-route (issue #51),
  // so the chip's native href lands on the model's own page, not the flat list.
  assert.deepEqual(chipTarget({ kind: "model", name: "User" }), {
    view: "models",
    id: "User",
    detail: "User",
  });
});

test("a table reference targets the ER diagram, keyed by table name", () => {
  assert.deepEqual(chipTarget({ kind: "table", name: "users" }), {
    view: "er",
    id: "users",
  });
});

test("a controller reference targets its own detail page, keyed by class name", () => {
  // Was: the Routes view, filtered to the class name. Since issue #69 a
  // controller has its own page (#/controllers/{fqn}) showing the actions AND
  // the routes per action — strictly more than the filter conveyed — so the
  // chip carries `detail` and lands there, exactly as a model chip does.
  assert.deepEqual(chipTarget({ kind: "controller", name: "UserController" }), {
    view: "controllers",
    id: "UserController",
    detail: "UserController",
  });
});

test("a route reference targets the Routes view, keyed by method + URI", () => {
  assert.deepEqual(
    chipTarget({ kind: "route", method: "GET", uri: "/users/{user}" }),
    { view: "routes", id: "GET /users/{user}" }
  );
});

test("a finding reference targets the Findings view, keyed by finding kind", () => {
  // Issue #28: inline danger flags (dead-route rows, unguarded-model cards)
  // cross-link to their corresponding finding category in the Findings view.
  // The name field carries the machine-readable Finding.Kind from the
  // contract (internal/model/findings.go): dead_routes / unguarded / ...
  assert.deepEqual(chipTarget({ kind: "finding", name: "dead_routes" }), {
    view: "findings",
    id: "dead_routes",
  });
  assert.deepEqual(chipTarget({ kind: "finding", name: "unguarded" }), {
    view: "findings",
    id: "unguarded",
  });
});

test("an unknown or missing kind resolves to no target", () => {
  assert.equal(chipTarget({ kind: "banana", name: "x" }), null);
  assert.equal(chipTarget(null), null);
  assert.equal(chipTarget({}), null);
});

test("a known kind with no identifying field resolves to no target", () => {
  // A recognized kind but no name/method/uri to key on must NOT produce a
  // chip carrying data-entity-id="undefined" — it degrades like an unknown
  // kind, so a later focus-this-entity consumer never gets a bogus id.
  assert.equal(chipTarget({ kind: "model" }), null);
  assert.equal(chipTarget({ kind: "model", name: "" }), null);
  assert.equal(chipTarget({ kind: "table", name: "" }), null);
  assert.equal(chipTarget({ kind: "controller", name: "" }), null);
  assert.equal(chipTarget({ kind: "route" }), null);
  assert.equal(chipTarget({ kind: "route", method: "", uri: "" }), null);
});

test("a route reference with only a URI still resolves (method optional)", () => {
  assert.deepEqual(chipTarget({ kind: "route", uri: "/health" }), {
    view: "routes",
    id: " /health",
  });
});

test("entityChip renders an anchor carrying the target view and entity id", () => {
  const html = entityChip({ kind: "model", name: "User" });
  assert.match(html, /<a\b/);
  assert.match(html, /class="entity-chip"/);
  // A model chip's native href is the model's own detail page (#/models/{name},
  // issue #51), NOT the bare flat-list view — so middle/cmd/shift-click open the
  // right page in a new tab, matching where a plain left-click navigates.
  assert.match(html, /href="#\/models\/User"/);
  assert.match(html, /data-view="models"/);
  assert.match(html, /data-entity-id="User"/);
  assert.match(html, />User<\/a>/);
});

test("a model chip's href points at its detail sub-route and encodes the name", () => {
  const html = entityChip({ kind: "model", name: "My Model" });
  assert.match(html, /href="#\/models\/My%20Model"/);
});

test("a non-detail chip's href stays the bare view hash", () => {
  // Table/controller chips still resolve to a whole view (focusing the specific
  // entity within it lands later), so their native href is the view hash.
  const table = entityChip({ kind: "table", name: "users" });
  assert.match(table, /href="#\/er"/);
});

test("entityChip escapes its label and id (names come from parsed source)", () => {
  const html = entityChip({ kind: "model", name: '<img src=x onerror=alert(1)>' });
  assert.doesNotMatch(html, /<img/);
  assert.match(html, /&lt;img/);
});

test("dangerFlag renders a danger-classed cross-link to the finding category", () => {
  // Issue #28: the inline DEAD / Unguarded markers are chips, so they ride the
  // same delegated navigation handler in main.js (a.entity-chip[data-view]),
  // but carry the compound danger-flag class so CSS paints them with the
  // danger token instead of the interactive cyan (same pattern as the
  // verdict-link chips from issue #27).
  const html = dangerFlag("dead_routes", "DEAD");
  assert.match(html, /<a\b/);
  assert.match(html, /class="entity-chip danger-flag"/);
  assert.match(html, /href="#\/findings"/);
  assert.match(html, /data-view="findings"/);
  assert.match(html, /data-entity-id="dead_routes"/);
  assert.match(html, />DEAD<\/a>/);
});

test("dangerFlag escapes its label", () => {
  const html = dangerFlag("unguarded", "<b>x</b>");
  assert.doesNotMatch(html, /<b>/);
  assert.match(html, /&lt;b&gt;/);
});

test("an unresolvable reference degrades to inert escaped text, not a link", () => {
  const html = entityChip({ kind: "banana", name: "Whatever" });
  assert.doesNotMatch(html, /<a\b/);
  assert.match(html, /Whatever/);
});
