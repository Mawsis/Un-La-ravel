---
tags: [adr]
status: accepted
date: 2026-07-03
aliases: ["ADR 0008 - Findings model and doctor"]
---

# ADR 0008 — The health verdict is a server-computed contract field, not a browser computation

The project's **health verdict** — an itemized list of problem categories (dead
routes, model↔schema disagreements, unguarded models) — is computed **once,
server-side**, into the [[CONTEXT|Project Model]] as a `findings` array
([[ADR 0004 - Serialized Project Model as output contract|ADR 0004]], contract
`1.6.0`). The CLI's `unlaravel doctor` command, the `unlaravel.json` output, and
the `serve` dashboard all **read** that one array; none of them re-derive it.

`doctor` exits **non-zero** when any finding is present, so it doubles as a CI
gate.

## Context

The health verdict first shipped in issue #27 as a **browser-only** computation:
`internal/web/assets/js/verdict.js` re-derived the verdict in JavaScript from the
raw model (counting `dead_routes`, `disagreements`, and models with an explicit
empty `$guarded`). That left three problems:

- **The CLI could not state a verdict.** `analyze` prints per-category lines, but
  there was no single "is this project healthy?" answer and no command to ask.
- **The JSON contract did not carry the verdict.** Any consumer (CI, another
  tool) had to re-implement the same three rules against `unlaravel.json`.
- **The rules existed in two places by intent** — the JS today, anything
  server-side tomorrow — and could silently drift apart.

The Go Model already anticipated the fix: the `Guarded` field's doc comment
(`internal/model/eloquent.go`) notes that a non-nil empty `$guarded = []` is what
*"a future doctor/lint rule"* keys on.

## Decision

- A new pure package **`internal/findings`** computes `[]model.Finding` from an
  assembled Project Model (like `internal/analyze`, it spans several node types
  no single extractor owns). `engine.Analyze` runs it **last**, over the fully
  assembled model, and appends the result to `ProjectModel.Findings`.
- `Finding` is a first-class contract type (`kind`, `count`, `label`, `view`),
  serialized as the `findings` array. The contract bumps to **`1.6.0`**
  (additive: the array is appended last).
- **`verdict.js` becomes a thin reader** of `model.findings`. The browser no
  longer recomputes; it surfaces what the server sent.
- **`unlaravel doctor`** is the CLI face: it runs the same `engine.Analyze`,
  prints the itemized verdict, and **exits non-zero on any finding**. `analyze`
  stays always-exit-0 (it reports; `doctor` judges).

## Consequences

- **One source of truth.** CLI, JSON, and dashboard agree by construction — a
  drift between them is now impossible, not merely unlikely.
- **CI-ready today.** `doctor`'s non-zero exit is the gate the roadmap's later
  `unlaravel check` bullet asked for, folded into the command that already
  computes the verdict.
- **The load-bearing nil-vs-empty `$guarded` rule is codified** in Go and covered
  by tests, exactly where the Model's doc comment said it belonged.
- The roadmap's `1.6.0` line also bundles **middleware alias extraction**; that is
  a separate vertical slice and is **not** part of this decision or PR.

## Considered and rejected

- **Keep the verdict in the browser; add a Go helper the CLI also calls.** No
  contract bump, but the logic still lives twice (Go + JS) and can drift — it
  fails the "in the contract, one source of truth" goal that motivated the work.
- **Make `doctor` always exit 0 and add a separate `check` command for gating.**
  Rejected as premature: `doctor` already computes the verdict, so gating on it is
  free; a distinct `check` can still be added later if its scope diverges.
