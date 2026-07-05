// Unit tests for the ER SVG emitter (issue #26).
//
// renderSvg is the pure transform from an ELK-laid-out graph (coordinates +
// edge sections) plus the source {nodes, edges} contract into the SVG markup
// the browser mounts. It owns no layout math (ELK did that) and touches no DOM
// — it returns a string — so what gets DRAWN (entity boxes, typed column rows,
// PK/FK/UK badges, edge polylines, crow's-foot markers, the data-table hook
// focus resolves against) is asserted here without a browser.
//
//   node --test 'internal/web/jstest/**/*.test.js'

import { test } from "node:test";
import assert from "node:assert/strict";

import { renderSvg } from "../assets/js/views/er-svg.js";

// A single laid-out table. `layout` is shaped like ELK's output; `graph` is the
// source contract renderSvg reads columns/keys from (ELK output drops them).
const oneNode = {
  layout: {
    width: 220,
    height: 78,
    children: [{ id: "users", x: 10, y: 20, width: 220, height: 78 }],
    edges: [],
  },
  graph: {
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
  },
};

test("emits an <svg> sized to the laid-out graph", () => {
  const svg = renderSvg(oneNode.layout, oneNode.graph);
  assert.match(svg, /^<svg\b/);
  assert.match(svg, /viewBox="0 0 220 78"/);
});

test("draws one entity group per table, tagged with data-table for focus", () => {
  const svg = renderSvg(oneNode.layout, oneNode.graph);
  // The focus id-lookup finds a table by this attribute — a fact the renderer
  // OWNS, not a reverse-engineered library-internal id.
  assert.match(svg, /data-table="users"/);
  // Positioned at the coordinates ELK assigned.
  assert.match(svg, /translate\(10,\s*20\)/);
});

test("renders the table name and every column with its type", () => {
  const svg = renderSvg(oneNode.layout, oneNode.graph);
  assert.match(svg, />users</);
  assert.match(svg, />id</);
  assert.match(svg, />bigInteger</);
  assert.match(svg, />name</);
  assert.match(svg, />string</);
});

test("draws a PK/FK/UK badge only on keyed columns", () => {
  const svg = renderSvg(oneNode.layout, oneNode.graph);
  // id is a PK; name has no key marker.
  assert.match(svg, /er-badge-pk[^>]*>PK</);
  // Exactly one badge in this two-column table (name has none).
  assert.equal((svg.match(/class="er-badge/g) || []).length, 1);
});

test("escapes attacker-shaped table and column names in every context", () => {
  // Table/column names are arbitrary strings parsed from PHP source, so they
  // can carry HTML-special characters. They must never break out of the
  // attribute (data-table="...") or text contexts they're interpolated into.
  const hostile = {
    layout: {
      width: 220,
      height: 56,
      children: [{ id: 'Robert"); <x>', x: 0, y: 0, width: 220, height: 56 }],
      edges: [],
    },
    graph: {
      nodes: [
        {
          table: 'Robert"); <x>',
          columns: [{ name: '<img src=x>', type: 'a&b"c', key: "PK" }],
        },
      ],
    },
  };
  const svg = renderSvg(hostile.layout, hostile.graph);

  // No raw markup-significant characters from the injected names survive.
  assert.ok(!svg.includes("<x>"), "table name broke out of its context");
  assert.ok(!svg.includes("<img src=x>"), "column name broke out of its context");
  // The attribute value's own quote is escaped, not left to close data-table.
  assert.ok(!/data-table="Robert"\)/.test(svg), "attribute quote not escaped");
  // The escaped forms are present instead.
  assert.match(svg, /&lt;img src=x&gt;/);
  assert.match(svg, /a&amp;b&quot;c/);
});

// A two-node layout with one FK edge routed by ELK: a polyline plus a
// crow's-foot at the "many" end.
const withEdge = {
  layout: {
    width: 300,
    height: 120,
    children: [
      { id: "users", x: 0, y: 40, width: 220, height: 56 },
      { id: "posts", x: 250, y: 0, width: 220, height: 78 },
    ],
    edges: [
      {
        id: "e0",
        sections: [
          {
            startPoint: { x: 220, y: 68 },
            bendPoints: [{ x: 235, y: 68 }],
            endPoint: { x: 250, y: 40 },
          },
        ],
      },
    ],
  },
  graph: {
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
    edges: [{ from: "users", to: "posts", kind: "one-to-many", label: "references (author_id)" }],
  },
};

test("draws each edge as a polyline through ELK's start, bend, and end points", () => {
  const svg = renderSvg(withEdge.layout, withEdge.graph);
  // The routed path visits every point ELK produced, in order.
  assert.match(svg, /<polyline[^>]*points="220,68 235,68 250,40"/);
});

test("marks the edge ends with the cardinality's crow's-foot markers", () => {
  const svg = renderSvg(withEdge.layout, withEdge.graph);
  // one-to-many: a "one" tick at the from (users) end, a crow's foot at the
  // to (posts) end. Markers are referenced via marker-start / marker-end.
  assert.match(svg, /marker-start="url\(#er-marker-one\)"/);
  assert.match(svg, /marker-end="url\(#er-marker-many\)"/);
  // The marker <defs> the polyline references must actually be defined.
  assert.match(svg, /<marker id="er-marker-one"/);
  assert.match(svg, /<marker id="er-marker-many"/);
});

test("schema-FK and Eloquent edges carry distinguishing classes", () => {
  // A schema FK ("references (...)") and an Eloquent relation over the same
  // pair must be visually separable, not collapsed into one look.
  const two = {
    layout: {
      width: 100,
      height: 100,
      children: [],
      edges: [
        { id: "e0", sections: [{ startPoint: { x: 0, y: 0 }, endPoint: { x: 1, y: 1 } }] },
        { id: "e1", sections: [{ startPoint: { x: 0, y: 0 }, endPoint: { x: 1, y: 1 } }] },
      ],
    },
    graph: {
      nodes: [],
      edges: [
        { from: "users", to: "posts", kind: "one-to-many", label: "references (author_id)" },
        { from: "users", to: "posts", kind: "one-to-many", label: "posts (hasMany)" },
      ],
    },
  };
  const svg = renderSvg(two.layout, two.graph);
  assert.match(svg, /class="er-edge er-edge-schema"/);
  assert.match(svg, /class="er-edge er-edge-eloquent"/);
});

test("an unresolved edge carries er-edge-unresolved; resolved edges never do", () => {
  // The edge-reconciliation contract (issue #36) flags edges the tool had to
  // repair with unresolved: true; the reskin (issue #38) styles that flag red
  // vs the resolved cyan. Edges without the flag — including older models
  // that predate it — must render as resolved.
  const g = {
    layout: {
      width: 100,
      height: 100,
      children: [],
      edges: [
        { id: "e0", sections: [{ startPoint: { x: 0, y: 0 }, endPoint: { x: 1, y: 1 } }] },
        { id: "e1", sections: [{ startPoint: { x: 0, y: 0 }, endPoint: { x: 1, y: 1 } }] },
      ],
    },
    graph: {
      nodes: [],
      edges: [
        { from: "users", to: "posts", kind: "one-to-many", label: "references (author_id)" },
        { from: "users", to: "commentz", kind: "one-to-many", label: "comments (hasMany)", unresolved: true },
      ],
    },
  };
  const svg = renderSvg(g.layout, g.graph);
  assert.match(svg, /class="er-edge er-edge-eloquent er-edge-unresolved"/);
  // The resolved schema edge carries no unresolved marker.
  assert.match(svg, /class="er-edge er-edge-schema"/);
  assert.equal((svg.match(/er-edge-unresolved/g) || []).length, 1);
});
