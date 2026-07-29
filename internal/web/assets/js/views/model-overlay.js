// model-overlay.js is the mass-assignment overlay for the Model page (issue
// #70): the join between the Schema's columns and what the Model says about
// each one. Schema and Model are separate nodes in the Project Model — the
// migration creates a column, the Eloquent class decides whether that column is
// mass-assignable, stripped from serialization, or cast — and this module is
// exactly and only that join.
//
// composeColumnOverlay is pure (model + schema columns in, per-column verdicts
// out), so Laravel's fillable/guarded semantics are unit-testable with no DOM
// and can't drift from the markup that consumes them — the same split as
// composeModelDetail.

import { escapeHtml } from "../dom.js";

// massAssignmentState classifies a model's fillable/guarded pair into one of
// four states. nil vs. [] is load-bearing here (see the doc comment on
// Fillable/Guarded in internal/model/eloquent.go): a bare "|| []" coercion
// would erase the distinction between "declared empty" and "not declared".
//
// Exported (rather than re-derived) so the Models list page and this overlay
// can never disagree about what state a model is in — models.js imports this
// one definition.
//
//   unguarded — `$guarded = []`, Laravel's everything-is-assignable escape hatch
//   fillable  — `$fillable` declared: an ALLOW-list, checked first by Laravel
//   guarded   — a non-empty `$guarded`: a DENY-list
//   protected — neither declared: guarded by omission, nothing assignable
//
// This describes what the model DECLARED — it is the headline the page uses to
// explain the column verdicts. It is NOT how those verdicts are computed:
// columnIsFillable decides per column, because wherever a non-empty declaration
// crosses an empty one these four labels and Laravel's isFillable diverge (see
// the worked cases on columnIsFillable).
export function massAssignmentState(m) {
  if (Array.isArray(m.guarded) && m.guarded.length === 0) {
    return "unguarded";
  }
  if (Array.isArray(m.fillable)) {
    return "fillable";
  }
  if (Array.isArray(m.guarded)) {
    return "guarded";
  }
  return "protected";
}

// columnIsFillable answers Laravel's isFillable question for ONE column. It
// transcribes Illuminate\Database\Eloquent\Concerns\GuardsAttributes rather
// than paraphrasing it, because the interesting cases are precisely the ones a
// paraphrase gets wrong:
//
//   isFillable($key):
//     1. if (in_array($key, getFillable()))  return true;
//     2. if (isGuarded($key))                return false;
//     3. return empty(getFillable()) && ...
//
//   isGuarded($key):
//     if (empty(getGuarded())) return false;          // an empty $guarded
//                                                     // guards NOTHING
//     return getGuarded() == ['*'] || in_array($key, getGuarded()) || ...
//
// The decision is per COLUMN, not per model: the four-state classification
// below describes what a model DECLARED, but it cannot be applied wholesale to
// every column without diverging from Laravel wherever a non-empty declaration
// crosses an empty one. Two cases make that concrete, and both are pinned by
// tests:
//
//   $fillable=['title'], $guarded=[]  — "unguarded" as a model-level label, yet
//       `body` is NOT fillable: step 2 is false (empty $guarded), and step 3
//       fails because $fillable is non-empty.
//   $fillable=[],        $guarded=['id'] — "fillable" as a label, yet `title`
//       IS fillable: step 1 admits nothing, step 2 clears it, and step 3 now
//       succeeds because $fillable IS empty.
//
// `*` is honored as Laravel's guard-everything wildcard. The `.`/`_` prefix
// exclusions of step 3 are deliberately NOT modelled: they concern nested and
// internal attribute keys, not the physical schema columns this overlay
// annotates.
function columnIsFillable(m, columnName) {
  const fillable = Array.isArray(m.fillable) ? m.fillable : [];
  // An UNDECLARED $guarded is not an empty one: Eloquent's property defaults to
  // `['*']`, the guard-everything wildcard. This is exactly why the contract
  // preserves nil-vs-[] on Guarded (internal/model/eloquent.go) — coercing a
  // nil to [] here would turn Laravel's protected default into its escape
  // hatch and report every column of an undeclared model as mass-assignable.
  const guarded = Array.isArray(m.guarded) ? m.guarded : ["*"];

  // 1. An explicit $fillable entry is mass-assignable, whatever $guarded says.
  if (fillable.includes(columnName)) return true;

  // 2. isGuarded: an EMPTY $guarded guards nothing, so this step cannot reject.
  if (guarded.length > 0 && (guarded.includes("*") || guarded.includes(columnName))) {
    return false;
  }

  // 3. Otherwise mass-assignable only when no $fillable allow-list narrows it.
  return fillable.length === 0;
}

// composeColumnOverlay annotates each of the mapped table's columns with what
// the model says about it: whether it is mass-assignable (and under which
// declared state), whether `$hidden` strips it from serialization, and the cast
// type it carries, if any.
//
// The returned array mirrors the SCHEMA's columns exactly — same entries, same
// order (determinism is a hard convention). A name the model declares that no
// column matches is NOT invented as a row; it is reported separately on the
// result's `unmatched` property, in declaration order, because that mismatch is
// itself a real finding (a renamed column, a typo) the page should show rather
// than silently drop.
export function composeColumnOverlay(model, columns) {
  const m = model || {};
  const cols = columns || [];
  const state = massAssignmentState(m);

  const hidden = new Set(Array.isArray(m.hidden) ? m.hidden : []);
  // A Cast's attribute name is `column`, not `name` (internal/model/eloquent.go
  // — Cast{Column, Type}). Reading `.name` here keys the map on undefined and
  // annotates nothing, while a test fixture that invents `name` still passes.
  const castByColumn = new Map((m.casts || []).map((c) => [c.column, c.type]));
  const known = new Set(cols.map((c) => c.name));

  const overlay = cols.map((c) => ({
    name: c.name,
    type: c.type || "",
    state,
    fillable: columnIsFillable(m, c.name),
    hidden: hidden.has(c.name),
    cast: castByColumn.get(c.name) || "",
  }));

  // Declared-but-absent names, in the order the source declared them, each
  // tagged with the property that named it so the reader knows where to look.
  // Built from the arrays (not the Sets) to keep declaration order.
  const unmatched = [];
  const collect = (names, source) => {
    (Array.isArray(names) ? names : []).forEach((name) => {
      if (!known.has(name)) unmatched.push({ name, source });
    });
  };
  collect(m.fillable, "fillable");
  collect(m.guarded, "guarded");
  collect(m.hidden, "hidden");
  collect((m.casts || []).map((c) => c.column), "casts");

  // Attached as a property rather than returned in a wrapper object: the
  // overlay IS the column list at every call site, and `unmatched` is the rare
  // side-channel. Non-enumerable would hide it from deepEqual in tests, so it
  // stays a plain property.
  overlay.unmatched = unmatched;
  return overlay;
}

// STATE_NOTE explains, in the model's own vocabulary, WHY the column verdicts
// below read the way they do — so a reader who has never met `$guarded` can
// still tell an explicit decision from an omission.
// Each note describes the DECLARATION, and stops short of claiming a verdict
// for every column — the per-column results below are authoritative, and the
// two only coincide when one of $fillable/$guarded is absent. In particular the
// unguarded note says "guards nothing" rather than "every column is
// mass-assignable", which would be false for a model that also declares a
// non-empty $fillable.
const STATE_NOTE = {
  unguarded: "$guarded = [] — the guard list is empty, so it guards nothing",
  fillable: "$fillable declared — an allow-list, checked before $guarded",
  guarded: "$guarded declared — a deny-list; columns it does not name are mass-assignable",
  protected: "no $fillable or $guarded — mass assignment is fully guarded",
};

// overlayHtml renders the overlay as a column table: one row per schema column,
// each carrying its mass-assignment verdict, a hidden marker, and its cast.
// Pure (overlay in, markup out) so the markup contract is unit-testable.
export function overlayHtml(overlay, state) {
  const cols = overlay || [];
  const note = '<div class="empty-note">' + escapeHtml(STATE_NOTE[state] || "") + "</div>";

  if (cols.length === 0) {
    return (
      '<section class="detail-section"><div class="subhead">Mass assignment</div>' +
      note +
      '<div class="empty-note">This model\'s table was not found in the schema, so its columns cannot be annotated.</div>' +
      unmatchedHtml(cols.unmatched) +
      "</section>"
    );
  }

  const rows = cols
    .map((c) => {
      // The mass-assignment verdict leads the row: it is the one fact that can
      // be a security decision. "unguarded" is called out by name because a
      // fillable column under `$guarded = []` was never actually chosen.
      //
      // NON-COLOR CHANNEL (DESIGN.md §1 hard rule, ADR 0011 decision 4): this
      // is a red-vs-green distinction, so the channel is named here — each
      // pill's own TEXT LABEL ("unguarded" / "fillable" / "guarded"). The
      // verdict is fully readable with the hue removed; the color only speeds
      // up scanning a long column list.
      const verdict =
        c.fillable && c.state === "unguarded"
          ? '<span class="pill danger">unguarded</span>'
          : c.fillable
            ? '<span class="pill ok">fillable</span>'
            : '<span class="pill dim">guarded</span>';
      const hidden = c.hidden ? '<span class="pill warn">hidden</span>' : "";
      const cast = c.cast ? '<span class="pill">cast: ' + escapeHtml(c.cast) + "</span>" : "";
      return (
        '<div class="col-row overlay-row">' +
        '<span class="col-name">' + escapeHtml(c.name) + "</span> " +
        '<span class="col-type">' + escapeHtml(c.type) + "</span> " +
        verdict + hidden + cast +
        "</div>"
      );
    })
    .join("");

  return (
    '<section class="detail-section"><div class="subhead">Mass assignment</div>' +
    note +
    '<div class="col-list">' + rows + "</div>" +
    unmatchedHtml(cols.unmatched) +
    "</section>"
  );
}

// unmatchedHtml surfaces the declared names that match no schema column — real
// drift between the class and the migration, shown plainly rather than dropped.
function unmatchedHtml(unmatched) {
  const list = unmatched || [];
  if (list.length === 0) return "";
  // The name reads as a chip (it is a column identifier, the mono register) and
  // the property that declared it as a warn pill — .pill.warn and .chip both
  // have rules; a `.chip.warn` or `.dim` variant would have none, so the drift
  // signal would silently render as plain text.
  const chips = list
    .map(
      (u) =>
        '<span class="chip">' + escapeHtml(u.name) + "</span>" +
        '<span class="pill warn">$' + escapeHtml(u.source) + "</span>"
    )
    .join("");
  return (
    '<div class="subhead">Declared, but not in the schema</div>' +
    '<div class="chips">' + chips + "</div>"
  );
}
