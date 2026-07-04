// The shared entity-chip / cross-link helper (issue #24): one place that turns
// any reference to a model, table, route, or controller into a consistent cyan
// chip carrying the correct in-app navigation target. This is the learned
// visual language for "this is a thing you can jump to," used identically in
// every view.

import { escapeHtml } from "./dom.js";

// entityChip renders a reference as the cyan cross-link chip: a real <a> whose
// href/data-view/data-entity-id carry the in-app target, so a plain left-click
// navigates (via the delegated handler in main.js) while middle/cmd/shift-click
// keep the browser's native open-in-new-tab. A reference whose kind has no
// target (see chipTarget) degrades to inert escaped text rather than a broken
// link. `label` overrides the visible text; it defaults to the reference name.
export function entityChip(ref, label) {
  const text = escapeHtml(label != null ? label : (ref && ref.name) || "");
  const target = chipTarget(ref);
  if (!target) {
    return '<span class="entity-chip is-plain">' + text + "</span>";
  }
  return (
    '<a class="entity-chip" href="#/' + escapeHtml(target.view) + '"' +
    ' data-view="' + escapeHtml(target.view) + '"' +
    ' data-entity-id="' + escapeHtml(target.id) + '">' +
    text +
    "</a>"
  );
}

// chipTarget maps an entity reference to its in-app navigation target: the
// sidebar view that owns that entity kind, plus an id used later to focus the
// specific entity within that view. Kept pure (no DOM) so it is unit-testable
// on its own and can't drift from the chip markup that consumes it.
//
// A recognized kind whose identifying field is missing/empty resolves to null
// (not a target) — the same degradation as an unknown kind. Otherwise a
// name-less reference would produce a chip carrying id="undefined" that a later
// focus-this-entity consumer would silently mis-target.
export function chipTarget(ref) {
  if (!ref) return null;
  const name = nonEmpty(ref.name);
  if (ref.kind === "model" && name) {
    return { view: "models", id: name };
  }
  if (ref.kind === "table" && name) {
    return { view: "er", id: name };
  }
  if (ref.kind === "controller" && name) {
    return { view: "routes", id: name };
  }
  if (ref.kind === "route") {
    const method = nonEmpty(ref.method);
    const uri = nonEmpty(ref.uri);
    if (method || uri) {
      return { view: "routes", id: (ref.method || "") + " " + (ref.uri || "") };
    }
  }
  return null;
}

// nonEmpty returns the value only when it is a non-blank string, so a missing
// or empty identifying field reads as falsy in chipTarget's guards.
function nonEmpty(v) {
  return typeof v === "string" && v.trim() !== "" ? v : null;
}
