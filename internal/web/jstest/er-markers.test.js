// Unit tests for crow's-foot cardinality markers (issue #26).
//
// edgeMarkers is a pure map from a graph edge's cardinality kind (the four
// Eloquent kinds plus the schema-FK one-to-many, see internal/render/er/
// graph.go) to the marker at each END of the drawn edge: which side reads
// "one" and which reads "many". Keeping it pure means the crow's-foot
// correctness — the part a reader actually decodes the relationship from — is
// tested without drawing anything; er-svg.js only turns these names into paths.
//
//   node --test 'internal/web/jstest/**/*.test.js'

import { test } from "node:test";
import assert from "node:assert/strict";

import { edgeMarkers, ONE, MANY } from "../assets/js/views/er-markers.js";

test("schema FK / hasMany one-to-many reads one at the parent, many at the child", () => {
  // from=parent (the "one"), to=child (the "many" crow's foot).
  assert.deepEqual(edgeMarkers("one-to-many"), { from: ONE, to: MANY });
});

test("belongsTo many-to-one reads many at the child, one at the parent", () => {
  assert.deepEqual(edgeMarkers("many-to-one"), { from: MANY, to: ONE });
});

test("hasOne one-to-one reads one at both ends", () => {
  assert.deepEqual(edgeMarkers("one-to-one"), { from: ONE, to: ONE });
});

test("belongsToMany many-to-many reads a crow's foot at both ends", () => {
  assert.deepEqual(edgeMarkers("many-to-many"), { from: MANY, to: MANY });
});

test("an unknown kind falls back to plain (no marker) rather than throwing", () => {
  // A future contract kind we don't know how to draw still renders a line,
  // just without cardinality decoration — never a crash.
  assert.deepEqual(edgeMarkers("who-knows"), { from: null, to: null });
});
