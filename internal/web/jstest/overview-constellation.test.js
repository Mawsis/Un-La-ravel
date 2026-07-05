// Unit tests for the pure constellation geometry behind the Overview wow
// moment (issue #39 variant A, committed via issue #44). Run via `node --test`
// (see js_test.go). Like er-settle, only the pure math is tested: the canvas
// choreography in overview-wow.js is DOM/rAF glue and stays untested.

import { test } from "node:test";
import assert from "node:assert/strict";
import { buildConstellation } from "../assets/js/views/overview-constellation.js";

const COUNTS = { tables: 24, models: 21, controllers: 15, routes: 68, requests: 9 };

test("deterministic: the same inputs produce the identical constellation", () => {
  const a = buildConstellation(COUNTS, 900, 520);
  const b = buildConstellation(COUNTS, 900, 520);
  assert.deepEqual(a, b); // seeded RNG — every replay is the same take (GIF-friendly)
});

test("subsamples to the node budget so any project size stays legible", () => {
  const big = buildConstellation(
    { tables: 500, models: 400, controllers: 300, routes: 2000, requests: 100 },
    900,
    520
  );
  assert.ok(big.nodes.length <= 130, `node budget blown: ${big.nodes.length}`);
  // Every kind is still represented — subsampling drops density, not kinds.
  const kinds = new Set(big.nodes.map((n) => n.kind));
  assert.equal(kinds.size, 5);
});

test("every node lands inside the stage bounds", () => {
  const { nodes } = buildConstellation(COUNTS, 900, 520);
  for (const n of nodes) {
    assert.ok(n.tx >= 0 && n.tx <= 900, `tx out of bounds: ${n.tx}`);
    assert.ok(n.ty >= 0 && n.ty <= 520, `ty out of bounds: ${n.ty}`);
  }
});

test("edges connect real nodes (never an orphan thread)", () => {
  const { nodes, edges } = buildConstellation(COUNTS, 900, 520);
  assert.ok(edges.length > 0, "a constellation with no threads is just dots");
  for (const [a, b] of edges) {
    assert.ok(Number.isInteger(a) && a >= 0 && a < nodes.length, `edge endpoint ${a} is not a node`);
    assert.ok(Number.isInteger(b) && b >= 0 && b < nodes.length, `edge endpoint ${b} is not a node`);
  }
});

test("an empty project still produces a drawable constellation", () => {
  const { nodes, edges } = buildConstellation(
    { tables: 0, models: 0, controllers: 0, routes: 0, requests: 0 },
    900,
    520
  );
  assert.ok(Array.isArray(nodes) && Array.isArray(edges));
  for (const n of nodes) {
    assert.ok(Number.isFinite(n.tx) && Number.isFinite(n.ty), "no NaN positions on empty input");
  }
});
