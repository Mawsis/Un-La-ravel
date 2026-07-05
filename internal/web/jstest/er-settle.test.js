// Unit tests for the ER settle animation's pure geometry (issue #30).
//
// The settle is the "un-ravel" metaphor at the one moment it's literally true:
// on first render, entity boxes ease from a rough, unresolved arrangement into
// the exact positions ELK laid out. The DOM choreography (applying transforms,
// the rAF/transition, the once-per-analysis gate, reduced-motion) lives in
// er.js; the load-bearing GEOMETRY — where a node starts before it resolves —
// is pure and tested here without a browser.
//
//   node --test 'internal/web/jstest/**/*.test.js'

import { test } from "node:test";
import assert from "node:assert/strict";

import { settleOffset, diagramCenter } from "../assets/js/views/er-settle.js";

// The diagram's center, which resolved nodes settle toward. A node's rough
// start is displaced OUTWARD from center (tangled/scattered), then it eases in
// to its laid-out spot — so the offset points away from center.
const center = { x: 500, y: 300 };

test("a node left/above center starts further left/above (displaced outward)", () => {
  // Resolved position is up-and-left of center → the rough start is even more
  // up-and-left, so the settle motion travels down-and-right into place.
  const off = settleOffset({ x: 100, y: 50, width: 220, height: 78 }, center);
  assert.ok(off.dx < 0, "expected leftward outward displacement");
  assert.ok(off.dy < 0, "expected upward outward displacement");
});

test("a node right/below center starts further right/below", () => {
  const off = settleOffset({ x: 900, y: 500, width: 220, height: 78 }, center);
  assert.ok(off.dx > 0, "expected rightward outward displacement");
  assert.ok(off.dy > 0, "expected downward outward displacement");
});

test("a node centered on the diagram doesn't move (no jitter at the origin)", () => {
  // A box whose center IS the diagram center has nowhere outward to go — its
  // settle offset is exactly zero, not a NaN or a nudge.
  const off = settleOffset({ x: 390, y: 261, width: 220, height: 78 }, center);
  assert.equal(off.dx, 0);
  assert.equal(off.dy, 0);
});

test("diagramCenter is the midpoint of the laid-out graph bounds", () => {
  // The center nodes settle toward is derived from ELK's overall size, so the
  // whole diagram converges on its own middle regardless of pan/zoom.
  assert.deepEqual(diagramCenter({ width: 1000, height: 600 }), { x: 500, y: 300 });
});
