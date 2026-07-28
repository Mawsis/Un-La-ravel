// Unit tests for the mass-assignment overlay on the Model page (issue #70).
//
// The overlay annotates the MAPPED TABLE's columns with what the MODEL says
// about each one: is it mass-assignable, is it stripped from serialization,
// does it carry a cast. Schema and Model are separate nodes in the Project
// Model and the overlay is exactly their join — so the load-bearing question
// each test pins is "does the column-level verdict follow Laravel's actual
// fillable/guarded semantics", including the nil-vs-empty distinction that
// internal/model/eloquent.go documents as deliberate.
//
// composeColumnOverlay is pure (model + columns in, per-column verdicts out) so
// the semantics are testable with no DOM and no ELK, the same split as
// composeModelDetail.

import { test } from "node:test";
import assert from "node:assert/strict";

import { composeColumnOverlay } from "../assets/js/views/model-overlay.js";

// columns builds the schema-side input: the overlay only reads column names.
function columns(...names) {
  return names.map((name) => ({ name, type: "string" }));
}

// byName indexes the overlay result so a test can assert about one column
// without depending on array position.
function byName(overlay) {
  return new Map(overlay.map((c) => [c.name, c]));
}

test("neither $fillable nor $guarded declared: every column is guarded by omission", () => {
  // Laravel's default. nil/nil means mass assignment is fully protected — so no
  // column may be reported fillable, and the reason must name the omission
  // rather than pretending an explicit rule exists.
  const overlay = composeColumnOverlay({ name: "Post" }, columns("id", "title"));

  assert.equal(overlay.length, 2);
  assert.ok(overlay.every((c) => c.fillable === false));
  assert.ok(overlay.every((c) => c.state === "protected"));
});

test("$guarded = [] makes every column mass-assignable — the escape hatch", () => {
  // The non-nil empty Guarded is Laravel's documented "everything is
  // mass-assignable" escape hatch, materially different from nil (eloquent.go).
  const overlay = composeColumnOverlay({ name: "Post", guarded: [] }, columns("id", "title"));

  assert.ok(overlay.every((c) => c.fillable === true));
  assert.ok(overlay.every((c) => c.state === "unguarded"));
});

test("$fillable lists exactly the mass-assignable columns; the rest are not", () => {
  const overlay = byName(
    composeColumnOverlay({ name: "Post", fillable: ["title", "body"] }, columns("id", "title", "body"))
  );

  assert.equal(overlay.get("title").fillable, true);
  assert.equal(overlay.get("body").fillable, true);
  // id is absent from $fillable — an allow-list excludes everything unlisted.
  assert.equal(overlay.get("id").fillable, false);
  assert.equal(overlay.get("id").state, "fillable");
});

test("$fillable = [] with no $guarded still means nothing is mass-assignable", () => {
  // Not because an empty $fillable allows nothing — step 3 would allow
  // everything — but because an UNDECLARED $guarded defaults to ['*'], which
  // rejects at step 2. Verified against real Laravel (case A in the table
  // below).
  const overlay = composeColumnOverlay({ name: "Post", fillable: [] }, columns("id", "title"));

  assert.ok(overlay.every((c) => c.fillable === false));
  assert.ok(overlay.every((c) => c.state === "fillable"));
});

// The mass-assignment truth table, transcribed from REAL Laravel rather than
// from a reading of the source. Each row was produced by calling
// Illuminate\Database\Eloquent\Model::isFillable on a model declaring exactly
// these properties (Laravel 11, isGuardableColumn stubbed true — it consults
// the live DB schema, a runtime concern this static tool has no equivalent of).
//
// These six rows are the whole contract; the individually-named tests above and
// below are the cases worth explaining in prose, and this table is the guard
// against a "simplification" that quietly breaks one of the others.
const LARAVEL_TRUTH_TABLE = [
  // declaration                                   id     title  body
  [{}, /*                              undeclared */ false, false, false],
  [{ fillable: [] }, /*                          A */ false, false, false],
  [{ fillable: ["title"], guarded: [] }, /*      B */ false, true, false],
  [{ fillable: [], guarded: ["id"] }, /*         C */ false, true, true],
  [{ guarded: [] }, /*                           E */ true, true, true],
  [{ fillable: ["title"], guarded: ["id"] }, /*  F */ false, true, false],
];

test("per-column fillability matches real Laravel across the declaration matrix", () => {
  LARAVEL_TRUTH_TABLE.forEach(([declared, id, title, body]) => {
    const overlay = byName(
      composeColumnOverlay({ name: "Post", ...declared }, columns("id", "title", "body"))
    );
    const label = JSON.stringify(declared);
    assert.equal(overlay.get("id").fillable, id, `id, for ${label}`);
    assert.equal(overlay.get("title").fillable, title, `title, for ${label}`);
    assert.equal(overlay.get("body").fillable, body, `body, for ${label}`);
  });
});

test("$guarded = ['*'] guards every column, the explicit form of the default", () => {
  const overlay = composeColumnOverlay({ name: "Post", guarded: ["*"] }, columns("id", "title"));
  assert.ok(overlay.every((c) => c.fillable === false));
});

test("a non-empty $guarded is a deny-list: everything NOT listed is fillable", () => {
  // The inverse of $fillable, and the case a naive implementation gets backwards.
  const overlay = byName(
    composeColumnOverlay({ name: "Post", guarded: ["id"] }, columns("id", "title", "body"))
  );

  assert.equal(overlay.get("id").fillable, false);
  assert.equal(overlay.get("title").fillable, true);
  assert.equal(overlay.get("body").fillable, true);
  assert.equal(overlay.get("title").state, "guarded");
});

test("$fillable wins over $guarded when a model declares both", () => {
  // Laravel's isFillable checks $fillable first: a column absent from a
  // non-empty $fillable is not mass-assignable regardless of $guarded. Reading
  // $guarded here would wrongly report `body` fillable.
  const overlay = byName(
    composeColumnOverlay(
      { name: "Post", fillable: ["title"], guarded: ["id"] },
      columns("id", "title", "body")
    )
  );

  assert.equal(overlay.get("title").fillable, true);
  assert.equal(overlay.get("body").fillable, false);
  assert.equal(overlay.get("id").fillable, false);
});

// The two cases below cross a NON-EMPTY declaration with an EMPTY one. They are
// where a four-way "classify the model, then apply the class to every column"
// shortcut diverges from Laravel, which decides per COLUMN. Both are pinned
// against Illuminate\Database\Eloquent\Concerns\GuardsAttributes::isFillable:
//
//   if (in_array($key, $this->getFillable())) return true;   // (1)
//   if ($this->isGuarded($key)) return false;                // (2)
//   return empty($this->getFillable()) && ...                // (3)
//
// ...and isGuarded returns FALSE outright when $guarded is empty.

test("$guarded = [] does NOT override a declared non-empty $fillable", () => {
  // `$guarded = []` is often described as "everything is mass-assignable", but
  // that is only true when $fillable is empty. Here step (1) admits `title`;
  // for `body`, step (2) is false (an empty $guarded guards nothing) and step
  // (3) fails because $fillable is NOT empty — so `body` is not fillable.
  // Classifying the whole model as "unguarded" and marking every column
  // assignable would overstate the risk on `body` and `id`.
  const overlay = byName(
    composeColumnOverlay(
      { name: "Post", fillable: ["title"], guarded: [] },
      columns("id", "title", "body")
    )
  );

  assert.equal(overlay.get("title").fillable, true);
  assert.equal(overlay.get("body").fillable, false);
  assert.equal(overlay.get("id").fillable, false);
});

test("an empty $fillable falls through to the $guarded deny-list", () => {
  // Step (1) admits nothing (empty $fillable). Step (2) rejects `id` because
  // $guarded names it. For `title`, step (2) is false and step (3) now SUCCEEDS
  // because $fillable IS empty — so `title` is fillable. Treating a declared
  // `$fillable = []` as "nothing is assignable" regardless of $guarded would
  // wrongly report `title` protected.
  const overlay = byName(
    composeColumnOverlay(
      { name: "Post", fillable: [], guarded: ["id"] },
      columns("id", "title")
    )
  );

  assert.equal(overlay.get("id").fillable, false);
  assert.equal(overlay.get("title").fillable, true);
});

test("$hidden marks the columns stripped from serialization, independent of fillability", () => {
  // Hidden is a SERIALIZATION fact, not a mass-assignment one — a column can be
  // both fillable and hidden (a password is written but never returned).
  const overlay = byName(
    composeColumnOverlay(
      { name: "User", fillable: ["email", "password"], hidden: ["password"] },
      columns("id", "email", "password")
    )
  );

  assert.equal(overlay.get("password").hidden, true);
  assert.equal(overlay.get("password").fillable, true, "hidden must not imply not-fillable");
  assert.equal(overlay.get("email").hidden, false);
  assert.equal(overlay.get("id").hidden, false);
});

test("an undeclared $hidden hides nothing", () => {
  const overlay = composeColumnOverlay({ name: "Post" }, columns("id", "secret"));
  assert.ok(overlay.every((c) => c.hidden === false));
});

test("a column's cast type is carried through by name", () => {
  // A Cast serializes as {"column": ..., "type": ...} — `column`, NOT `name`
  // (internal/model/eloquent.go). Keying on the wrong field yields a cast map
  // of undefined and silently annotates nothing, which a fixture that invented
  // `name` would have hidden.
  const overlay = byName(
    composeColumnOverlay(
      { name: "Post", casts: [{ column: "published_at", type: "datetime" }, { column: "is_live", type: "boolean" }] },
      columns("id", "published_at", "is_live")
    )
  );

  assert.equal(overlay.get("published_at").cast, "datetime");
  assert.equal(overlay.get("is_live").cast, "boolean");
  // An uncast column carries "" — not undefined and not a fabricated type, so
  // the renderer can test one falsy value.
  assert.equal(overlay.get("id").cast, "");
});

test("declared names that match no schema column are reported as unmatched, never invented", () => {
  // $fillable naming a column the migration does not create is a real, common
  // drift (a renamed column, a typo). The overlay must not silently drop it:
  // the page shows it as declared-but-absent rather than fabricating a row.
  const result = composeColumnOverlay(
    { name: "Post", fillable: ["title", "subtitle"], hidden: ["secret"] },
    columns("id", "title")
  );

  // The column rows still mirror the schema exactly — no invented rows.
  assert.deepEqual(result.map((c) => c.name), ["id", "title"]);
  // And the drift is surfaced on the result, in declaration order.
  assert.deepEqual(result.unmatched, [
    { name: "subtitle", source: "fillable" },
    { name: "secret", source: "hidden" },
  ]);
});

test("a cast naming a column the schema lacks is reported as unmatched too", () => {
  // The casts pass reads Cast.column like the annotation pass does; keying it
  // on the wrong field would report every cast as unmatched drift.
  const result = composeColumnOverlay(
    { name: "Post", casts: [{ column: "published_at", type: "datetime" }, { column: "gone", type: "boolean" }] },
    columns("id", "published_at")
  );

  assert.deepEqual(result.unmatched, [{ name: "gone", source: "casts" }]);
});

test("the overlay preserves schema column order (determinism)", () => {
  const overlay = composeColumnOverlay(
    { name: "Post", fillable: ["body", "title"] },
    columns("id", "title", "body", "created_at")
  );
  assert.deepEqual(overlay.map((c) => c.name), ["id", "title", "body", "created_at"]);
});

test("a model with no mapped table yields an empty overlay, not a crash", () => {
  const overlay = composeColumnOverlay({ name: "Ghost", fillable: ["x"] }, []);
  assert.deepEqual(
    overlay.map((c) => c.name),
    []
  );
  // Every declared name is unmatched when there are no columns at all.
  assert.deepEqual(overlay.unmatched, [{ name: "x", source: "fillable" }]);
});
