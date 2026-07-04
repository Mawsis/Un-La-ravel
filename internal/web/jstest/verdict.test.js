// Unit tests for the Overview health-verdict renderer (issue #27).
//
// Run with the zero-dependency Node built-in test runner (no build step, no
// node_modules — matches the "no external CDN / no build step" architecture
// constraint the rest of the web assets follow):
//
//   node --test 'internal/web/jstest/**/*.test.js'
//
// Like chip.test.js, these live outside assets/ so the embedded tree stays free
// of dead test code, and the runner test in internal/web/js_test.go invokes them
// under `go test`.
//
// healthVerdict is a pure function of the analyzed model — no DOM — so it is
// tested directly against model fixtures. Tests assert the *observable* verdict:
// clean vs. problems, the itemized breakdown by category, and that each item
// carries the finding link. They never assert DOM structure or class names,
// which churn during the redesign.

import { test } from "node:test";
import assert from "node:assert/strict";

import { healthVerdict } from "../assets/js/verdict.js";

test("a clean project yields a clean verdict with no problems", () => {
  const v = healthVerdict({
    dead_routes: [],
    disagreements: [],
    models: [{ name: "User", guarded: null }],
  });
  assert.equal(v.clean, true);
  assert.deepEqual(v.problems, []);
});

test("dead routes surface one problem item that links to Findings", () => {
  const v = healthVerdict({
    dead_routes: [
      { method: "GET", uri: "/a" },
      { method: "POST", uri: "/b" },
    ],
    disagreements: [],
    models: [],
  });
  assert.equal(v.clean, false);
  assert.equal(v.problems.length, 1);
  const [p] = v.problems;
  assert.equal(p.kind, "dead_routes");
  assert.equal(p.count, 2);
  assert.equal(p.view, "findings");
});

test("unguarded counts only models with an explicit empty $guarded, not omitted", () => {
  // guarded === [] is the risky explicit `$guarded = []` escape hatch and IS a
  // finding; guarded === null (property omitted) is guarded-by-omission and is
  // NOT. A non-empty guarded list is also fine. Only the two [] models count.
  const v = healthVerdict({
    dead_routes: [],
    disagreements: [],
    models: [
      { name: "Open1", guarded: [] },
      { name: "Open2", guarded: [] },
      { name: "Safe", guarded: null },
      { name: "Partial", guarded: ["id"] },
    ],
  });
  assert.equal(v.problems.length, 1);
  const [p] = v.problems;
  assert.equal(p.kind, "unguarded");
  assert.equal(p.count, 2);
  assert.equal(p.view, "findings");
});

test("all three categories itemize in a fixed dead/disagreements/unguarded order", () => {
  const v = healthVerdict({
    dead_routes: [{ method: "GET", uri: "/x" }],
    disagreements: [{ model: "Post" }, { model: "Tag" }, { model: "User" }],
    models: [{ name: "Open", guarded: [] }],
  });
  assert.equal(v.clean, false);
  assert.deepEqual(
    v.problems.map((p) => [p.kind, p.count]),
    [
      ["dead_routes", 1],
      ["disagreements", 3],
      ["unguarded", 1],
    ]
  );
});

test("each problem carries a human label with correct singular/plural wording", () => {
  const one = healthVerdict({
    dead_routes: [{ method: "GET", uri: "/x" }],
    disagreements: [{ model: "Post" }],
    models: [{ name: "Open", guarded: [] }],
  });
  assert.deepEqual(
    one.problems.map((p) => p.label),
    ["1 dead route", "1 disagreement", "1 unguarded model"]
  );

  const many = healthVerdict({
    dead_routes: [{ uri: "/x" }, { uri: "/y" }],
    disagreements: [{ model: "A" }, { model: "B" }],
    models: [
      { name: "O1", guarded: [] },
      { name: "O2", guarded: [] },
    ],
  });
  assert.deepEqual(
    many.problems.map((p) => p.label),
    ["2 dead routes", "2 disagreements", "2 unguarded models"]
  );
});
