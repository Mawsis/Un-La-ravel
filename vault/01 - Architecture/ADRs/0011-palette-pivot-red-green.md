---
tags: [adr]
status: accepted
date: 2026-07-05
aliases: ["ADR 0011 - Palette pivot: resolved goes green, cyan retires to the logo"]
---

# ADR 0011 — Palette pivot: `--resolved` goes green, cyan retires to the logo accent

The owner reviewed the shipped design system ([[0010-design-system-and-thread-metaphor|ADR 0010]])
against a real 54-table project and made a taste call with teeth: the cyan
resolved pole does not read as *this* product. The working palette becomes
**red/green** — the pair Laravel's own ecosystem speaks — and cyan survives
only as the logo's endpoint dot.

## Context

- ADR 0010 committed the thread metaphor as two state roles: `--unresolved`
  (brand-adjacent red) and `--resolved` (cyan). Every stylesheet reads the
  token, so the hue lives in exactly one place (`tokens.css`).
- The spec deliberately kept `--ok` (green) distinct from `--resolved` (cyan)
  so "healthy" and "resolved" could not be confused. With green taking the
  resolved role, that separation is no longer possible on hue.
- Red vs. green is the most common color-vision deficiency (~8% of men).
  Under ADR 0010 the ER edge legend leaned on hue plus dash pattern, but a
  red *dashed* unresolved edge and a green *dashed* Eloquent edge would now
  differ by hue alone.

## Decision

1. **`--resolved` becomes green** — `oklch(0.75 0.17 152)`, hover
   `oklch(0.82 0.14 152)`. All interactive chrome, entity-chips, resolved ER
   edges, and the wow moment's resolution gesture follow automatically via
   the token layer.
2. **Cyan retires to `--logo-accent`** — `oklch(0.78 0.13 210)`, allowed only
   on the logo SVG's endpoint dot (hero + sidebar marks in `index.html`).
   It never appears in the working UI. The conformance checklist gains a
   grep-able line for this.
3. **`--resolved` and `--ok` deliberately share one green hue family.** The
   confusion this permits ("resolved" vs. "healthy") is on the positive side
   and judged harmless; fewer hues keep the shell calm. `--danger` stays a
   cooler red, distinct from `--unresolved`/`--brand`, so "problem" is still
   never confused with "unresolved".
4. **Colorblind redundancy becomes a HARD RULE** (DESIGN.md §1): no
   red-vs-green distinction may ride on hue alone. Unresolved ER edges gain a
   midpoint break glyph in addition to red; severity keeps dot + label; diff
   states (future) carry +/− glyphs. Every new red-vs-green surface must name
   its non-color channel in the PR.

## Consequences

- One-token re-skin: because ADR 0010 centralized hue in `tokens.css`, the
  pivot touched two token values, one new token, two SVG `fill` attributes,
  and comments. No component stylesheet changed a rule.
- The ER edge break-glyph (decision 4) is spec-first: DESIGN.md requires it,
  `er-svg.js` does not draw it yet. It ships with the next ER edge PR.
- PRODUCT.md's north star sentence and scarcity rule now read red→green;
  the metaphor is unchanged, only its resolved hue.
- `restyle_test.go` pins token *names*, not hues, so the suite is unaffected
  — deliberate under ADR 0010 and validated by this pivot.
