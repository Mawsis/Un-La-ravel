// Unit tests for the Models view's mass-assignment section (issue #28: the
// unguarded state is an inline danger flag cross-linking to its finding).
// massAssignmentHtml is pure (model in, markup string out), exported for the
// same no-DOM testability reason as routeRowHtml in routes.js.

import { test } from "node:test";
import assert from "node:assert/strict";

import { massAssignmentHtml } from "../assets/js/views/models.js";

test("an unguarded model is flagged and cross-links to the unguarded finding", () => {
  // $guarded = [] — Laravel's "everything is mass-assignable" escape hatch.
  const html = massAssignmentHtml({ name: "Post", guarded: [] });
  // Severity leads with the status dot (issue #38) — the same dot+tint
  // language as finding rows, never a side-stripe. Decorative, so aria-hidden.
  assert.match(html, /class="status-dot danger" aria-hidden="true"/);
  assert.match(html, /class="entity-chip danger-flag"/);
  assert.match(html, /data-view="findings"/);
  assert.match(html, /data-entity-id="unguarded"/);
  assert.match(html, />Unguarded<\/a>/);
});

test("a fillable model carries no danger flag", () => {
  const html = massAssignmentHtml({ name: "User", fillable: ["name", "email"] });
  assert.doesNotMatch(html, /danger-flag/);
  assert.match(html, /Fillable \(2\)/);
});

test("a guarded or undeclared model carries no danger flag", () => {
  assert.doesNotMatch(massAssignmentHtml({ guarded: ["id"] }), /danger-flag/);
  assert.doesNotMatch(massAssignmentHtml({}), /danger-flag/);
});
