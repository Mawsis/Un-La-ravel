---
tags: [adr, moc, index]
aliases: [ADR Index]
---

# ADR Index

Architecture Decision Records — *why* the project is shaped the way it is. Each records a decision that is **hard to reverse**, **surprising without context**, and **the result of a real trade-off**. Captured during the grilling session of [[Engineering Journal|2026-06-30]].

| # | Decision | Status | The one-line "why" |
|---|---|---|---|
| [[ADR 0001 - Project Model as the core abstraction\|0001]] | Project Model as the core abstraction | ✅ accepted | One engine + many renderers beats 30 parallel features |
| [[ADR 0002 - Six-node MVP scope\|0002]] | Six-node MVP scope | ✅ accepted | Ship the easy-to-medium nodes; defer the false-positive-prone ones |
| [[ADR 0003 - Static analysis without booting Laravel\|0003]] | Static analysis via PHP AST | ✅ accepted | Real AST (VKCOM/php-parser), never boot Laravel; runs on any project |
| [[ADR 0004 - Serialized Project Model as output contract\|0004]] | `unlaravel.json` is the output contract | ✅ accepted | A versioned JSON seam unlocks the package, testing, and a future web UI |
| [[ADR 0005 - Laravel package is thin wrapper over Go binary\|0005]] | Laravel package = thin wrapper | ✅ accepted | Composer package shells out to the one Go engine; PHP stays minimal |
| [[ADR 0006 - Two-phase extraction with symbol table\|0006]] | Two-phase extraction + symbol table | ✅ accepted | Correct cross-file edges; dangling refs = free dead-route detection |
| [[ADR 0007 - No database in core\|0007]] | No database in the core | ✅ accepted | In-memory graph + JSON; SQLite/GORM is premature infra (overrides brief) |
| [[ADR 0008 - Findings model and doctor\|0008]] | Health verdict is a server-computed contract field | ✅ accepted | One `findings` array computed once server-side; CLI/JSON/dashboard agree by construction; `doctor` gates CI |
| [[ADR 0009 - ER settle animation and diagram export\|0009]] | ER settle animation + SVG/PNG export | ✅ accepted | A once-per-session settle is the un-ravel metaphor, not decoration; a narrow recorded exception to #25's no-settle rule, done as a token-timed transition so the motion tests don't loosen |
| [[ADR 0010 - Design system and the thread metaphor\|0010]] | Design system: thread roles, warmed OKLCH, modular scale | ✅ accepted | PRODUCT.md/DESIGN.md become the committed intention; red/cyan turn into `unresolved`/`resolved` state roles, neutrals warm off the Tailwind ramp, and six grep-able anti-references gate every visual PR |

## How these connect

```mermaid
flowchart TD
    a1["0001 Project Model core"] --> a2["0002 Six-node scope"]
    a1 --> a3["0003 Static AST"]
    a1 --> a4["0004 JSON contract"]
    a4 --> a5["0005 Thin Laravel package"]
    a3 --> a6["0006 Two-phase + symbol table"]
    a4 --> a7["0007 No database"]
    a5 -.reinforced by.-> a7
    a1 --> a8["0008 Findings + doctor"]
    a4 --> a8
```

## Decisions that did NOT get an ADR (and why)

- **Vertical-slice-first build order / Milestone 1 = migrations→ER** — a sequencing choice, easily reversed, not surprising. Lives in [[Roadmap]].
- **Package layout (`cmd/`, `internal/...`)** — standard Go convention, no trade-off. Lives in [[Architecture Overview]].
- **Exact JSON fields, Mermaid syntax, CLI flag names** — implementation details, cheap to reverse. Decided in code.
