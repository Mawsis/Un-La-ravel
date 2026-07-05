# PRODUCT.md — Un(la)ravel

> The intention layer of the design system. DESIGN.md derives its tokens and
> component rules from what is stated here. When a visual decision is ambiguous,
> this file is the tiebreaker, not personal taste.

## Register

**product** — the design serves the tool. Un(la)ravel is an analysis dashboard a
developer works inside, not a marketing surface. One deliberate exception: the
Overview view carries a **brand-register wow moment** on first contact (see
"Dual register" below). Everywhere else, design is in service of legibility and
speed.

## Product purpose

Un(la)ravel statically analyzes a Laravel project — no boot, no `artisan`, no
database, no `vendor/` — and un-ravels it into one resolved Project Model that
every view renders: schema, Eloquent models, routes, controllers, form
requests, an ER diagram, an OpenAPI spec, and a health verdict. It works on
legacy and half-broken projects precisely because it never runs them.

## Users

- **Primary — the intermediate Laravel developer.** Knows the framework, is
  auditing or onboarding onto a real (often legacy, often messy) codebase. They
  do not linger on a landing view; they go straight to the surface they need —
  Swagger, Models, Routes, the ER diagram. For them the tool must be **calm,
  fast, and legible over long working sessions**.
- **Secondary — the first-contact newcomer.** Points the tool at a project for
  the first time. For them the first view must **impress** and make the product
  thesis land in one gesture: "this thing un-ravels my Laravel project."
- Both are members of the **Laravel community**, which has a strong shared
  aesthetic culture (the red mark, a craft-and-developer-happiness ethos, the
  Herd/Pulse/Nightwatch visual language). A wow moment lands when it feels like
  it *belongs* in that world.

## North star (one sentence)

> Un(la)ravel is a workbench for Laravel developers that takes a tangled, often
> half-broken legacy project and visibly un-ravels it into resolved, readable
> structure. Its identity is the thread: **red where the source is still tangled
> or unresolved, cyan where the tool has resolved it.** It speaks with the calm
> confidence of a senior engineer narrating over raw code — impressive at first
> contact, quietly efficient in daily use.

Everything downstream derives from this sentence:

- Red and cyan are **semantic states, not decoration.** The palette has an
  `unresolved` role and a `resolved` role.
- Type is a **human-voice / machine-voice split**: the tool's sentences in the
  display face, every project identifier in mono.
- Motion means **resolution** — one gesture (the settle), used as identity, not
  sprinkled as decoration.
- The register is **dual**: wow for the newcomer's first contact, calm for the
  intermediate's daily use.

## The thread metaphor (scarcity rule)

The thread / red→cyan / un-ravel gesture is the product's one ownable idea. It
is powerful **because it is scarce**. It appears only where there is a genuine
*resolution* or *connection*:

- The Overview constellation (the mark un-raveling into the project's structure).
- ER edges: resolved relationships in cyan, unresolved (name-mismatched, dangling)
  in red.
- Entity-chips that jump to a linked thing.

It must **never** appear as: a severity indicator, a nav-state accent, a card
decoration, or ambient background. Metaphor wallpaper is how this tool became
slop the first time. Keep it rare.

## Dual register

| | Newcomer (first contact) | Intermediate (daily use) |
|---|---|---|
| Surface | Overview | Swagger, Models, Routes, ER, Findings |
| Job | Impress; land the thesis in one gesture | Legible, fast, calm over hours |
| Volume | Loud: the un-ravel wow moment | Quiet: honest data, no spectacle |
| Stats | Meaningful, animated (the un-ravel tally) | Present, static, subordinate to the data |

The loud and the quiet must not bleed into each other. The Overview may be
cinematic; the routes table may not.

## Tone & voice

The calm confidence of a senior engineer reading your code back to you. Direct,
specific, never chirpy. States what it found and what it means. No exclamation
marks in chrome, no marketing adjectives, no reassurance theater. When it flags
a problem it names the problem and the fix, factually.

## Anti-references (hard bans — the anti-slop machine)

These are the exact mechanisms that produced the "AI slop" the tool is being
rebuilt to escape. Each is **grep-able**; a reviewer or future agent can check
for the violation. They are rules, not guidelines.

1. **No Tailwind-default neutral ramp.** Pure `#0a0a0a / #171717 / #262626`
   grays are the single biggest template tell. Neutrals are OKLCH, faintly
   warmed toward the brand red hue.
2. **No colored vertical side-stripes, ever.** No `border-left`/`border-right`
   greater than 1px as a colored accent — not on cards, findings, model cards,
   callouts, or nav. Severity is a leading status dot + background tint. No
   exceptions, and no "thread costume" that revives the pattern under the
   metaphor's name.
3. **No em dashes in UI copy** (also a standing repo rule). Periods, commas,
   colons, parentheses.
4. **No hero-metric template as decoration.** Stats enlarge or animate **only**
   when they carry real meaning (the Overview un-ravel tally). Never a row of
   interchangeable inert tiles in a working view.
5. **No uppercase-micro-label reflex.** `text-transform: uppercase` is reserved
   for true section headings only. Hierarchy comes from scale, weight, and the
   display/mono face contrast — not letter-spacing tricks.
6. **No metaphor wallpaper.** See the scarcity rule above. The thread means
   resolution/connection and nothing else.

Plus the shared impeccable bans: no gradient text, no decorative glassmorphism,
no neon/colored glow shadows, no modal-as-first-thought.

## What must survive the makeover (keep list)

The prior build was disciplined slop — real thought, no committed point of view.
These parts are genuinely good and are kept and pushed harder, not discarded:

- The **semantic token architecture** (naming color by role, one file as source
  of truth). Push the values; keep the structure.
- The **un-ravel logo SVG + wordmark** — the one ownable identity asset. It was
  hidden after first analysis; the makeover promotes it.
- The **settle animation** (ADR 0009) — the truest expression of the metaphor,
  already built. Promoted from one panel to the app's signature gesture.
- The **three-stack type strategy** (display for identity, mono for code,
  system-sans for zero-cost body). Kept; the scale is what changes.
- **Determinism, non-nil empty slices, parser isolation, static-only** — the
  repo's hard conventions are untouched by any visual work.
