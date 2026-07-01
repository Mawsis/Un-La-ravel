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

### Milestone 2 — Widen the model (remaining extractors)
Each slots into the proven pipeline, in dependency order:
- [ ] `extract/model` — Eloquent models + relationships (`hasMany`/`belongsTo`/…). Feeds richer ER edges.
- [ ] `extract/route` — `routes/*.php`, incl. groups, middleware stacks, and `apiResource` macro expansion.
- [ ] `extract/controller` — map Route → Controller Action.
- [ ] `extract/middleware` — aliases from HTTP Kernel + application on routes/groups.
- [ ] `extract/formrequest` — `rules()` arrays for request bodies.

### Milestone 3 — The other two renderers
- [ ] `render/report` — Markdown architecture report (routes, middleware, models, schema).
- [ ] `render/openapi` — OpenAPI 3 spec from Route + Controller + FormRequest. Pairs with Swagger UI for a demo.

### Milestone 4 — Polish for portfolio
- [ ] Killer README: GIF demo, generated-output samples, install instructions.
- [ ] `go install` / Homebrew tap / GoReleaser for binaries.
- [ ] CI (build + test + coverage badge), 80%+ coverage.

### Milestone 5 — Laravel package ([[ADR 0005 - Laravel package is thin wrapper over Go binary|ADR 0005]])
- [ ] Composer package + service provider registering `php artisan unlaravel:analyze`.
- [ ] Binary distribution via post-install download hook (platform-specific).
- [ ] Publish to Packagist.

## Deferred (explicitly out of MVP)

| Item | Why deferred | Becomes |
|---|---|---|
| **API Resource** node ([[CONTEXT]]) | `toArray()` shapes need real PHP expression analysis | A future Extractor → richer OpenAPI responses |
| **N+1 / Hotspot** analysis | Static detection has high false-positive risk that makes the tool look *wrong* | A future Extractor + report section, once precision is provable |
| **Artisan enrichment** (Option D, [[ADR 0003 - Static analysis without booting Laravel|ADR 0003]]) | Optional cross-check when app is bootable | Opt-in flag, never required |
| **Web dashboard** | Out of scope for "CLI + package" done-definition | Consumes the same `unlaravel.json` ([[ADR 0004 - Serialized Project Model as output contract|ADR 0004]]) |

## Watch-items / known risks

- **Parser PHP-version ceiling:** `VKCOM/php-parser` v0.8.2 (Jun 2022) parses up to ~PHP 8.1. PHP 8.2+ syntax (readonly classes, DNF types) may fail to parse. Re-evaluate if real apps trip it. ([[ADR 0003 - Static analysis without booting Laravel|ADR 0003]])
- **`apiResource` macro expansion:** routes are not 1:1 with declarations — must expand to index/store/show/update/destroy. ([[CONTEXT]] example dialogue)
- **Model ↔ Schema disagreement** is a *feature* to surface, not an error to hide. ([[CONTEXT]])
