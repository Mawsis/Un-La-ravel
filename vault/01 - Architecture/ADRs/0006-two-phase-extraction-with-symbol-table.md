---
tags: [adr]
status: accepted
date: 2026-06-30
aliases: ["ADR 0006 - Two-phase extraction with symbol table"]
---

# ADR 0006 — Two-phase extraction with a symbol table

Extraction runs in **two phases**. Phase 1 walks *all* PHP files and builds a **symbol table** (fully-qualified class name → file → AST node) plus per-file `use`-import maps. Phase 2 resolves every cross-file reference (Route→Controller, Model→Model, Route→FormRequest) against that table. References that don't resolve become explicit **dangling edges** — which doubles as **dead-route / dead-reference detection**.

## Context

The *value* of un-ravelling is the **edges**, and edges cross file boundaries. A route `Route::post('/posts', [PostController::class, 'store'])` only names `PostController`; resolving it to `App\Http\Controllers\PostController` requires the file's `use` imports, namespace context, and possibly a `Route::namespace()` group or legacy `'PostController@store'` string syntax. The reference is ambiguous until name resolution runs against the whole project.

## Decision

**Level 1 — two-phase with a symbol table.**

- **Phase 1 (collect):** parse every file; record each declared class by FQN and each file's `use` aliases.
- **Phase 2 (resolve):** for every cross-file reference, resolve the short name → FQN → symbol-table entry. Unresolved → a typed **dangling edge** carrying the unresolved reference.
- Dangling edges are surfaced, not hidden: a route pointing at a missing/renamed controller is a **dead route**.

## Consequences

- Edges are **correct**, not guessed — a route map that draws wrong arrows is worse on a portfolio than one that draws none.
- **Free feature:** dead-route / dangling-reference detection, recovering part of the deferred "code quality" category ([[Roadmap]]) at near-zero cost.
- The engine owns a **symbol table** component (normal, well-understood).
- Implies extraction can't be purely per-file-parallel: Phase 1 must complete before Phase 2 resolves. (Phase 1 itself can be concurrent.)

## Considered and rejected

- **Level 2 — single-pass best-effort string matching** (match `PostController` by short name). Rejected: collides on same-short-name classes in different namespaces and **silently draws wrong edges** — the diagram lies. Faster, but corrupts the product.
- **Level 3 — no cross-file edges in MVP.** Rejected on sight: the edges *are* the product.
