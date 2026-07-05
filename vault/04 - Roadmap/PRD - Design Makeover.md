# PRD — Design Makeover & ER Fix

> **Status:** Ready to execute. Grilled and approved 2026-07-04.
> **Foundation:** [PRODUCT.md](../../PRODUCT.md) (intention) + [DESIGN.md](../../DESIGN.md) (system).
> **Companion decision record:** ADR 0010 (to be written in slice 0).
>
> This PRD replaces the "disciplined slop" first-pass UI with a committed design
> system built on the **un-ravel thread metaphor**, and fixes the ER diagram
> crash. An agent can execute it end-to-end; each slice is an independent
> vertical PR per the repo's slice convention.

## Problem

Two distinct failures, deliberately kept separate:

1. **The ER diagram crashes** ("Could not render the ER diagram") on real
   projects. **Verified root cause:** models whose class name is already plural
   or uncountable (`WaiterCalls`, `RestaurantStaff`) infer over-pluralized table
   names (`waiter_callses`, `restaurant_staffs`) that don't exist in the schema.
   The disagreement pass *already flags* this, but `er.eloquentEdges` still emits
   an ELK edge to the phantom node; ELK throws on the first dangling edge and the
   whole diagram dies. One bad edge kills everything.
2. **The UI reads as AI slop.** Two independent design reviews scored it 28/40
   ("good, not distinctive"). Root cause: no stated intention. The un-ravel
   metaphor lives only in code comments and is hidden after first analysis; the
   working shell is a Tailwind-default dark dashboard indistinguishable from any
   admin template. The flagship Overview view is literally **empty** (renders an
   sr-only heading and nothing else).

## Goals

- ER diagram renders on any project; a name-mismatched relationship becomes a
  *visible unresolved-red edge + a finding*, not a crash.
- A real design system (PRODUCT.md + DESIGN.md) exists and every view conforms.
- The six anti-slop bans hold, checkably.
- The Overview becomes a first-contact wow moment aimed at the Laravel community.
- Intermediate users keep calm, fast, legible working views.

## Non-goals

- No change to the engine's static-only, deterministic, non-nil-slice,
  parser-isolation conventions.
- No rebinding of a model's inferred `Table` to the real table (honesty stays in
  the model; correction lives in the disagreement).
- No new fuzzy/edit-distance matching beyond unambiguous singular/plural siblings
  (precision over coverage, ADR 0002).

---

## Slice 0 — Foundation docs (no code)

**Ship:** PRODUCT.md + DESIGN.md (both already drafted at repo root) reviewed and
committed, plus **ADR 0010 "Design system and the thread metaphor"** in
`vault/01 - Architecture/ADRs/`, and a row in the ADR Index. Roadmap updated.

**Done when:** the north star, the six anti-references, and the token/type/motion
specs are the agreed contract every later slice conforms to.

---

## Slice 1 — ER correctness fix (Go, standalone, ships FIRST)

Independent of all visual work. Lands first so the tool *works* while the design
slices proceed.

**Changes:**
1. **Edge reconciliation** in `internal/render/er/graph.go`: when an Eloquent
   edge endpoint (`mdl.Table` / target table) is absent from the schema table
   set, attempt an **unambiguous singular/plural sibling match** against real
   table names (`waiter_callses`↔`waiter_calls`, `restaurant_staffs`↔`restaurant_staff`).
   Match only when **exactly one** candidate exists; ambiguous → drop the edge.
2. **`EREdge.Origin` / unresolved flag** on the model type: matched-via-correction
   edges are marked unresolved so the renderer can style them red (styling ships
   in slice 3). Resolves the [[er-edge-origin-followup]] memory note.
3. **Surface the mismatch as a finding** (the disagreement pass already computes
   it; ensure it reaches Findings with a "did you mean `waiter_calls`? add
   `protected $table`" correction).
4. **Client-side guard** in `internal/web/assets/js/views/er-graph.js`
   `buildElkGraph`: drop any edge whose `from`/`to` is not in the node set. The
   never-dies backstop; with reconciliation it should essentially never fire.
5. **Additive inflector nudge** in `internal/extract/model/tablename.go`: handle
   already-plural stems and uncountables (e.g. `staff`) so `Model.Table`
   inference improves at the source. Additive only — not the load-bearing fix.

**Contract:** this touches `ProjectModel` (the `EREdge` flag) → **bump
`model.CurrentSchemaVersion`** with a doc-comment paragraph, **regenerate goldens
deliberately and review the diff**, update affected `er`/`web` tests. TDD: write
the failing test on the fixture (or a new fixture with a plural-class model)
first.

**Done when:** `./unlaravel serve` renders the ER diagram for
`/Users/mac/Workshop/Work/the-tag-backend` (54 tables, previously crashed);
the 12 previously-orphan edges either land on their real node (unresolved-flagged)
or are cleanly dropped; `go test ./... -race -cover` green; goldens reviewed.

---

## Slice 2 — Token layer migration (CSS foundation)

Rewrite `internal/web/assets/css/tokens.css` to the DESIGN.md system. Everything
downstream inherits this.

**Changes:** OKLCH warmed neutrals (§1); rename `--accent`→`--resolved`, add
`--unresolved`, unban `--brand` for semantics; real ~1.25 modular type scale with
display-step tokens (§2); fix small-text contrast to AA. Update the existing
token/restyle/fonts tests.

**Done when:** the shell renders on the new tokens, no pure gray, contrast checks
pass, existing JS/DOM tests green.

---

## Slice 3 — Working-view reskin (calm register)

The daily-use surfaces the intermediates live in. Apply DESIGN.md §5.

**Changes:** kill **all** side-stripes (findings, model-cards, dead-route rows,
nav active state) → dot+tint severity + tinted/filled nav active; apply the
human/machine face split across Models/Routes/Findings; ER edges styled
resolved-cyan vs. unresolved-red (using slice 1's flag); remove the dimmed-numbers
hack; fix the em dash in the hero value prop copy.

**Done when:** the §8 grep-able conformance checklist passes on every working
view; no anti-reference violation remains.

---

## Slice 4 — Overview wow moment (GATED on prototype)

**Prototype first** via `/prototype`: build a throwaway of the un-ravel
constellation animation (the Laravel mark un-raveling into an abstract node cloud
of the project's entities, red→cyan, stats tallying in sync). **Judge whether it
lands before committing scope.**

- If it lands → build it as the Overview view, filling the empty panel.
- If it doesn't → fall back to the logo resolving red→cyan with a synced stat
  count-up (still special, far cheaper).

Either way: degrade to a static resolved end-state under reduced motion; no jank
on a slow machine; screenshot/GIF-friendly (Laravel community shares tools).

**Done when:** a first-contact user sees the thesis land in one gesture, and the
Overview is no longer empty.

---

## Slice 5 — Motion pass (app-wide signature)

Promote the settle from one panel to the identity gesture (DESIGN.md §6): views
resolve in from a slightly-tangled state on first paint; a persistent sidebar
thread-mark re-settles on each new analysis. Tune timing; verify reduced-motion
degradation everywhere. Scarce, not sprinkled.

**Done when:** the un-ravel gesture is the app's recurring signature without
motion fatigue, and every animation respects `prefers-reduced-motion`.

---

## Sequencing & dependencies

```
Slice 0 (docs) ─┬─► Slice 1 (ER fix, Go) ──────────────► [ships first, standalone]
                └─► Slice 2 (tokens) ─► Slice 3 (views) ─► Slice 4 (wow, gated) ─► Slice 5 (motion)
```

Slice 1 is independent of 2–5 and lands first. Slices 2→5 stack on the token
foundation. Slice 4 is gated behind a prototype verdict.

## Definition of done (whole PRD)

- ER renders on real projects; name-mismatches are visible findings, not crashes.
- Design health re-review scores materially above 28/40; the "AI made this"
  test fails (you would *not* believe it).
- All six anti-references hold under the grep-able checklist.
- Newcomers get the wow; intermediates keep calm working views.
- `go test ./... -race -cover` green; goldens reviewed; contract version bumped
  where touched.
