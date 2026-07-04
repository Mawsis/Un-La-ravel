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
// healthVerdict is now a THIN READER of the server-computed findings array
// (model.findings), not a recomputation from raw counts: the verdict is computed
// once, server-side, into the unlaravel.json contract (internal/findings) so the
// CLI, the JSON, and the dashboard agree by construction. These tests assert the
// *observable* verdict — clean vs. problems, the itemized breakdown, and that each
// item carries its finding link — against a model whose findings the server
// supplied. They never assert DOM structure or class names.

import { test } from "node:test";
import assert from "node:assert/strict";

import { healthVerdict } from "../assets/js/verdict.js";

test("a clean project (empty findings) yields a clean verdict with no problems", () => {
  const v = healthVerdict({ findings: [] });
  assert.equal(v.clean, true);
  assert.deepEqual(v.problems, []);
});

test("a model with no findings key at all is treated as clean", () => {
  const v = healthVerdict({});
  assert.equal(v.clean, true);
  assert.deepEqual(v.problems, []);
});

test("server findings surface as problem items that link to Findings", () => {
  const v = healthVerdict({
    findings: [
      { kind: "dead_routes", count: 2, label: "2 dead routes", view: "findings" },
    ],
  });
  assert.equal(v.clean, false);
  assert.equal(v.problems.length, 1);
  const [p] = v.problems;
  assert.equal(p.kind, "dead_routes");
  assert.equal(p.count, 2);
  assert.equal(p.label, "2 dead routes");
  assert.equal(p.view, "findings");
});

test("all server findings itemize in the order the server emitted them", () => {
  const v = healthVerdict({
    findings: [
      { kind: "dead_routes", count: 1, label: "1 dead route", view: "findings" },
      { kind: "disagreements", count: 3, label: "3 disagreements", view: "findings" },
      { kind: "unguarded", count: 1, label: "1 unguarded model", view: "findings" },
    ],
  });
  assert.equal(v.clean, false);
  assert.deepEqual(
    v.problems.map((p) => [p.kind, p.count, p.label]),
    [
      ["dead_routes", 1, "1 dead route"],
      ["disagreements", 3, "3 disagreements"],
      ["unguarded", 1, "1 unguarded model"],
    ]
  );
});

test("the verdict READS findings and does not recompute from raw counts", () => {
  // The server is authoritative: even though dead_routes and an unguarded model
  // are populated on the model, the (empty) findings array is what the verdict
  // reflects. This proves the browser no longer re-derives the verdict — a raw
  // model that disagrees with its findings follows the findings.
  const v = healthVerdict({
    findings: [],
    dead_routes: [{ method: "GET", uri: "/x" }],
    disagreements: [{ model: "Post" }],
    models: [{ name: "Open", guarded: [] }],
  });
  assert.equal(v.clean, true);
  assert.deepEqual(v.problems, []);
});
