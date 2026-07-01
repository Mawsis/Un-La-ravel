---
tags: [journal, moc]
aliases: [Engineering Journal]
---

# Engineering Journal

Dated log of decisions and findings, newest first. Decisions that are hard-to-reverse get promoted to an [[ADR Index|ADR]]; everything else is captured here for narrative.

---

## 2026-06-30 — SIX-NODE MVP COMPLETE: FormRequests → OpenAPI 3

PRD [#5](https://github.com/Mawsis/Un-La-ravel/issues/5) implemented and verified. This is the **final node type** — all six of Schema, Model, Route, Controller, Middleware, and FormRequest now extract → Project Model → `unlaravel.json`, and the tool emits a **Swagger-loadable `openapi.json`**.

### What landed
- **`internal/extract/formrequest`** — parses `app/Http/Requests` `rules()` arrays (both `'a|b|c'` string and `['a','b']` array syntaxes; rules with args like `max:255`, `in:a,b,c`) into source-faithful `Rule{Name,Args}`. 91.5% covered.
- **`internal/extract/controller`** (extended) — now also captures each action's type-hinted parameter classes, so a FormRequest can be linked to its route.
- **`internal/analyze/formrequest.go`** — links Route→FormRequest via the action's typed param, resolved through the **ADR 0006 symbol table** (its second consumer, validating that investment). False-positive-safe. 99.1% covered.
- **`internal/render/openapi`** — the showpiece: Project Model → valid OpenAPI 3.0.3 (`paths`, path params, `requestBody` from FormRequest rules, default 200s). Contains the **rules→JSON-Schema mapping**. 92.3% covered.
- **`internal/cli`** — `--openapi <path>` flag; FormRequests wired into the pipeline. `schema_version` 1.2.0 → 1.3.0.
- **`Roadmap.md`** — six-node MVP marked COMPLETE.

### De-risking caught two real OpenAPI bugs before implementation
The mapping spike surfaced, and the agents then fixed + regression-tested, both:
- **Numeric constraints must be JSON numbers, not strings** — `maxLength: 255`, not `"255"` (invalid OpenAPI). Guarded by a test asserting the number and the absence of the string.
- **Type must resolve before max/min (order-independence)** — a two-pass mapper, proven by an explicit `max:100` *before* `integer` test dispatching to `maximum` (not `maxLength`).

### Verified (clean, uncached, -race)
Build + vet + gofmt clean. All 13 packages PASS. Live run: 3 tables, 3 models, 11 routes, 1 dead route, **1 FormRequest** — the full six-node model in one `unlaravel.json`, plus a valid `openapi.json` (3.0.3, 7 paths; `POST /posts` requestBody with `title` maxLength 255 as an int, `status` enum, correct required list). Both spike bugs confirmed fixed by my own asserts. PRD #1/#2/#4 fully intact.

> [!success] The engine is feature-complete for the MVP
> One Project Model, six node types, three renderers (ER diagram, route map, OpenAPI) — every output a thin view over one core, exactly as [[ADR 0001 - Project Model as the core abstraction|ADR 0001]] set out.

---

## 2026-06-30 — Milestone 2 slice 2 SHIPPED: Routes + Controllers + symbol table + dead routes

PRD [#4](https://github.com/Mawsis/Un-La-ravel/issues/4) implemented and verified. This is the **ADR 0006 milestone** — the first time the two-phase symbol table exists — and the point where Un(la)ravel "un-ravels a whole app," not just its database.

### What landed
- **`internal/symbol`** — the two-phase symbol table ([[ADR 0006 - Two-phase extraction with symbol table|0006]]). Phase 1 collects declared FQNs + per-file `use` maps; Phase 2 resolves short controller names → FQN **via the naming file's imports** (no by-short-name shortcut — the anti-wrong-edge property). 100% covered.
- **`internal/extract/controller`** — Controller nodes: FQN (namespace + class) + public Actions (excludes `__construct`/private/protected; keeps `__invoke`). 100% covered.
- **`internal/extract/route`** — the hardest extractor: verb routes, **route-group flattening** (inherited prefix + accumulated middleware through nested closures), **apiResource/resource expansion**, legacy `'C@m'` strings, `{param}` preservation, per-route `->middleware()/->name()`. 91.1% covered.
- **`internal/analyze`** (route.go) — resolves Route→Controller@Action and emits **Dead Route** findings (`missing_controller` / `missing_action`), false-positive-safe. 98.8% covered.
- **`internal/render/routemap`** — aligned route-map table with a `⚠ DEAD` marker. 100% covered.
- **`internal/cli`** — two-phase pipeline wired (collect controllers → build symbol table → extract+resolve routes); route map + dead-routes sections printed.
- **`CONTEXT.md`** — added **Dead Route**. `schema_version` bumped 1.1.0 → 1.2.0 (`routes`/`controllers`/`dead_routes` arrays).

### De-risking that paid off (three times running)
The route spike proved the *entire* hard path before agents touched it: group-prefix inheritance (`/admin/users`), apiResource expansion, legacy-string splitting, and `use`-based dead-route resolution — all correct against running code. It also surfaced the AST subtlety that `Route::middleware(...)` is a **static** call (first link of a `Route::x()->y()` chain), not a method call — which later tripped one test assumption; caught and corrected.

### Verified (clean, uncached, -race)
Build + vet + gofmt clean. All tests PASS. Live run: 11 routes with group prefixes flattened, middleware inherited, apiResource expanded, and exactly **one genuine dead route** (`DELETE /admin/users/{id}` → `UserController@destroy`, which only declares `index()`) with **zero false positives** — every live route carries a resolved FQN. PRD #1/#2 fully intact (schema, models, disagreements, ER diagram). Two post-verify coverage gaps (routemap 0%→100%, phpast 48%→93%) closed before commit. Every module now >80%.

---

## 2026-06-30 — Milestone 2 slice 1 SHIPPED: Eloquent Models + relationship-aware ER

PRD [#2](https://github.com/Mawsis/Un-La-ravel/issues/2) implemented and verified. The ER diagram is now a map of the **application's data model**, not just its physical schema — and the tool now catches a class of real bug.

### What landed
- **`internal/model/eloquent.go`** — Model, Relationship, and the new **Disagreement** finding types. `schema_version` bumped to **1.1.0** (contract grew with `models` + `disagreements` arrays).
- **`internal/extract/model`** — Eloquent Model extractor: `hasMany`/`hasOne`/`belongsTo`/`belongsToMany`, `X::class` and string targets, explicit `$table` vs inferred table-name (with English pluralization), non-Model classes ignored.
- **`internal/analyze/disagreement.go`** — light in-memory Model↔Schema correlation (NOT the ADR 0006 symbol table). Flags relationships whose target table or explicit FK column the Schema lacks.
- **`internal/render/er`** — extended to draw Eloquent relationship lines with cardinality + method-name labels, alongside the existing FK lines.
- **`internal/cli`** — wired Models + Disagreements into `analyze`; prints model count and a Disagreements section.
- **`CONTEXT.md`** — added **Relationship** and **Disagreement** (a *finding*, not an error/bug/mismatch).

### De-risking that paid off (again)
The spike caught a **real bug before any agent saw it**: `php-parser` keeps the leading `$` on variable names (`ExprVariable.Name.Value` is `"$this"`, not `"this"`). Comparing to `"this"`/`"table"` without stripping `$` would have made the relationship extractor silently extract nothing. Centralized the fix in `phpast.VariableName`.

### Verified (clean, uncached, -race)
Build + vet + gofmt clean. All tests PASS. Target-module coverage: extract/model **95.0%**, model **92.3%**, analyze **100%**, render/er **85.3%** — all over 80%. Live binary run shows real Eloquent edges (`POSTS }o--|| USERS : "author (belongsTo)"`) and the **false-positive discipline holds**: of Post's four relationships only `Post::editor` is flagged (explicit `editor_id` absent), while the implicit-FK `category` relationship is correctly silent. PRD #1 schema output fully intact (no regression). Known gap: `phpast` coverage dipped to 60.3% (new helpers under-tested) — non-blocking, a follow-up polish item.

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
