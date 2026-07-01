---
tags: [adr]
status: accepted
date: 2026-06-30
aliases: ["ADR 0002 - Six-node MVP scope"]
---

# ADR 0002 — Six-node MVP scope

The MVP [[CONTEXT|Project Model]] has **exactly six node types** — Schema, Model, Route, Controller, Middleware, FormRequest — populated by six extractors. **API Resources and N+1/perf analysis are explicitly deferred** to the [[Roadmap]].

## Context

Static analysis of PHP *without booting Laravel* ([[ADR 0003 - Static analysis without booting Laravel]]) ranges from easy (migrations are regular and line-parseable) to genuinely hard (Resource `toArray()` shapes and N+1 detection need real expression/data-flow analysis). The hard tier is where portfolio projects stall: months of work, fragile output, and false positives that make the tool look *wrong* — worse than incomplete.

## Decision

Include the six node types whose extraction is **easy-to-medium** and which together unlock three impressive renderers at once:

| Node | Source | Unlocks |
|---|---|---|
| Schema | `database/migrations/*.php` | ER diagram |
| Model (+relationships) | `app/Models/*.php` | ER diagram |
| Route | `routes/*.php` | Route map, OpenAPI |
| Controller / Action | `app/Http/Controllers` | Route map, OpenAPI |
| Middleware | `Kernel.php` + route groups | Route map |
| FormRequest | `app/Http/Requests` | OpenAPI request bodies |

**Deferred:** API **Resource** (response-body shapes) and **Hotspot** (N+1/perf) — both require real PHP expression/data-flow analysis. See [[CONTEXT]] and [[Roadmap]].

## Consequences

- The MVP can legitimately demo routes, database, middleware, and Swagger — the "everything" *feel* — without entering the high-risk tier.
- A false-positive-prone feature (N+1) is kept out of v1, protecting the tool's credibility.
- Extraction order follows dependency: **Schema → Model → Route → Controller → Middleware → FormRequest**.
