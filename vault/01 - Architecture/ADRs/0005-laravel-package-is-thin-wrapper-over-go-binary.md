---
tags: [adr]
status: accepted
date: 2026-06-30
aliases: ["ADR 0005 - Laravel package is thin wrapper over Go binary"]
---

# ADR 0005 — The Laravel package is a thin Composer wrapper over the Go binary

The "Laravel package" is a **Composer package that registers an Artisan command** (`php artisan unlaravel:analyze`) and **delegates to the single Go engine**, consuming/emitting the shared `unlaravel.json` ([[ADR 0004 - Serialized Project Model as output contract|ADR 0004]]). It is **not** a native PHP re-implementation of analysis. The brain stays in Go; the package is convenience glue.

## Context

"CLI + Laravel package" was chosen as the definition of done, but "Laravel package" had two opposite meanings: (1) a thin wrapper around the Go binary, or (2) a native PHP package that introspects the *booted* Laravel app at runtime. Interpretation 2 would silently discard [[ADR 0001 - Project Model as the core abstraction|ADR 0001]] (one engine) and [[ADR 0003 - Static analysis without booting Laravel|ADR 0003]] (no booting Laravel) — it's a different project in a different language.

## Decision

**Interpretation 1.** The Composer package:

- Registers `php artisan unlaravel:analyze` (and friends) via a service provider.
- Locates/ships the platform-appropriate Go binary and shells out to it.
- Reads/writes the shared `unlaravel.json`; renders the same outputs.
- Contains minimal PHP (~glue), consistent with this being a Go-learning project.

## Consequences

- Consistent with every prior ADR: one engine, static analysis, shared JSON contract.
- Portfolio sentence is stronger: *"a Go static-analysis engine with a native Artisan integration"* beats *"a PHP package that calls Laravel's router."*
- **New cost: binary distribution via Composer** — platform-specific binaries delivered by a post-install download hook (the `php-download`-style pattern). Solvable, and itself a "I solved cross-language packaging" story. Tracked in [[Roadmap]].
- The package still does **not** require a bootable app — it analyzes files, same as the CLI.

## Considered and rejected

- **Interpretation 2 — native PHP analysis inside booted Laravel.** Rejected: discards the Go engine, the AST work, the static premise, and the single-binary story; requires a bootable app; contradicts ADRs 0001 and 0003. Accurate routes, but a different and lesser project.
