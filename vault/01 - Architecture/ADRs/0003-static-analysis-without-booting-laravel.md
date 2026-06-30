---
tags: [adr]
status: accepted
date: 2026-06-30
aliases: ["ADR 0003 - Static analysis without booting Laravel"]
---

# ADR 0003 — Static analysis via PHP AST, without booting Laravel

The engine reads Laravel source by building a **real PHP AST** with `github.com/VKCOM/php-parser` and walking it with the visitor pattern. It **never boots Laravel** (no `php artisan`, no `vendor/`, no `.env`, no DB). The tool stays a single static Go binary that runs on any Laravel project, including broken or legacy ones.

## Context

Laravel's routes, relationships, and validation rules live *in PHP code* (`Route::get(...)`, `$this->hasMany(...)`, `rules()` arrays). A Go binary must extract them. Four strategies were considered (see below). The two failure modes to avoid: (1) a regex hack that demos on a toy app and shatters on a real one — which a reviewer *will* test — and (2) a tool that requires a bootable app, failing on exactly the legacy projects people most want to un-ravel.

## Decision

**Option B: real PHP AST, pure Go, fully static.**

- Parser: **`github.com/VKCOM/php-parser`** (verified via the Go module proxy; latest **v0.8.2**, Jun 2022). This is the maintained successor to the deprecated `z7zmey/php-parser`.
- Extractors walk the AST with visitors and resolve nodes into the [[CONTEXT|Project Model]].
- Single static binary; no runtime PHP dependency.

> [!warning] Corrects a `CLAUDE.md` error
> The original `CLAUDE.md` referenced `github.com/go-ast-php/php-parser-go` as the PHP parser. That package **does not exist** (HTTP 404 on the Go module proxy). `CLAUDE.md` must be updated to reference `VKCOM/php-parser`.

## Considered and rejected

- **A — Regex / line-based parsing.** Rejected: breaks on multi-line routes, groups, `apiResource` macros, namespaces, heredocs. Impressive on a toy, worthless on a real app. Not a portfolio-grade analyzer.
- **C — Shell out to `php artisan route:list`.** Rejected for the core: requires PHP + installed `vendor/` + a bootable app (valid `.env`, reachable DB). Defeats the static premise and fails on legacy/broken projects. Also not *our* engine — it scrapes Laravel's output.
- **D — Hybrid (AST default + optional artisan enrichment).** Deferred to [[Roadmap]] as an opt-in cross-check, not part of the static core.

## Consequences

- The "I built a real static analyzer" portfolio story (AST visitors, node resolution) is the centrepiece — and the extra effort *is* the value, fitting the Go-learning goal.
- **Known limitation:** VKCOM v0.8.2 parses up to ~PHP 8.1 syntax. PHP 8.2+ constructs (readonly classes, DNF types) may not parse. Acceptable for the vast majority of Laravel apps; documented in [[Roadmap]] as a watch-item. If it becomes a real blocker, re-evaluate the parser or contribute upstream.
- Every [[CONTEXT|Extractor]] depends on this choice; swapping the parser later is a meaningful cost — hence this ADR.
