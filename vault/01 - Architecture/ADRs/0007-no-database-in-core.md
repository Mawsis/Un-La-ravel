---
tags: [adr]
status: accepted
date: 2026-06-30
aliases: ["ADR 0007 - No database in core"]
---

# ADR 0007 — No database in the core (drop GORM/SQLite/Postgres)

The engine has **no database**. The [[CONTEXT|Project Model]] lives in memory during a run and serializes to `unlaravel.json` ([[ADR 0004 - Serialized Project Model as output contract|ADR 0004]]). If incremental caching is ever needed, it's a **content-hash cache** (no ORM, no SQL). PostgreSQL / multi-project is a property of the **deferred web service**, not the engine.

> [!warning] This contradicts `CLAUDE.md`
> The original brief specifies GORM + SQLite ("local analysis cache, relationship storage, fast queries") and PostgreSQL ("multi-project / team"). This ADR overrides that for the MVP. A reader seeing GORM in `CLAUDE.md` and no database in the code should read this ADR. `CLAUDE.md` should be updated.

## Context

After [[ADR 0004 - Serialized Project Model as output contract|ADR 0004]], the model is an in-memory graph and a JSON file. A Laravel app yields hundreds to low-thousands of nodes — trivially in-memory, queried in microseconds. The brief's three justifications don't hold for the MVP:

- **Relationship storage / fast queries** — an in-memory graph already does this; a DB would mean loading JSON into SQLite to read it back.
- **Incremental cache** — the only semi-real need, but a content-hash → cached-result map solves it in ~40 lines; SQLite+GORM is overkill.
- **Multi-project / Postgres** — belongs to the deferred web service, which is out of the "CLI + package" scope.

## Decision

No DB in the core. In-memory model + `unlaravel.json`. Optional future content-hash cache. DB is a web-service concern only.

## Consequences

- **Single static Go binary, zero runtime deps** — a stronger portfolio artifact, and clean cross-compilation.
- **Directly supports [[ADR 0005 - Laravel package is thin wrapper over Go binary|ADR 0005]]:** avoids CGo-SQLite, which would complicate the cross-platform binary distribution the Composer package depends on.
- Avoids "premature infrastructure" — designing the core around a DB for a feature (team/web) that was cut.

## Considered and rejected

- **Keep GORM + SQLite as the brief specifies.** Rejected: answers no query an in-memory graph + JSON can't, adds a CGo dependency that fights binary distribution, and reads as scope creep on a portfolio.
