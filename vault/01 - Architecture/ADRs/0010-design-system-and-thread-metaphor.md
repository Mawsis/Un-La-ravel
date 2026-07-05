---
tags: [adr]
status: accepted
date: 2026-07-04
aliases: ["ADR 0010 - Design system and the thread metaphor"]
---

# ADR 0010 — A committed design system: the thread metaphor as semantic color roles, warmed OKLCH neutrals, and a modular type scale

Two independent design reviews scored the dashboard 28/40: "good, not
distinctive" — a generic dark admin template with flat stat cards, an arbitrary
cyan accent, and Tailwind-default grays. The diagnosis was not carelessness.
The token architecture (issue #20), the self-hosted fonts (#22), and the motion
policy (#25, revised by [[0009-er-settle-animation-and-export|ADR 0009]]) are
each genuinely thoughtful. The cause was an **absent intention**: with no
stated north star, every component made a locally reasonable decision, and
local decisions with no shared point of view converge on the generic. The one
ownable idea — the un-ravel thread, red where the source is tangled, cyan where
the tool resolved it — lived only in code comments.

## Context

- The product already *had* the metaphor: `tokens.css` described `--brand` as
  the "source pole" and `--accent` as the "resolved pole", and ADR 0009 shipped
  the settle animation as "the un-ravel metaphor at the moment structure
  resolves". But nothing enforced the metaphor, so the working UI read as a
  cyan-accented template where the story was invisible.
- Per the repo's own convention, a change to a load-bearing convention needs an
  ADR. The token layer's palette, naming, and scale are consumed by every
  stylesheet and pinned by `restyle_test.go` — this is that change's record.

## Decision

**The intention is committed as files, senior to taste.** `PRODUCT.md` (the
intention layer: north star, users, dual register, tone, hard anti-references)
and `DESIGN.md` (the derived system: color, type, spacing, component rules,
conformance checklist) live at the repo root. When a visual decision is
ambiguous, they are the tiebreaker. Where `DESIGN.md` and CSS disagree, the CSS
is wrong.

**The thread metaphor becomes semantic state roles, not accents.**

- `--accent` (cyan) is renamed **`--resolved`**: "the tool resolved this".
  Interactive chrome, resolved ER edges, entity-chips, focus.
- A new **`--unresolved`** (brand-adjacent red) means "still tangled":
  unresolved ER edges, disagreements, name-mismatch flags. This is the color
  half of the [[er-edge-origin-followup|ER edge Origin]] follow-up.
- `--brand` (Laravel red) stays identity-only, but is **unbanned** for genuine
  unresolved-state semantics. Never decoration — the metaphor is powerful
  because it is scarce (PRODUCT.md scarcity rule).
- `--danger`/`--warn`/`--ok` stay deliberately distinct from the thread roles
  so a *problem* is never confused with the metaphor's *unresolved*.
- `--on-accent` keeps its name: it describes the on-fill text relationship,
  not a hue, and renaming it bought nothing.

**Neutrals move to warmed OKLCH.** Every neutral is authored in OKLCH with a
small chroma (0.004–0.010) tilted toward the brand hue (~25°) — killing the
pure Tailwind-neutral ramp (anti-reference #1), present enough to feel like
*this* red-branded tool, low enough that a routes table read for an hour never
feels tinted.

**Type becomes a real ~1.25 modular scale with display-step tokens.** Body
moves 14→15px; the top end gains hierarchy (19/24/30/38 + a fluid
`--text-hero` clamp). Display sizes are tokens; ad-hoc `font-size` pixel
literals in consumer stylesheets are now a test failure. The
human-voice/machine-voice face split (tool sentences in display/sans, every
project identifier in mono) is stated as a hard rule for later view PRs.

**Small colored text meets the legibility floor.** `--text-2xs` (11px) is
dense metadata only, never colored; colored labels sit at 12px+; amber
lightened one step for dark surfaces.

**The six anti-references become a grep-able conformance gate** (DESIGN.md §8),
applied as a manual review checklist on every visual PR: no Tailwind-default
grays, no colored side-stripes > 1px, no em dashes in UI copy, no hero-metric
filler, no uppercase-micro-label reflex, no metaphor wallpaper — plus the
shared bans (no gradient text, decorative glassmorphism, colored glow,
modal-first).

## Consequences

- **Three of the gate's violations are known debt, deliberately left standing**
  for the working-view reskin PRD, because this PRD is token-layer-only: the
  nav active 2px cyan stripe (`layout.css`), the finding card's 3px colored
  `border-left` (`components.css`), and the `.cards .card .n` dimmed-numbers
  hack. The gate exists to stop *new* slop from merging; the reskin PRD removes
  the existing three.
- `restyle_test.go` now pins the system instead of issue #20's palette: thread
  role names present, `--accent` absent everywhere, OKLCH neutrals, the
  modular scale, no ad-hoc font-size literals, no colored 2xs text. A future
  palette change edits `tokens.css` and these assertions together, on purpose.
- The keep list is explicit (PRODUCT.md): semantic token architecture, the
  logo/wordmark, the settle animation, the three-stack type strategy, and all
  engine-side hard conventions are untouched. This makeover pushes those
  values harder; it does not start over.

## Considered and rejected

- **Restyle components in the same PR as the token migration.** Rejected: the
  vertical-slice convention holds. This slice is tokens + docs + gate; the
  working-view reskin and the Overview wow moment are separate PRDs that
  depend on it.
- **Automate the conformance gate as a test suite now.** Rejected for this PR:
  the checklist items that are cheaply grep-able are already tests
  (`restyle_test.go`); the judgment-heavy ones (metaphor wallpaper, hero-metric
  filler) would produce false positives — precision over coverage, same logic
  as ADR 0002. The checklist runs as a manual review gate.
- **Keep `--accent` and just add `--unresolved`.** Rejected: an "accent" is by
  definition decorative — the rename is the point. Color that says what it
  *means* is what prevents the next arbitrary hue swap.
