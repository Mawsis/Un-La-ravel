// Unit tests for the model detail page's markup (issue #51).
//
// modelDetailHtml is the pure per-page template: the composed detail view-model
// (from composeModelDetail) in, the page's HTML string out. Pure (no DOM) so the
// markup contract — every reference an entity-chip, a defined not-found state,
// calm per-section empty notes — is unit-testable without a browser, the same
// split as findingRowHtml.

import { test } from "node:test";
import assert from "node:assert/strict";

import { modelDetailHtml } from "../assets/js/views/model-detail.js";
import { hrefFor } from "../assets/js/links.js";

// A composed-detail fixture builder: the shape composeModelDetail returns.
function detail(overrides) {
  return {
    name: "Post",
    found: true,
    forwardRelations: [],
    reverseRelations: [],
    table: null,
    findings: [],
    routes: [],
    formRequests: [],
    ...overrides,
  };
}

test("an unknown model renders a defined not-found state, not a blank page", () => {
  const html = modelDetailHtml(detail({ name: "Ghost", found: false }));
  assert.match(html, /not[- ]found|no model|unknown/i);
  assert.match(html, /Ghost/);
});

test("the page header names the model", () => {
  const html = modelDetailHtml(detail({ name: "Post" }));
  assert.match(html, /<h[12][^>]*>[^<]*Post/);
});

test("a forward relation renders its target as a model entity-chip", () => {
  const html = modelDetailHtml(
    detail({ forwardRelations: [{ kind: "belongsTo", method: "author", target: "User" }] })
  );
  // The target is a chip pointing at the target model's own detail page.
  assert.match(html, /entity-chip/);
  assert.match(html, /data-view="models"/);
  assert.match(html, /data-entity-id="User"/);
  assert.match(html, /author/); // the method name is shown as the relation label
});

test("a reverse relation renders its source model as an entity-chip", () => {
  const html = modelDetailHtml(
    detail({ reverseRelations: [{ from: "Comment", method: "post", kind: "belongsTo", target: "Post" }] })
  );
  assert.match(html, /data-entity-id="Comment"/);
});

test("an owning route renders with its method, URI, and a controller chip", () => {
  const html = modelDetailHtml(
    detail({
      routes: [
        { method: "GET", uri: "/posts", controller: "PostController", action: "index", match: "controller" },
      ],
    })
  );
  assert.match(html, /GET/);
  assert.match(html, /\/posts/);
  // The controller is an entity-chip to the Routes view.
  assert.match(html, /data-entity-id="PostController"/);
});

test("the table schema section renders columns and the table name as a chip to ER", () => {
  const html = modelDetailHtml(
    detail({
      table: { name: "posts", columns: [{ name: "id", type: "bigInteger" }], indexes: [] },
    })
  );
  assert.match(html, /data-view="er"/);
  assert.match(html, /data-entity-id="posts"/);
  assert.match(html, /\bid\b/);
});

test("a scoped finding renders with its severity as a status dot", () => {
  const html = modelDetailHtml(
    detail({
      findings: [{ kind: "unguarded", severity: "blocker", title: "Unguarded:", subject: "Post", reason: "x" }],
    })
  );
  assert.match(html, /status-dot/);
  assert.match(html, /danger/); // blocker → the danger dot, per findings.js DOT_CLASS
});

test("empty sections render a calm per-section note, never vanish", () => {
  const html = modelDetailHtml(detail({}));
  // Relations, routes, form requests, findings, table — each says "nothing here"
  // plainly rather than being omitted (owner's choice: calm empty notes).
  const emptyNotes = html.match(/empty-note/g) || [];
  assert.ok(emptyNotes.length >= 4, "expected calm empty notes for the empty sections, got " + emptyNotes.length);
});

test("hrefFor('model', ...) points at the model's detail page, not the flat list", () => {
  // A model chip should open #/models/{name} (the detail page), carrying the
  // current path forward so the deep link stays analyzable on reload.
  const href = hrefFor("model", "Post", new URLSearchParams({ path: "/tmp/app" }));
  assert.match(href, /^#\/models\/Post/);
  assert.match(href, /path=%2Ftmp%2Fapp/);
});

test("all names are HTML-escaped (names come from parsed source)", () => {
  const html = modelDetailHtml(
    detail({ name: "Post", forwardRelations: [{ kind: "hasMany", method: "x", target: "<img src=x onerror=alert(1)>" }] })
  );
  assert.doesNotMatch(html, /<img/);
});
