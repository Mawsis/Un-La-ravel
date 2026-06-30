---
tags: [adr]
status: accepted
date: 2026-06-30
aliases: ["ADR 0001 - Project Model as the core abstraction"]
---

# ADR 0001 — Project Model as the core abstraction

We model Un(la)ravel as **one engine that builds a single [[CONTEXT|Project Model]] of a Laravel app, plus many thin [[CONTEXT|Renderers]]** over that model — rather than ~30 independent feature scripts as the original `CLAUDE.md` vision implied.

## Context

The original brief listed ~30 capabilities across 8 categories (routes, DB, security, perf, …). Built as parallel features, each would land at demo quality, reading to a portfolio reviewer as "started big, finished nothing." But nearly every listed capability is secretly a *read* over the same data: route analysis, Swagger, and middleware flow all need routes+controllers; ER diagrams and migration analysis all need migrations+models.

## Decision

There is **one hard thing**: a typed in-memory graph (the **Project Model**) populated by **Extractors** and consumed by **Renderers**. Renderers never touch source files. "Features" are reframed as renderers/extractors — a roadmap, not parallel builds.

## Consequences

- The breadth of the original vision survives as a **renderer roadmap** ([[Roadmap]]), not as scope cut.
- The word "feature" is retired from architecture discussion in favour of **Extractor** / **Renderer** / **Node** — see [[CONTEXT]].
- Adding an output later is cheap (a new Renderer); adding a new source of truth is the only expensive move (a new Extractor + Node type).
- This is the literal "un-ravel" thesis: one tangled project → one clean model → many clear views.

## Considered and rejected

- **30 parallel feature scripts.** Rejected: no shared model means duplicated PHP-reading logic, inconsistent output, and a repo that looks unfinished.
- **Boot Laravel and introspect at runtime** (e.g. shell out to `php artisan route:list`). Deferred/rejected for the core — see [[ADR 0003 - Static analysis without booting Laravel]].
