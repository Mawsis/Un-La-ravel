// The shared entity-chip / cross-link helper (issue #24): one place that turns
// any reference to a model, table, route, or controller into a consistent resolved-green
// chip carrying the correct in-app navigation target. This is the learned
// visual language for "this is a thing you can jump to," used identically in
// every view.

import { escapeHtml } from "./dom.js";

// entityChip renders a reference as the resolved-green cross-link chip: a real <a> whose
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
  // The native href must land on the SAME place a plain left-click navigates to
  // (main.js's delegated handler), so middle/cmd/shift-click open the right page
  // in a new tab. A target with a `detail` addresses one entity via a sub-route
  // (#/models/{name}, issue #51); without one, the href is the bare view hash.
  const href = target.detail
    ? "#/" + escapeHtml(target.view) + "/" + escapeHtml(encodeURIComponent(target.detail))
    : "#/" + escapeHtml(target.view);
  return (
    '<a class="entity-chip" href="' + href + '"' +
    ' data-view="' + escapeHtml(target.view) + '"' +
    ' data-entity-id="' + escapeHtml(target.id) + '">' +
    text +
    "</a>"
  );
}

// dangerFlag renders an inline danger marker (issue #28: DEAD on a dead-route
// row, Unguarded on a model card) as a cross-link chip to its finding category
// in the Findings view. It is the entity-chip visual language with the
// compound danger-flag class, so the delegated handler in main.js navigates it
// like any chip while CSS paints it with the danger token (never the resolved-green
// accent — a problem must not read as ordinary interactive chrome).
// findingKind is the machine-readable Finding.Kind from the contract
// (internal/model/findings.go); label is the visible marker text.
export function dangerFlag(findingKind, label) {
  const target = chipTarget({ kind: "finding", name: findingKind });
  const text = escapeHtml(label);
  if (!target) {
    return '<span class="entity-chip is-plain danger-flag">' + text + "</span>";
  }
  return (
    '<a class="entity-chip danger-flag" href="#/' + escapeHtml(target.view) + '"' +
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
    // A model resolves to its own detail page (#/models/{name}, issue #51): the
    // detail carries the name into the sub-route so the native href, not just
    // the JS-intercepted click, lands on the right page.
    return { view: "models", id: name, detail: name };
  }
  if (ref.kind === "table" && name) {
    return { view: "er", id: name };
  }
  if (ref.kind === "controller" && name) {
    return { view: "routes", id: name };
  }
  if (ref.kind === "finding" && name) {
    // name carries the machine-readable Finding.Kind from the contract
    // (internal/model/findings.go: dead_routes / disagreements / unguarded),
    // so an inline danger flag can later focus its category in the view.
    return { view: "findings", id: name };
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
