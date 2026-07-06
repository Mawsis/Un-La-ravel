# DESIGN.md — Un(la)ravel design system

> Derived from [PRODUCT.md](./PRODUCT.md). This file is the spec the token layer
> (`internal/web/assets/css/tokens.css`) and every component must conform to.
> Where a value here differs from current CSS, this file wins and the CSS is
> migrated. Colors are authored in **OKLCH**; the hex equivalents below are
> reference/fallback only.

## 1. Color

### Strategy

**Restrained-plus-semantic.** Warmed-neutral surfaces carry the interface; two
named color roles (`resolved`, `unresolved`) carry the thread metaphor as
*meaning*, not accent. The loud red↔green drama is concentrated in the Overview
wow moment and in genuine resolution states (ER edges, disagreements). The
working shell stays quiet. The retired resolved-cyan survives only as
`--logo-accent`, the logo's endpoint dot; it never appears in the working UI.

### Neutrals — OKLCH, faintly red-warmed (kills anti-ref #1)

Author every neutral in OKLCH with a small chroma tilted to the brand red hue
(~25°). Chroma stays `0.006–0.012` — present enough to feel like *this* tool,
low enough that a routes table read for an hour never feels tinted.

| Token | OKLCH (author) | ~hex (ref) | Role |
|---|---|---|---|
| `--surface` | `oklch(0.145 0.008 25)` | ~#0c0a0a | app background |
| `--surface-elevated` | `oklch(0.205 0.009 25)` | ~#181615 | cards, sidebar, panels |
| `--surface-raised` | `oklch(0.265 0.010 25)` | ~#272321 | inputs, chips, badges |
| `--border` | `oklch(0.35 0.010 25)` | ~#413b39 | hairlines |
| `--border-strong` | `oklch(0.44 0.010 25)` | ~#544d4a | emphasized edges |
| `--text` | `oklch(0.97 0.004 25)` | ~#faf8f7 | primary text |
| `--text-dim` | `oklch(0.72 0.008 25)` | ~#a8a09d | secondary text |

Never `#000` / `#fff`. The one documented exception is the vendored Swagger
light frame (`color-scheme: light`), which stays as-is with its use-site comment.

### Semantic thread roles (the metaphor, as color)

| Token | OKLCH | Meaning | Where |
|---|---|---|---|
| `--resolved` | `oklch(0.75 0.17 152)` (green) | the tool resolved this | interactive chrome, resolved ER edges, entity-chips, links, focus |
| `--unresolved` | `oklch(0.62 0.20 27)` (brand-adjacent red) | still tangled / mismatched | unresolved ER edges, disagreements, name-mismatch flags |
| `--brand` | `oklch(0.58 0.22 27)` (Laravel #F53003) | identity | wordmark, logo, the un-ravel moment |
| `--logo-accent` | `oklch(0.78 0.13 210)` (cyan) | identity accent only | the logo SVG's endpoint dot, nowhere else |

`--resolved` went cyan → green in the palette pivot (owner decision, 2026-07):
red/green like Laravel's own ecosystem, with the old cyan retired to
`--logo-accent`. `--brand` red is **unbanned** from the working UI but used
only for genuine unresolved-state semantics and identity — never as a
decorative accent. `--on-accent` (readable text on a resolved fill) stays.

**Colorblind redundancy (HARD RULE with the red/green palette):** red vs.
green is the most common color-vision collision, so the resolved/unresolved
distinction must never be carried by hue alone. Unresolved ER edges always
carry a non-color marker (midpoint break glyph) in addition to red; severity
keeps its leading dot + label; diff states carry +/− glyphs. Any new
red-vs-green surface must name its non-color channel in the PR.

### Status (kept distinct from the thread roles)

`--danger` (cooler red), `--warn` (amber), `--ok` (green) remain for
severity/health. `--danger` stays deliberately distinct from
`--unresolved`/`--brand` so a "problem" is never confused with the metaphor's
"unresolved." `--ok` and `--resolved` now deliberately share one green hue
family: "resolved" and "healthy" overlapping is a harmless confusion, and
fewer hues keep the shell calm. Low-alpha tints (`--danger-tint`,
`--warn-tint`, `--ok-tint`) drive dot+tint severity (see §5).

**Contrast:** every text-on-surface pairing meets WCAG AA. The prior amber-at-11px
failure is fixed — small colored text is ≥12px and uses a lighter shade on dark.

## 2. Typography

### Faces (kept)

- `--display: "Instrument Sans"` — the **human voice**: the tool's sentences,
  headings, verdicts, the wordmark.
- `--mono: "JetBrains Mono"` — the **machine voice**: every project identifier
  (table, column, route URI, model, controller, method).
- `--sans` (system stack) — body/UI reading text only, zero webfont cost.

### Face split (HARD RULE)

> Any project identifier renders in `--mono`. Any tool-authored sentence renders
> in `--display` (headings) or `--sans` (body). A reader must be able to tell
> "the tool talking" from "your code" by typeface alone.

This is semantic, not stylistic. It also replaces the banned uppercase reflex as
the primary hierarchy device.

### Scale — real ~1.25 modular scale (kills the flat-scale flag)

Display steps are **tokens**, not ad-hoc literals. Headings out-scale body
through the system.

| Token | px | Use |
|---|---|---|
| `--text-2xs` | 11 | dense metadata only (never colored small text) |
| `--text-xs` | 12 | chips, captions |
| `--text-sm` | 13 | table body, secondary |
| `--text-base` | 15 | body (up from 14 for legibility) |
| `--text-lg` | 19 | lead paragraph, sub-headings |
| `--text-xl` | 24 | section headings |
| `--text-2xl` | 30 | view titles |
| `--text-3xl` | 38 | Overview hero / wordmark |
| `--text-hero` | `clamp(38px, 6vw, 60px)` | the un-ravel wow number/mark |

Steps ≥ 1.25 ratio at the top end where hierarchy must read. Weight contrast:
body 400, emphasis 600, display 600–700. Uppercase reserved for true section
headings only.

## 3. Spacing & rhythm

Keep the 4→48 scale (`--space-1`…`--space-12`); it has genuine variety. Rule:
**vary padding to create rhythm** — a hero breathes (`--space-12`), a table row
is tight (`--space-2`). No single padding value applied everywhere. Cap prose at
65–75ch.

## 4. Borders, radius, elevation

- **Borders:** 1px hairlines only. The `--border` / `--border-strong` pair.
  **No border > 1px used as a colored accent** (anti-ref #2), full stop.
- **Radius:** `--radius-sm 5 / --radius-md 8 / --radius-lg 10` (kept).
- **Elevation:** flat and matte. The only shadow is the command-palette modal
  lift (neutral black, functional). No colored glow, no ambient effects, no
  decorative glassmorphism.

## 5. Components — the rules that fell out of the system

### Severity (replaces the banned side-stripe)

Never a colored left border. Severity reads as: a **leading status dot** (`--danger`
/ `--warn` / `--ok`) plus a **background tint** (`--danger-tint` etc.), optionally
a leading label. Applies to findings, model-cards, dead-route rows, and the Auth
view's unauthenticated-route rows (a write reads as the danger dot *and* says
"write", so the signal survives without hue).

### Stat display

Static, subordinate, and honest inside working views. Enlarged/animated **only**
in the Overview un-ravel tally, where the number *is* the count of what un-raveled
(anti-ref #4). Delete the `.cards .card .n` dimmed-numbers hack — the working-view
stat strip is quiet *by size and placement*, not by apologizing in CSS.

### ER edges (the metaphor, structural)

- Resolved relationship → `--resolved` (green). Schema FK solid, Eloquent relation
  dashed (distinguishable, kept).
- Unresolved / name-mismatched → `--unresolved` (red), dashed, **plus a
  midpoint break glyph** so the state survives red/green color blindness
  (a red dashed edge and a green dashed Eloquent edge must not differ by hue
  alone). This is the [[er-edge-origin-followup]] `Origin` flag made visible.

### Entity-chip

The shared "jump-to-a-linked-thing" affordance. `--resolved` green, mono, subtle
underline on hover. The `is-plain` degraded form is dimmed, non-interactive.

### Nav (kills the 2px active stripe)

Active state reads as a filled/tinted background or a leading marker — not a
left border > 1px.

## 6. Motion

- One easing family: ease-out (`cubic-bezier(0.2, 0, 0, 1)`). No bounce, no
  elastic. Never animate layout properties (width/height/top/left/margin);
  animate `transform`/`opacity` only.
- **The settle is the signature gesture** (ADR 0009, now promoted app-wide):
  views resolve in from a slightly-tangled state on first paint; a small
  persistent thread-mark in the sidebar re-settles on each new analysis. Used as
  identity, kept scarce — not every hover, not every list.
- Functional transitions stay fast (`--motion-fast 150ms`, `--motion-view 200ms`).
- Every motion degrades under `prefers-reduced-motion`: the resolved end-state
  shows statically, no dance.

## 7. The Overview wow moment (gate passed — variant A shipped)

> **Status:** the throwaway prototype was judged by the owner and variant A
> (the constellation) landed. It ships as `overview-wow.js` +
> `overview-constellation.js` (issue #44): once per analysis, held until the
> Overview is visible, static resolved end-state under reduced motion, colors
> read from the token layer at play time.

The one brand-register surface. The Laravel mark un-ravels into an **abstract
constellation** of the project's entities (points, not the literal 54-table ER),
threads resolving red→green, stats **tallying the un-ravel** in sync. Ends on the
verdict. **Build only after a `/prototype` throwaway proves it lands**; fallback
is the logo resolving red→green with a synced stat count-up. Must degrade to a
static resolved end-state under reduced motion and never jank on a slow machine.

## 8. Conformance checklist (grep-able)

Before any visual PR merges:
- [ ] No `border-left`/`border-right` > 1px as a colored accent (anti-ref #2)
- [ ] No pure `#000`/`#fff` outside the documented Swagger frame (anti-ref #1)
- [ ] No em dash (—) or `--` in UI copy (anti-ref #3)
- [ ] No `text-transform: uppercase` outside true section headings (anti-ref #5)
- [ ] Every project identifier in `--mono`; tool sentences in display/sans (§2 rule)
- [ ] No stat enlarged/animated outside the Overview tally (anti-ref #4)
- [ ] Thread/red→green used only for resolution/connection, never nav/decor (anti-ref #6)
- [ ] Cyan appears only as `--logo-accent` on the logo's endpoint dot (§1)
- [ ] Every red-vs-green distinction carries a non-color channel (dot, label, glyph, dash+marker) (§1)
- [ ] Neutrals authored in OKLCH, warmed (§1)
- [ ] Every text-on-surface pairing meets WCAG AA (§1)
