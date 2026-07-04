// Unit tests for the ELK-input builder (issue #26).
//
// buildElkGraph is the pure transform from the server's ER graph contract
// ({nodes, edges}, see internal/render/er/graph.go) to the ELK "layered"
// input graph the browser feeds to elk.layout(). It is where the load-bearing
// correctness lives — node sizing, one fixed port per column row, and FK edges
// routed to the EXACT referencing column's port — so it is tested directly
// with no DOM and no ELK (ELK only turns this input into coordinates).
//
//   node --test 'internal/web/jstest/**/*.test.js'

import { test } from "node:test";
import assert from "node:assert/strict";

import {
  buildElkGraph,
  portId,
  HEADER_HEIGHT,
  ROW_HEIGHT,
  nodeHeight,
} from "../assets/js/views/er-graph.js";

// A one-table graph: the smallest true fact — one ELK node, one port per
// column, sized so every column row fits.
const oneTable = {
  nodes: [
    {
      table: "users",
      columns: [
        { name: "id", type: "bigInteger", key: "PK" },
        { name: "name", type: "string", key: "" },
      ],
    },
  ],
  edges: [],
};

test("port ids never collide across distinct (table, column) pairs", () => {
  // Table and column names are arbitrary strings parsed from PHP source, not
  // validated SQL identifiers — a name may itself contain the delimiter. Two
  // distinct pairs must still get distinct port ids, or ELK aliases their ports
  // and an FK edge silently attaches to the wrong column.
  assert.notEqual(portId("a", "b::c"), portId("a::b", "c"));
  assert.notEqual(portId("x", ""), portId("", "x"));
  // The id must round-trip its pair unambiguously.
  assert.equal(portId("users", "id"), portId("users", "id"));
});

test("each table becomes one ELK node carrying a port per column", () => {
  const elk = buildElkGraph(oneTable);

  assert.equal(elk.children.length, 1);
  const node = elk.children[0];
  assert.equal(node.id, "users");
  assert.equal(node.ports.length, 2);
  assert.deepEqual(
    node.ports.map((p) => p.id),
    [portId("users", "id"), portId("users", "name")]
  );
});

test("an empty or missing graph yields an empty ELK graph, not a throw", () => {
  for (const input of [null, undefined, {}, { nodes: [], edges: [] }]) {
    const elk = buildElkGraph(input);
    assert.deepEqual(elk.children, []);
    assert.deepEqual(elk.edges, []);
  }
});

test("a node is sized for its header plus one row per column", () => {
  const node = buildElkGraph(oneTable).children[0];
  // Two columns: header + 2 rows.
  assert.equal(node.height, HEADER_HEIGHT + 2 * ROW_HEIGHT);
  assert.equal(node.height, nodeHeight(2));
  assert.ok(node.width > 0);
});

// The fixture's shape (internal/render/er/testdata/golden.graph.json): a
// schema FK "references (author_id)" from users to posts, an Eloquent hasMany,
// a belongsToMany, and a belongsTo — including two distinct edges between the
// users/posts pair.
const twoTables = {
  nodes: [
    { table: "users", columns: [{ name: "id", type: "bigInteger", key: "PK" }] },
    {
      table: "posts",
      columns: [
        { name: "id", type: "bigInteger", key: "PK" },
        { name: "author_id", type: "bigInteger", key: "FK" },
      ],
    },
  ],
  edges: [
    { from: "users", to: "posts", kind: "one-to-many", label: "references (author_id)" },
    { from: "users", to: "posts", kind: "one-to-many", label: "posts (hasMany)" },
  ],
};

test("a schema FK edge targets the exact referencing column's port", () => {
  const elk = buildElkGraph(twoTables);
  const fk = elk.edges.find((e) => e.labels[0].text === "references (author_id)");

  // The referencing column author_id lives on the child (to) table posts, so
  // the edge attaches to that column's port — not the whole box.
  assert.deepEqual(fk.targets, [portId("posts", "author_id")]);
  // The parent (from) side has no named column; it attaches at the box (node id).
  assert.deepEqual(fk.sources, ["users"]);
});

test("every edge becomes a distinct ELK edge with a stable id", () => {
  const elk = buildElkGraph(twoTables);
  // Two edges between the same pair must both survive as separate edges.
  assert.equal(elk.edges.length, 2);
  const ids = elk.edges.map((e) => e.id);
  assert.equal(new Set(ids).size, 2, "edge ids must be unique");
});

test("an Eloquent edge with no referencing column attaches box-to-box", () => {
  const elk = buildElkGraph(twoTables);
  const rel = elk.edges.find((e) => e.labels[0].text === "posts (hasMany)");
  assert.deepEqual(rel.sources, ["users"]);
  assert.deepEqual(rel.targets, ["posts"]);
});

test("ports use fixed positions so a column row is addressable for edges", () => {
  const node = buildElkGraph(oneTable).children[0];
  // FIXED_POS lets FK edges attach to the exact referencing column's row,
  // and the SVG emitter draws each row at the port's y.
  assert.equal(node.layoutOptions["elk.portConstraints"], "FIXED_POS");

  // Each port sits at the vertical center of its column row, below the header.
  const [idPort, namePort] = node.ports;
  assert.equal(idPort.y, HEADER_HEIGHT + 0.5 * ROW_HEIGHT);
  assert.equal(namePort.y, HEADER_HEIGHT + 1.5 * ROW_HEIGHT);
});
