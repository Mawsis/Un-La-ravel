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

import { chipTarget, entityChip } from "../assets/js/chip.js";

test("a model reference targets the Models view, keyed by model name", () => {
  assert.deepEqual(chipTarget({ kind: "model", name: "User" }), {
    view: "models",
    id: "User",
  });
});

test("a table reference targets the ER diagram, keyed by table name", () => {
  assert.deepEqual(chipTarget({ kind: "table", name: "users" }), {
    view: "er",
    id: "users",
  });
});

test("a controller reference targets the Routes view, keyed by class name", () => {
  assert.deepEqual(chipTarget({ kind: "controller", name: "UserController" }), {
    view: "routes",
    id: "UserController",
  });
});

test("a route reference targets the Routes view, keyed by method + URI", () => {
  assert.deepEqual(
    chipTarget({ kind: "route", method: "GET", uri: "/users/{user}" }),
    { view: "routes", id: "GET /users/{user}" }
  );
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
  assert.match(html, /href="#\/models"/);
  assert.match(html, /data-view="models"/);
  assert.match(html, /data-entity-id="User"/);
  assert.match(html, />User<\/a>/);
});

test("entityChip escapes its label and id (names come from parsed source)", () => {
  const html = entityChip({ kind: "model", name: '<img src=x onerror=alert(1)>' });
  assert.doesNotMatch(html, /<img/);
  assert.match(html, /&lt;img/);
});

test("an unresolvable reference degrades to inert escaped text, not a link", () => {
  const html = entityChip({ kind: "banana", name: "Whatever" });
  assert.doesNotMatch(html, /<a\b/);
  assert.match(html, /Whatever/);
});
