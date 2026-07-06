// Unit tests for the interactive layer over the Overview constellation (issue
// #52): assigning a real entity to each drawn dot, hit-testing a pointer to the
// nearest dot, and building the visually-hidden entity list that gives the map
// a keyboard/AT path. Run via `node --test` (see js_test.go). Pure geometry +
// mapping only — the canvas/pointer glue in overview-wow.js stays untested, the
// same split as overview-constellation.

import { test } from "node:test";
import assert from "node:assert/strict";
import { buildConstellation } from "../assets/js/views/overview-constellation.js";
import { assignEntities, hitTest, entityListHtml } from "../assets/js/views/overview-hit.js";

// A small many-of-each project so every kind has both drawn dots and a real
// name list to distribute across them.
const ENTITIES = {
  tables: ["users", "posts", "categories"],
  models: ["User", "Post", "Category"],
  controllers: ["UserController", "PostController"],
  routes: ["GET /posts", "POST /posts"],
  requests: ["StorePostRequest"],
};
const COUNTS = {
  tables: ENTITIES.tables.length,
  models: ENTITIES.models.length,
  controllers: ENTITIES.controllers.length,
  routes: ENTITIES.routes.length,
  requests: ENTITIES.requests.length,
};

test("every drawn dot is assigned a real entity of its own kind", () => {
  const { nodes } = buildConstellation(COUNTS, 900, 520);
  const assigned = assignEntities(nodes, ENTITIES);

  const kindKeys = ["tables", "models", "controllers", "routes", "requests"];
  for (const n of assigned) {
    const names = ENTITIES[kindKeys[n.kind]];
    assert.ok(
      names.includes(n.entityName),
      `node of kind ${n.kind} got "${n.entityName}", not one of that kind's entities`
    );
  }
});

test("deterministic: the same nodes and entities name the same dots", () => {
  const { nodes } = buildConstellation(COUNTS, 900, 520);
  const a = assignEntities(nodes, ENTITIES);
  const b = assignEntities(nodes, ENTITIES);
  assert.deepEqual(
    a.map((n) => n.entityName),
    b.map((n) => n.entityName)
  ); // no RNG in the binding — every replay names the same sky
});

test("spread preserves source order: first drawn dot names the first entity, last names the last", () => {
  const { nodes } = buildConstellation(COUNTS, 900, 520);
  const assigned = assignEntities(nodes, ENTITIES);
  // The models kind (index 1) draws several dots for its 3 names; the first
  // drawn model dot must name ENTITIES.models[0], the last name ENTITIES.models[2].
  const modelDots = assigned.filter((n) => n.entityKind === "models");
  assert.ok(modelDots.length >= 3, "expected several model dots to spread across");
  assert.equal(modelDots[0].entityName, "User");
  assert.equal(modelDots[modelDots.length - 1].entityName, "Category");
});

// hitTest maps a pointer position onto the nearest drawn dot within a radius —
// the resolved end-state uses each node's settled tx/ty, since interaction only
// begins after the settle completes.
const DOTS = [
  { tx: 100, ty: 100, size: 3, entityName: "A", entityKind: "tables" },
  { tx: 300, ty: 300, size: 3, entityName: "B", entityKind: "models" },
];

test("hitTest returns the dot under the pointer", () => {
  const hit = hitTest(DOTS, 302, 298);
  assert.equal(hit && hit.entityName, "B");
});

test("hitTest returns null over empty sky", () => {
  const hit = hitTest(DOTS, 200, 200); // equidistant from both, far from either
  assert.equal(hit, null);
});

test("hitTest picks the nearer of two dots", () => {
  const hit = hitTest(DOTS, 108, 104); // just off A, nowhere near B
  assert.equal(hit && hit.entityName, "A");
});

// entityListHtml builds the visually-hidden, focusable, linked list that gives
// the constellation a keyboard/AT path so the map is not mouse-only. Each model
// is an <a> to its detail page; other kinds are named but not linked (only
// models have a detail page, mirroring chipTarget). Pure (string in/out) so the
// markup contract is unit-testable, the same split as modelDetailHtml.

test("entityListHtml links every model to its detail page", () => {
  const html = entityListHtml(ENTITIES);
  for (const name of ENTITIES.models) {
    assert.ok(
      html.includes('href="#/models/' + name + '"'),
      `model ${name} is not linked to its detail page`
    );
    assert.ok(
      html.includes('data-view="models"'),
      "model link is missing data-view for the delegated router handler"
    );
  }
});

test("entityListHtml names non-model entities without a model detail link", () => {
  const html = entityListHtml(ENTITIES);
  // A table is named in the list...
  assert.ok(html.includes("users"), "table name is missing from the AT list");
  // ...but never linked to a models detail page (no detail page exists for it;
  // only models navigate, matching chipTarget and the confirmed contract).
  assert.ok(
    !html.includes('href="#/models/users"'),
    "a non-model entity must not link to a model detail page"
  );
});
