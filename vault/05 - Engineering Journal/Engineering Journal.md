---
tags: [journal, moc]
aliases: [Engineering Journal]
---

# Engineering Journal

Dated log of decisions and findings, newest first. Decisions that are hard-to-reverse get promoted to an [[ADR Index|ADR]]; everything else is captured here for narrative.

---

## 2026-06-30 — Milestone 0 + 1 SHIPPED: the vertical slice works

PRD [#1](https://github.com/Mawsis/Un-La-ravel/issues/1) (M0 + M1) implemented and verified. The repo now **builds, runs, and tells the truth**.

### What landed
- **`cmd/unlaravel/main.go`** — the missing entrypoint. Repo builds into a binary (the #1 fix).
- **`internal/model`** — the Project Model + versioned JSON contract (`schema_version`, snake_case throughout — see decision below).
- **`internal/phpast`** — the deep wrapper around `VKCOM/php-parser` v0.8.2; the ONLY package importing the parser ([[ADR 0003 - Static analysis without booting Laravel|0003]]).
- **`internal/extract/schema`** — migrations → Schema nodes (create, `Schema::table` alter-merge, `foreignId`/explicit-`foreign()` FK, `timestamps()`/`softDeletes()` expansion, both modern anon-class and classic named-class shapes).
- **`internal/render/er`** — Project Model → Mermaid ER diagram.
- **`internal/cli`** — fake `✅` output deleted; real pipeline wired (`detect → parse → extract → model → json → render`).
- **`CLAUDE.md`** — corrected: real parser ref ([[ADR 0003 - Static analysis without booting Laravel|0003]]) + database marked deferred ([[ADR 0007 - No database in core|0007]]).

### De-risking that paid off
Before fanning out agents, ran a **spike** that parsed a real Laravel migration and extracted its table+columns — this proved the `php-parser` v0.8.2 visitor model (per-node-type methods, embed `visitor.Null`; NOT a generic `EnterNode`) and caught a wrong dispatch assumption *before* it reached six agents.

### Verified (clean, uncached, -race)
Build + vet + gofmt clean. All tests PASS. Coverage on the four deep modules: extract/schema **90.2%**, model **87.5%**, phpast **89.1%**, render/er **93.2%** — all over the 80% bar. The binary produces a real ER diagram + valid `unlaravel.json` from the fixture app; cross-migration ALTER merge confirmed live (`posts.category_id` from a separate `Schema::table` migration). Honest error (exit 1) on a non-Laravel dir.

### Decision: JSON contract is snake_case
The verify pass caught a mismatch — code emitted `schema_version` but [[ADR 0004 - Serialized Project Model as output contract|ADR 0004]] prose said `schemaVersion`. Resolved in favour of **snake_case everywhere** (consistent with `project_name`/`laravel_version`/`schemas`); ADR 0004 and the doc comment were amended to match. Not a new ADR — it's a clarification of 0004, not a reversal.

---

## 2026-06-30 — Grilling session: architecture nailed down

Full interactive grill ([[grill-with-docs]]) to decide how to finish Un(la)ravel as a portfolio piece. Seven decisions resolved, all recorded as ADRs.

### Starting-state assessment (the honest truth)

Read the entire codebase — it's near-greenfield, **3 Go files**:

- **`internal/detector/composer.go`** — ✅ real, working. Parses `composer.json`, extracts Laravel version + deps, flags `laravel/*` core packages.
- **`internal/detector/laravel.go`** — ✅ real, working. Checks for `artisan`, builds an in-memory directory tree (skips `.git`/`node_modules`/`vendor`).
- **`internal/cli/root.go`** — ⚠️ Cobra shell, but `analyzeProject` is **fake**: it prints hardcoded `green.Println("✅ Database schema analyzed")` etc. No analysis happens, and it **never calls the working detector**.

> [!danger] Two critical gaps found
> 1. **No `cmd/unlaravel/main.go`.** `CLAUDE.md` says `go build ./cmd/unlaravel`, but that package doesn't exist and nothing calls `cli.Execute()`. **The repo does not build into a runnable binary.** This is the single most important fact about its real state.
> 2. **The working detector is orphaned** — built but never wired to the fake CLI command.

### Decisions made (→ ADRs)

1. **[[ADR 0001 - Project Model as the core abstraction|0001]]** — Reframed the brief's ~30 features as *one Project Model engine + many renderers*. "Everything" survives as a renderer roadmap, not 30 parallel builds.
2. **[[ADR 0002 - Six-node MVP scope|0002]]** — MVP = 6 node types (Schema, Model, Route, Controller, Middleware, FormRequest). Deferred Resources + N+1.
3. **[[ADR 0003 - Static analysis without booting Laravel|0003]]** — Real PHP AST via `VKCOM/php-parser`. **Found `CLAUDE.md` references a fictional package** (`go-ast-php/php-parser-go` → HTTP 404 on the Go proxy). VKCOM is real (v0.8.2) but caps at ~PHP 8.1.
4. **[[ADR 0004 - Serialized Project Model as output contract|0004]]** — `unlaravel.json` (versioned) is the public contract. Renderers + package consume it.
5. **[[ADR 0005 - Laravel package is thin wrapper over Go binary|0005]]** — Disambiguated "Laravel package": it's a thin Composer wrapper (`artisan unlaravel:analyze`) over the Go binary, **not** a native PHP re-implementation.
6. **[[ADR 0006 - Two-phase extraction with symbol table|0006]]** — Two-phase extraction + symbol table for correct cross-file edges; dangling refs = free dead-route detection.
7. **[[ADR 0007 - No database in core|0007]]** — Dropped GORM/SQLite/Postgres from the MVP (**overrides `CLAUDE.md`**). In-memory graph + JSON; DB is a deferred web-service concern.

### Follow-ups queued (not yet done)

- [ ] **Update `CLAUDE.md`** for the two corrections it now contradicts: the fictional parser package ([[ADR 0003 - Static analysis without booting Laravel|0003]]) and the GORM/SQLite strategy ([[ADR 0007 - No database in core|0007]]).
- [ ] Begin [[Roadmap|Milestone 0]]: add `main.go`, delete fake output, wire the detector.

### Terms sharpened (→ [[CONTEXT]])

- "feature" → **Extractor** / **Renderer** / **Node** (banned in architecture talk)
- "Schema" vs "Migration" vs "Model" — three distinct concepts; Schema is the *result* of Migrations, Model *maps onto* it
- "everything" → "every output is a Renderer over one Project Model"
