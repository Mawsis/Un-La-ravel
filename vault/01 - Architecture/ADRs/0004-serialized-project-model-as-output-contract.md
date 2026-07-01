---
tags: [adr]
status: accepted
date: 2026-06-30
aliases: ["ADR 0004 - Serialized Project Model as output contract"]
---

# ADR 0004 — Serialized Project Model (`unlaravel.json`) is the public output contract

The engine's output is a **documented, versioned JSON serialization of the [[CONTEXT|Project Model]]** (`unlaravel.json`). Every [[CONTEXT|Renderer]] — and the future Laravel package and any web dashboard — consumes this JSON. The in-memory Go structs are an implementation detail; the JSON is the contract.

## Context

We chose "CLI + Laravel package" as the definition of done. The CLI and the package must share one representation of an analyzed app. We also need the renderer layer to be cheaply testable to hit the project's 80% coverage bar. Two shapes were on the table: in-memory Go structs only, or a serialized model as a first-class artifact.

## Decision

The boundary of the **engine** is: produce a valid `unlaravel.json` conforming to a versioned schema. Everything downstream is a **consumer** of that JSON.

- `unlaravel analyze --output model.json` emits the model directly — itself a useful feature (a machine-readable map of any Laravel app).
- Built-in renderers (ER diagram, OpenAPI, Markdown report) read the model; they are written in Go but defined against the JSON shape, not the structs.
- The JSON carries a **`schema_version`** field so consumers can detect incompatibilities. The whole contract uses `snake_case` keys (`schema_version`, `project_name`, `laravel_version`, `schemas`) for internal consistency.

## Consequences

- **Renderers are fixture-testable:** feed a fixture `unlaravel.json` → assert the rendered output. No hand-built Go structs. This is how we reach 80% coverage on the renderer layer cheaply.
- **The Laravel package and any future web dashboard are unlocked** — they consume the same JSON the CLI emits. See [[Roadmap]].
- We take on a **schema-design + versioning** cost. Treated as a feature (demonstrates API/contract maturity), not a tax.
- Clean architectural seam: the *hard* engine and the *easy* renderers are decoupled across a documented boundary.

## Considered and rejected

- **In-memory structs only.** Rejected: renderers would be Go-only and same-process only, every renderer test would construct structs by hand, and the model couldn't be shared with the package. Cheaper today, more expensive at exactly the point our "done" definition needs it.
