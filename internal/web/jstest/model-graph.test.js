// Unit tests for the model-centric graph builder (issue #70).
//
// This is a DISTINCT graph from the ER diagram's. The ER diagram is
// table-centric: its nodes are Schema tables and its edges are foreign keys
// plus Eloquent associations projected onto tables. The Model graph is
// model-centric: its nodes are Eloquent Models and its edges are Relationships
// labeled by kind (hasMany, belongsTo, ...), centered on ONE model. Conflating
// the two would be a Schema/Model conflation the domain glossary forbids, so
// the tests below pin the model-graph facts specifically: the center, its
// direct neighbours in both directions, the kind labels, and the one-hop bound.
//
// buildModelGraph is pure (model name + Project Model in, {nodes, edges} out),
// so the graph's shape is tested without ELK and without a DOM; ELK only turns
// the graph into coordinates later.

import { test } from "node:test";
import assert from "node:assert/strict";

import { buildModelGraph } from "../assets/js/views/model-graph.js";

function projectModel(models) {
  return { models };
}

test("the named model is the center node", () => {
  const g = buildModelGraph("Post", projectModel([{ name: "Post", table: "posts", relationships: [] }]));

  assert.equal(g.center, "Post");
  assert.deepEqual(g.nodes.map((n) => n.name), ["Post"]);
  assert.equal(g.nodes[0].center, true);
});

test("forward relations become edges from the center to their target models", () => {
  const g = buildModelGraph(
    "Post",
    projectModel([
      {
        name: "Post",
        table: "posts",
        relationships: [
          { kind: "belongsTo", method: "author", target: "User" },
          { kind: "hasMany", method: "comments", target: "Comment" },
        ],
      },
      { name: "User", table: "users", relationships: [] },
      { name: "Comment", table: "comments", relationships: [] },
    ])
  );

  assert.deepEqual(
    g.edges.map((e) => ({ from: e.from, to: e.to, label: e.label, direction: e.direction })),
    [
      { from: "Post", to: "User", label: "belongsTo", direction: "outbound" },
      { from: "Post", to: "Comment", label: "hasMany", direction: "outbound" },
    ]
  );
  // Every edge endpoint is a node — the invariant ELK requires (issue #36).
  assert.deepEqual(g.nodes.map((n) => n.name).sort(), ["Comment", "Post", "User"]);
});

test("an edge carries the relationship METHOD as well as the kind", () => {
  // The kind is the edge label (what the diagram draws), but the method is the
  // thing the reader greps for in the source, so it must survive to the node.
  const g = buildModelGraph(
    "Post",
    projectModel([
      { name: "Post", table: "posts", relationships: [{ kind: "belongsTo", method: "author", target: "User" }] },
      { name: "User", table: "users", relationships: [] },
    ])
  );

  assert.equal(g.edges[0].method, "author");
});

test("inbound relations from other models become edges into the center", () => {
  const g = buildModelGraph(
    "User",
    projectModel([
      { name: "User", table: "users", relationships: [] },
      { name: "Post", table: "posts", relationships: [{ kind: "belongsTo", method: "author", target: "User" }] },
    ])
  );

  assert.deepEqual(
    g.edges.map((e) => ({ from: e.from, to: e.to, direction: e.direction })),
    [{ from: "Post", to: "User", direction: "inbound" }]
  );
  assert.deepEqual(g.nodes.map((n) => n.name).sort(), ["Post", "User"]);
});

test("the graph is one hop only — a neighbour's own unrelated relations are excluded", () => {
  // Post → User, and User → Profile. Profile is TWO hops from Post and must not
  // appear: the page is about one model's direct bindings, and pulling in the
  // transitive closure would redraw the whole ER diagram under another name.
  const g = buildModelGraph(
    "Post",
    projectModel([
      { name: "Post", table: "posts", relationships: [{ kind: "belongsTo", method: "author", target: "User" }] },
      { name: "User", table: "users", relationships: [{ kind: "hasOne", method: "profile", target: "Profile" }] },
      { name: "Profile", table: "profiles", relationships: [] },
    ])
  );

  assert.deepEqual(g.nodes.map((n) => n.name).sort(), ["Post", "User"]);
  assert.equal(g.edges.length, 1);
});

test("a self-referential relation is one node with a self-edge", () => {
  // Adjacency list: Category belongsTo Category. A second "Category" node would
  // make ELK draw the same model twice; the edge must loop on the one node.
  const g = buildModelGraph(
    "Category",
    projectModel([
      {
        name: "Category",
        table: "categories",
        relationships: [{ kind: "belongsTo", method: "parent", target: "Category" }],
      },
    ])
  );

  assert.deepEqual(g.nodes.map((n) => n.name), ["Category"]);
  assert.equal(g.edges.length, 1);
  assert.equal(g.edges[0].from, "Category");
  assert.equal(g.edges[0].to, "Category");
});

test("a relation targeting an unknown model still gets a node, marked unresolved", () => {
  // The target names a class the extractor never found (a package model, a
  // typo). Dropping the edge would hide a real declared relation; inventing a
  // fully-fledged node would overstate what is known — so the node exists and
  // says it is unresolved, the same honesty the ER edge contract carries.
  const g = buildModelGraph(
    "Post",
    projectModel([
      { name: "Post", table: "posts", relationships: [{ kind: "belongsTo", method: "owner", target: "Tenant" }] },
    ])
  );

  const tenant = g.nodes.find((n) => n.name === "Tenant");
  assert.ok(tenant, "the unknown target still gets a node so the edge has an endpoint");
  assert.equal(tenant.unresolved, true);
  assert.equal(tenant.table, "");
  assert.equal(g.edges.length, 1);
});

test("a model absent from the project yields an empty graph, not a crash", () => {
  const g = buildModelGraph("Ghost", projectModel([{ name: "Post", table: "posts", relationships: [] }]));

  assert.equal(g.found, false);
  assert.deepEqual(g.nodes, []);
  assert.deepEqual(g.edges, []);
});

test("two relations between the same pair stay two distinct edges", () => {
  // A Post has an author AND an editor, both Users. Deduping by endpoint pair
  // would silently drop one declared relationship.
  const g = buildModelGraph(
    "Post",
    projectModel([
      {
        name: "Post",
        table: "posts",
        relationships: [
          { kind: "belongsTo", method: "author", target: "User" },
          { kind: "belongsTo", method: "editor", target: "User" },
        ],
      },
      { name: "User", table: "users", relationships: [] },
    ])
  );

  assert.equal(g.edges.length, 2);
  assert.deepEqual(g.edges.map((e) => e.method), ["author", "editor"]);
  // ...but still ONE User node.
  assert.equal(g.nodes.filter((n) => n.name === "User").length, 1);
});

test("node and edge order is deterministic: center first, then discovery order", () => {
  // Determinism is a hard convention (CLAUDE.md) — no Set/Map iteration order
  // leaking into the output, and the center always drawn first.
  const g = buildModelGraph(
    "Post",
    projectModel([
      { name: "Zeta", table: "zetas", relationships: [{ kind: "belongsTo", method: "post", target: "Post" }] },
      {
        name: "Post",
        table: "posts",
        relationships: [{ kind: "hasMany", method: "comments", target: "Comment" }],
      },
      { name: "Comment", table: "comments", relationships: [] },
    ])
  );

  // Center, then outbound targets in declaration order, then inbound sources in
  // model-discovery order.
  assert.deepEqual(g.nodes.map((n) => n.name), ["Post", "Comment", "Zeta"]);
  assert.deepEqual(g.edges.map((e) => e.direction), ["outbound", "inbound"]);
});

test("a neighbour node carries its mapped table so the page can chip through to it", () => {
  const g = buildModelGraph(
    "Post",
    projectModel([
      { name: "Post", table: "posts", relationships: [{ kind: "belongsTo", method: "author", target: "User" }] },
      { name: "User", table: "app_users", relationships: [] },
    ])
  );

  assert.equal(g.nodes.find((n) => n.name === "User").table, "app_users");
});
