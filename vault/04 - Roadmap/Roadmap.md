---
tags: [roadmap, moc]
aliases: [Roadmap]
---

# Roadmap

How Un(la)ravel gets from "~100 lines that don't compile" to "portfolio-grade CLI + Laravel package." Sequencing is **vertical-slice-first**: prove the whole pipeline on one node type before widening.

See decisions: [[ADR Index]]. See language: [[CONTEXT]].

## Build philosophy: vertical slices, not horizontal layers

> [!important] Why vertical
> Building all six extractors before any renderer leaves the binary half-fake for weeks. Instead, each milestone takes **one node type all the way through** the pipeline — `main.go → analyze → extractor → Project Model → unlaravel.json → renderer → output` — so the binary builds and does something real from Milestone 1.

## Target package layout

```
cmd/unlaravel/main.go     # entrypoint (MISSING TODAY — Milestone 0)
internal/
  cli/                    # cobra commands               (exists)
  detector/               # composer + laravel detection  (exists, working)
  model/                  # Project Model types, schemaVersion, JSON  (ADR 0004)
  extract/                # one sub-package per node type: schema, model, route, controller, middleware, formrequest
  render/                 # one sub-package per renderer: er, openapi, report
testdata/                 # fixture Laravel snippets + golden unlaravel.json + golden outputs
```

## Milestones

### ✅ Milestone 0 — Make it build & stop lying _(unblocks everything)_
- [ ] Add `cmd/unlaravel/main.go` calling `cli.Execute()` (currently missing — the repo does not build into a runnable binary).
- [ ] Delete the fake hardcoded `✅` lines in `analyzeProject` ([[Engineering Journal|see journal: starting state]]).
- [ ] Wire `analyze` to actually call `detector.DetectLaravel`.
- [ ] Fix `CLAUDE.md`: replace fictional `go-ast-php/php-parser-go` with `github.com/VKCOM/php-parser` ([[ADR 0003 - Static analysis without booting Laravel|ADR 0003]]).

### 🎯 Milestone 1 — Vertical slice: migrations → ER diagram
- [ ] `internal/model`: define core types + `schemaVersion`, JSON tags ([[ADR 0004 - Serialized Project Model as output contract|ADR 0004]]).
- [ ] `internal/extract/schema`: parse `database/migrations/*.php` via PHP AST → Schema nodes.
- [ ] `internal/render/er`: Schema → Mermaid ER diagram.
- [ ] `unlaravel analyze --output model.json` emits valid `unlaravel.json`.
- [ ] Golden-file tests with a fixture Laravel app in `testdata/`.
- [ ] **Deliverable:** a real Mermaid ER diagram from real migrations — first README screenshot.

### ✅ Milestone 2 — Widen the model (remaining extractors)
The six-node MVP node set is **COMPLETE** — every extractor slots into the proven pipeline, in dependency order:
- [x] `extract/model` — Eloquent models + relationships (`hasMany`/`belongsTo`/…). Feeds richer ER edges.
- [x] `extract/route` — `routes/*.php`, incl. groups, middleware stacks, and `apiResource` macro expansion.
- [x] `extract/controller` — map Route → Controller Action (+ action param type-hints for the FormRequest link).
- [x] `extract/middleware` — aliases from HTTP Kernel + application on routes/groups.
- [x] `extract/formrequest` — `rules()` arrays for request bodies, linked to routes via the ADR 0006 symbol table.

> [!success] Six nodes, all the way through
> Schema, Model, Route, Controller, Middleware, and FormRequest all extract → model → `unlaravel.json` (schema `1.3.0`).

### Milestone 3 — The other two renderers
- [x] `render/openapi` — OpenAPI 3 spec from Route + Controller + FormRequest, via `analyze --openapi <path>`. Pairs with Swagger UI for a demo.
- [ ] `render/report` — Markdown architecture report (routes, middleware, models, schema).

### ✅ Milestone 3.5 — Interactive web dashboard (`unlaravel serve`)
Originally listed under Deferred as out of scope for the "CLI + package" done-definition — built anyway as a local dev tool once the engine was extracted from the CLI, since it's just another consumer of the same Project Model (ADR 0001/0004).
- [x] `internal/engine` — `Analyze(path) (*model.ProjectModel, error)`, the pipeline extracted out of the CLI so both `analyze` and `serve` share one entry point.
- [x] `internal/web` — `net/http` + JSON API (`/api/analyze`, `/api/er`, `/api/openapi`), localhost-only, embedded single-page dashboard.
- [x] ER diagram (Mermaid, pan/zoom via svg-pan-zoom for large schemas), filterable route table with dead-route highlighting, Swagger UI over the generated OpenAPI spec, findings panel.
- [x] All dashboard JS assets (Mermaid, svg-pan-zoom, Swagger UI) vendored into the binary via `go:embed` — the dashboard works fully offline.

### Milestone 4 — Polish for portfolio
- [ ] Killer README: GIF demo, generated-output samples, install instructions.
- [ ] `go install` / Homebrew tap / GoReleaser for binaries.
- [ ] CI (build + test + coverage badge), 80%+ coverage.

### Milestone 5 — Laravel package ([[ADR 0005 - Laravel package is thin wrapper over Go binary|ADR 0005]])
- [ ] Composer package + service provider registering `php artisan unlaravel:analyze`.
- [ ] Binary distribution via post-install download hook (platform-specific).
- [ ] Publish to Packagist.

### Milestone 6 — From MVP to professional toolkit
Four flagship capabilities, all consuming the existing Project Model — no re-architecture, per ADR 0001. Each is its own vertical slice/PR.
- [ ] Engine deepening I: `$fillable`/`$guarded`/`$casts` on Model, `indexes` on Table → contract `1.4.0`.
- [ ] Shared query layer (`internal/query`) + `unlaravel why` / `unlaravel trace` → contract `1.5.0` (route-model-binding edges). New ADR 0008.
- [ ] Findings model (`internal/findings`) + `unlaravel doctor` + middleware alias extraction → contract `1.6.0`. New ADR 0009.
- [ ] AI integration: `unlaravel mcp` (official `modelcontextprotocol/go-sdk`, read-only tools wrapping the query layer) + `unlaravel context`. New ADR 0010.
- [ ] CI guardrails: `unlaravel check` (exit-code gate) + `unlaravel diff` (structural changelog between two `unlaravel.json` snapshots).
- [ ] _(optional, cuttable)_ Through/polymorphic relationship kinds → contract `1.7.0`.
- [ ] Distribution: GoReleaser + Homebrew tap, then the Laravel Composer package in a separate repo. New ADR 0011.

## Deferred (explicitly out of MVP)

| Item | Why deferred | Becomes |
|---|---|---|
| **API Resource** node ([[CONTEXT]]) | `toArray()` shapes need real PHP expression analysis | A future Extractor → richer OpenAPI responses |
| **N+1 / Hotspot** analysis | Static detection has high false-positive risk that makes the tool look *wrong* | A future Extractor + report section, once precision is provable |
| **Artisan enrichment** (Option D, [[ADR 0003 - Static analysis without booting Laravel|ADR 0003]]) | Optional cross-check when app is bootable | Opt-in flag, never required |

## Watch-items / known risks

- **Parser PHP-version ceiling:** `VKCOM/php-parser` v0.8.2 (Jun 2022) parses up to ~PHP 8.1. PHP 8.2+ syntax (readonly classes, DNF types) may fail to parse. Re-evaluate if real apps trip it. ([[ADR 0003 - Static analysis without booting Laravel|ADR 0003]])
- **`apiResource` macro expansion:** routes are not 1:1 with declarations — must expand to index/store/show/update/destroy. ([[CONTEXT]] example dialogue)
- **Model ↔ Schema disagreement** is a *feature* to surface, not an error to hide. ([[CONTEXT]])
