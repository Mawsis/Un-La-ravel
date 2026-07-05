---
tags: [adr]
status: accepted
date: 2026-07-04
aliases: ["ADR 0009 - ER settle animation and diagram export"]
---

# ADR 0009 — The ER diagram gets a one-shot settle animation and SVG/PNG export, reversing issue #25's blanket no-settle rule

Issue #25 established a **functional-only motion policy**: animation is allowed
only for state the user caused (hover feedback, view changes, a loading spinner,
the ER-focus highlight), and **decorative settle/stagger/count-up motion is
banned**. That rule is not just documentation — `internal/web/motion_test.go`
enforces it against the shipped CSS with a keyframes allowlist and a
no-raw-duration check.

Issue #30 (a child of the [[Roadmap|#19 dashboard redesign]]) asks for exactly
what #25 ruled out: a **settle animation** where the ER entity boxes ease from a
rough arrangement into their laid-out positions on first render. Per the
[[CLAUDE|project conventions]], reversing a load-bearing convention "needs an
ADR, not a one-line justification." This is that ADR.

It also records the second half of #30 — **SVG and PNG export** of the diagram —
because both features became cheap for the same reason: the ER renderer is now
**owned** ([[0004-serialized-project-model-as-output-contract|the SVG is our own
markup]], issue #26), not a vendored Mermaid embed.

## Context

Two facts changed the calculus since #25:

- **The renderer is owned.** Issue #26 replaced Mermaid with a hand-rolled SVG
  emitter (`er-svg.js`) laid out by ELK. Every entity `<g>` carries its resolved
  `translate(x, y)` and a `data-table` hook. Animating our own nodes from a
  rough start to that resolved position is now a few lines; exporting the SVG is
  a serialization we fully control.
- **The settle is the product metaphor, not idle decoration.** The tool is
  *un(la)ravel* — it resolves a tangled Laravel project into a clear structure.
  A settle that plays **once, at the moment the structure resolves**, is that
  metaphor made literal. This is the distinction #25 drew (functional vs.
  decorative) landing on the functional side for this one animation — not a
  blanket re-opening of stagger/count-up decoration.

The #25 policy remains correct for everything else. What #30 needs is a **narrow,
recorded exception**, not a repeal.

## Decision

**Settle animation.**

- On the **first** diagram to mount per page session, each entity box eases from
  a rough start — displaced outward from the diagram center — into its
  ELK-laid-out position. It plays **once** (`er.js` gates on a module-level
  `settlePlayed`); re-renders, focus deep-links, and view switches do not replay
  it.
- It is a **FLIP-style CSS transition on inline transforms**, deliberately *not*
  a `@keyframes`. The node's resolved position lives in its
  `transform="translate(x, y)"`; a keyframe animating the `transform` property
  would override and clobber that placement. `er.js` sets each node's inline
  transform to its offset start, arms `.er-settling` (which carries the
  transition), then flips the inline transform to the resolved position on the
  next animation frame. Because there is no new keyframe, the `motion_test.go`
  allowlist is **unchanged** — the exception costs nothing there.
- The settle's duration lives in the token layer as **`--motion-settle`**
  (single source of truth), consumed by `components.css` via `var()`. No raw
  duration literal is introduced, so the no-raw-duration check also passes
  untouched.
- **Reduced motion is honored twice:** `er.js` reads
  `prefers-reduced-motion: reduce` and skips the whole dance (no pointless
  reflow), and `base.css`'s existing reduced-motion kill switch collapses the
  transition to an instant snap as a backstop.
- The pure geometry — where a node starts (`settleOffset`) and the diagram
  center it converges on (`diagramCenter`) — lives in `er-settle.js` and is
  unit-tested without a browser, matching the `er-svg.js` "pure transform, DOM
  wiring stays thin" split.

**Export.**

- `er-export.js` serializes the mounted diagram to a **self-contained** SVG
  string: the token-driven styling is resolved to concrete values via
  `getComputedStyle` and inlined as a `<style>` block, so the artifact renders
  correctly **outside** the app where no stylesheet or `var(--…)` exists
  (acceptance: "exports reflect the current diagram styling"). PNG is that SVG
  rasterized onto a canvas at 2× for crispness.
- The pure pieces (self-contained markup, raster dimensions) are unit-tested;
  the DOM/canvas download path is browser-only and is wired-and-verified against
  the fixture app rather than unit-tested (Node's built-in test runner has no
  canvas).

## Consequences

- **The #25 policy stands, with one recorded carve-out.** A future
  settle/stagger still has to argue itself past `motion_test.go` and this ADR —
  the bar is intact, this is the single documented exception.
- **No test infrastructure was widened.** Because the settle is a
  transition-not-keyframe timed by a token, the two motion checks that would
  otherwise have needed loosening (keyframe allowlist, raw-duration ban) did
  not change. The reversal is recorded here, not smuggled into a weaker test.
- **Both features stayed pure-core-plus-thin-wiring.** The layout/geometry and
  the serialization are unit-tested; only the irreducibly browser-bound canvas
  and download paths are verified manually — the smallest possible untested
  surface.

## Considered and rejected

- **Honor #25 literally and drop the settle from #30.** Rejected: #30 is an
  approved child of the redesign and the settle is the product's core metaphor
  at its most legible moment. The right move is a recorded exception, not
  ignoring the ticket.
- **Implement the settle as a `@keyframes` and add it to the allowlist.**
  Rejected on correctness, not policy: a keyframe animating `transform` clobbers
  each node's `translate(x, y)` placement, landing every box at the origin. The
  FLIP transition eases the inline transform we control and sidesteps the clash
  entirely — and as a bonus needs no allowlist change.
- **Export by screenshotting the live DOM (e.g. a canvas capture of the
  container).** Rejected: it bakes in the current pan/zoom and the browser's
  rasterization of our SVG, not a clean vector. Serializing our own owned markup
  gives a crisp, editable SVG and a faithful PNG derived from it.
