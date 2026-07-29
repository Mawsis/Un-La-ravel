// Unit tests for the model detail page compose logic (issue #51).
//
// The detail page (#/models/{name}) is composed entirely client-side over the
// already-loaded Project Model — no server endpoint. composeModelDetail is the
// pure function at its heart: given a model name and the whole Project Model, it
// derives everything the page shows (bidirectional relations, owning routes,
// form requests, table schema, scoped findings). It is a pure function of its
// inputs so the compose logic is unit-testable without a DOM, the same split as
// findingsList in findings.js and routeRowHtml in routes.js.
//
// Run under Node's zero-dependency built-in test runner (no build step, no
// node_modules) — the runner in internal/web/js_test.go invokes this under
// `go test`.

import { test } from "node:test";
import assert from "node:assert/strict";

import { composeModelDetail } from "../assets/js/views/model-detail.js";

// A minimal Project Model fixture builder: only the fields the compose logic
// reads, so each test states exactly the shape it depends on.
function projectModel(overrides) {
  return {
    models: [],
    schemas: [],
    routes: [],
    form_requests: [],
    disagreements: [],
    dead_routes: [],
    ...overrides,
  };
}

test("composeModelDetail returns the model's own forward relations", () => {
  const pm = projectModel({
    models: [
      {
        name: "Post",
        table: "posts",
        relationships: [
          { kind: "belongsTo", method: "author", target: "User" },
          { kind: "hasMany", method: "comments", target: "Comment" },
        ],
      },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.name, "Post");
  assert.deepEqual(
    detail.forwardRelations.map((r) => ({ method: r.method, target: r.target, kind: r.kind })),
    [
      { method: "author", target: "User", kind: "belongsTo" },
      { method: "comments", target: "Comment", kind: "hasMany" },
    ]
  );
});

test("reverse relations are derived by inverting other models' relations that target this one", () => {
  const pm = projectModel({
    models: [
      { name: "User", table: "users", relationships: [] },
      {
        name: "Post",
        table: "posts",
        relationships: [{ kind: "belongsTo", method: "author", target: "User" }],
      },
      {
        name: "Comment",
        table: "comments",
        relationships: [{ kind: "belongsTo", method: "poster", target: "User" }],
      },
    ],
  });

  const detail = composeModelDetail("User", pm);

  // User declares nothing forward, but Post.author and Comment.poster both
  // point at it — so its reverse relations name the source model, the method
  // that reaches User, and that method's kind. Source/discovery order preserved.
  assert.deepEqual(
    detail.reverseRelations.map((r) => ({ from: r.from, method: r.method, kind: r.kind })),
    [
      { from: "Post", method: "author", kind: "belongsTo" },
      { from: "Comment", method: "poster", kind: "belongsTo" },
    ]
  );
});

test("a self-referential relation shows in both directions", () => {
  // Adjacency-list pattern: Category belongsTo Category as parent. It is both an
  // outbound relation (this Category points at a parent Category) AND an inbound
  // one (a child Category points at this one), so both sections must show it.
  const pm = projectModel({
    models: [
      { name: "Category", table: "categories", relationships: [{ kind: "belongsTo", method: "parent", target: "Category" }] },
    ],
  });

  const detail = composeModelDetail("Category", pm);

  assert.deepEqual(detail.forwardRelations.map((r) => r.method), ["parent"]);
  assert.deepEqual(
    detail.reverseRelations.map((r) => ({ from: r.from, method: r.method })),
    [{ from: "Category", method: "parent" }]
  );
});

test("the model's table schema (columns + indexes) is resolved via Model.table", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [] }],
    schemas: [
      {
        name: "posts",
        columns: [{ name: "id", type: "bigInteger", is_primary_key: true }],
        indexes: [{ name: "posts_slug_unique", columns: ["slug"], unique: true }],
      },
      { name: "users", columns: [], indexes: [] },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.table.name, "posts");
  assert.deepEqual(detail.table.columns.map((c) => c.name), ["id"]);
  assert.deepEqual(detail.table.indexes.map((i) => i.name), ["posts_slug_unique"]);
});

test("a model whose table is absent from the schema resolves to a null table", () => {
  const pm = projectModel({
    models: [{ name: "Ghost", table: "ghosts", relationships: [] }],
    schemas: [{ name: "users", columns: [], indexes: [] }],
  });

  const detail = composeModelDetail("Ghost", pm);

  // Null, not undefined and not a fabricated empty table: the renderer shows a
  // calm "table not found in schema" note, the same signal models.js gives.
  assert.equal(detail.table, null);
});

test("disagreements about this model are scoped in, carrying warn severity", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [] }],
    disagreements: [
      { model: "Post", relationship: "author", reason: "target table \"users\" not found", kind: "missing_table" },
      { model: "Comment", relationship: "post", reason: "unrelated", kind: "missing_table" },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.findings.length, 1);
  assert.equal(detail.findings[0].kind, "disagreements");
  assert.equal(detail.findings[0].severity, "warn");
  assert.match(detail.findings[0].subject, /author/);
});

test("an unguarded model surfaces a blocker finding scoped to itself", () => {
  const pm = projectModel({
    // guarded: [] is the escape hatch — every column mass-assignable (blocker).
    models: [{ name: "Post", table: "posts", relationships: [], guarded: [] }],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.findings.length, 1);
  assert.equal(detail.findings[0].kind, "unguarded");
  assert.equal(detail.findings[0].severity, "blocker");
});

test("a guarded-by-omission model (nil guarded) is NOT flagged unguarded", () => {
  const pm = projectModel({
    // Neither fillable nor guarded declared: guarded-by-omission, fully
    // protected — the nil-vs-[] distinction is load-bearing (eloquent.go).
    models: [{ name: "Post", table: "posts", relationships: [] }],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.findings.length, 0);
});

test("routes whose controller is {Model}Controller are owned by the model (strong match)", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [] }],
    routes: [
      { method: "GET", uri: "/posts", controller: "PostController", action: "index", middleware: [] },
      { method: "GET", uri: "/users", controller: "UserController", action: "index", middleware: [] },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.routes.length, 1);
  assert.equal(detail.routes[0].uri, "/posts");
  assert.equal(detail.routes[0].match, "controller");
});

test("routes linked to the model via a form request are owned (form-request match)", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [] }],
    form_requests: [{ name: "StorePostRequest", fqn: "App\\Http\\Requests\\StorePostRequest", fields: [] }],
    routes: [
      {
        method: "POST",
        uri: "/submit",
        controller: "FormController",
        action: "store",
        middleware: [],
        form_request: "App\\Http\\Requests\\StorePostRequest",
      },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.routes.length, 1);
  assert.equal(detail.routes[0].uri, "/submit");
  assert.equal(detail.routes[0].match, "form_request");
});

test("form-request match is convention-bounded, not a loose substring", () => {
  // The FormRequest naming convention is {Verb}{Model}Request. Matching on a
  // bare substring would misattribute PowerUserRequest (a PowerUser feature) to
  // User — a false positive ADR 0002 (precision over coverage) forbids. The
  // match must be the model name as a whole convention token, not a substring.
  const pm = projectModel({
    models: [{ name: "User", table: "users", relationships: [] }],
    form_requests: [{ name: "StorePowerUserRequest", fqn: "App\\Http\\Requests\\StorePowerUserRequest", fields: [] }],
    routes: [
      {
        method: "POST",
        uri: "/power-users",
        controller: "PowerUserController",
        action: "store",
        middleware: [],
        form_request: "App\\Http\\Requests\\StorePowerUserRequest",
      },
    ],
  });

  const detail = composeModelDetail("User", pm);

  // StorePowerUserRequest validates PowerUser, not User — no match on User.
  assert.equal(detail.routes.length, 0);
});

test("form-request match accepts the CRUD-verb convention forms for the model", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "elsewhere", relationships: [] }],
    form_requests: [
      { name: "StorePostRequest", fqn: "App\\Http\\Requests\\StorePostRequest", fields: [] },
      { name: "UpdatePostRequest", fqn: "App\\Http\\Requests\\UpdatePostRequest", fields: [] },
      { name: "PostRequest", fqn: "App\\Http\\Requests\\PostRequest", fields: [] },
    ],
    routes: [
      { method: "POST", uri: "/a", controller: "C", action: "store", middleware: [], form_request: "App\\Http\\Requests\\StorePostRequest" },
      { method: "PUT", uri: "/b", controller: "C", action: "update", middleware: [], form_request: "App\\Http\\Requests\\UpdatePostRequest" },
      { method: "POST", uri: "/c", controller: "C", action: "save", middleware: [], form_request: "App\\Http\\Requests\\PostRequest" },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.deepEqual(detail.routes.map((r) => r.uri).sort(), ["/a", "/b", "/c"]);
  assert.ok(detail.routes.every((r) => r.match === "form_request"));
});

test("routes whose URI segment matches the model's table are owned (weak match)", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [] }],
    routes: [
      { method: "GET", uri: "/posts/{post}/comments", controller: "MiscController", action: "show", middleware: [] },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.routes.length, 1);
  assert.equal(detail.routes[0].match, "uri");
});

test("owning routes are ranked strongest-match-first and deduped", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [] }],
    form_requests: [{ name: "StorePostRequest", fqn: "App\\Http\\Requests\\StorePostRequest", fields: [] }],
    routes: [
      // Weak URI-only match, listed first in source.
      { method: "GET", uri: "/posts/feed", controller: "FeedController", action: "index", middleware: [] },
      // Strong controller match, listed second in source.
      { method: "GET", uri: "/admin/dash", controller: "PostController", action: "index", middleware: [] },
      // Matches BOTH controller and URI — must appear once, at controller rank.
      {
        method: "POST",
        uri: "/posts",
        controller: "PostController",
        action: "store",
        middleware: [],
        form_request: "App\\Http\\Requests\\StorePostRequest",
      },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  // Two controller matches (deduped to one each) rank above the URI-only one.
  assert.deepEqual(
    detail.routes.map((r) => ({ uri: r.uri, match: r.match })),
    [
      { uri: "/admin/dash", match: "controller" },
      { uri: "/posts", match: "controller" },
      { uri: "/posts/feed", match: "uri" },
    ]
  );
});

// --- Model graph + mass-assignment overlay (issue #70) ---------------------
//
// The detail page gained two sections that are joins across node types: a
// model-centric GRAPH (Model↔Model relationships, distinct from the
// table-centric ER diagram) and the mass-assignment OVERLAY (what the Model
// says about each of the Schema's columns). Their internals are tested in
// model-graph.test.js and model-overlay.test.js; what is pinned here is that
// composeModelDetail actually composes them onto the view-model, correctly
// wired to THIS model.

test("the composed detail carries a model-centric graph centered on this model", () => {
  const pm = projectModel({
    models: [
      { name: "Post", table: "posts", relationships: [{ kind: "belongsTo", method: "author", target: "User" }] },
      { name: "User", table: "users", relationships: [] },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  assert.equal(detail.graph.center, "Post");
  assert.deepEqual(detail.graph.nodes.map((n) => n.name), ["Post", "User"]);
  assert.equal(detail.graph.edges.length, 1);
  assert.equal(detail.graph.edges[0].label, "belongsTo");
});

test("the graph is the MODEL graph, not the ER table graph", () => {
  // The distinction the ticket exists to protect: nodes are model class names,
  // never the tables those models map to. A graph keyed by "posts"/"users"
  // would be the ER diagram wearing the Model page's label.
  const pm = projectModel({
    models: [
      { name: "Post", table: "posts", relationships: [{ kind: "belongsTo", method: "author", target: "User" }] },
      { name: "User", table: "users", relationships: [] },
    ],
    schemas: [{ name: "posts", columns: [], indexes: [] }, { name: "users", columns: [], indexes: [] }],
  });

  const names = composeModelDetail("Post", pm).graph.nodes.map((n) => n.name);
  assert.deepEqual(names, ["Post", "User"]);
  assert.ok(!names.includes("posts"), "graph nodes are models, not tables");
});

test("the mass-assignment overlay annotates the MAPPED table's columns", () => {
  const pm = projectModel({
    models: [
      {
        name: "User",
        table: "users",
        relationships: [],
        fillable: ["email", "password"],
        hidden: ["password"],
        // Cast{Column, Type} — the field is `column`, not `name`.
        casts: [{ column: "email_verified_at", type: "datetime" }],
      },
    ],
    schemas: [
      {
        name: "users",
        columns: [{ name: "id" }, { name: "email" }, { name: "password" }, { name: "email_verified_at" }],
        indexes: [],
      },
    ],
  });

  const overlay = new Map(composeModelDetail("User", pm).overlay.map((c) => [c.name, c]));

  assert.equal(overlay.get("email").fillable, true);
  assert.equal(overlay.get("id").fillable, false);
  assert.equal(overlay.get("password").hidden, true);
  assert.equal(overlay.get("email_verified_at").cast, "datetime");
});

test("a model whose table is absent from the schema still composes an empty overlay", () => {
  // The section must degrade to "no columns to annotate", never to a crash or a
  // fabricated column list.
  const pm = projectModel({
    models: [{ name: "Ghost", table: "ghosts", relationships: [], fillable: ["x"] }],
    schemas: [],
  });

  const detail = composeModelDetail("Ghost", pm);

  assert.equal(detail.table, null);
  assert.deepEqual(detail.overlay.map((c) => c.name), []);
  assert.deepEqual(detail.overlay.unmatched, [{ name: "x", source: "fillable" }]);
});

test("the composed detail names the model's mass-assignment state", () => {
  // The per-column verdicts only make sense against the state that produced
  // them, so the state travels with the overlay rather than being re-derived.
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [], guarded: [] }],
  });

  assert.equal(composeModelDetail("Post", pm).massAssignment, "unguarded");
});

test("an unknown model composes an empty graph and overlay, not undefined", () => {
  const detail = composeModelDetail("Ghost", projectModel({}));

  assert.equal(detail.found, false);
  assert.deepEqual(detail.graph.nodes, []);
  assert.deepEqual(detail.overlay, []);
});

test("form requests validating the model's writes are composed from its owning routes", () => {
  const pm = projectModel({
    models: [{ name: "Post", table: "posts", relationships: [] }],
    form_requests: [
      { name: "StorePostRequest", fqn: "App\\Http\\Requests\\StorePostRequest", fields: [{ name: "title", rules: [] }] },
      { name: "UpdatePostRequest", fqn: "App\\Http\\Requests\\UpdatePostRequest", fields: [] },
      { name: "OtherRequest", fqn: "App\\Http\\Requests\\OtherRequest", fields: [] },
    ],
    routes: [
      {
        method: "POST",
        uri: "/posts",
        controller: "PostController",
        action: "store",
        middleware: [],
        form_request: "App\\Http\\Requests\\StorePostRequest",
      },
      {
        method: "PUT",
        uri: "/posts/{post}",
        controller: "PostController",
        action: "update",
        middleware: [],
        form_request: "App\\Http\\Requests\\UpdatePostRequest",
      },
      // Owned route with no form request contributes none.
      { method: "GET", uri: "/posts", controller: "PostController", action: "index", middleware: [] },
    ],
  });

  const detail = composeModelDetail("Post", pm);

  // The two writes' form requests, resolved to their full objects (so fields
  // are available on the page), deduped, in route order.
  assert.deepEqual(
    detail.formRequests.map((f) => f.name),
    ["StorePostRequest", "UpdatePostRequest"]
  );
  assert.deepEqual(detail.formRequests[0].fields.map((f) => f.name), ["title"]);
});
