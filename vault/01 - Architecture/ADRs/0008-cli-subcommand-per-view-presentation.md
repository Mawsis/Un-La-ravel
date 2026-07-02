---
tags: [adr]
status: accepted
date: 2026-07-02
aliases: ["ADR 0008 - CLI subcommand-per-view presentation"]
---

# ADR 0008 — CLI subcommand-per-view, TTY/pipe/`--json` presentation

The CLI moves from one `analyze` command that dumps everything (colored status,
full Mermaid source, full route map, findings) to one subcommand per view —
`routes`, `models`, `er`, `openapi`, `findings` — plus a compact `analyze`
summary. Each view command adapts its output to how it's being consumed: a
styled table on a TTY, the byte-identical golden-pinned renderer output when
piped, and a JSON projection under `--json`.

## Context

`unlaravel analyze` interleaves decorative status lines with the two
machine-shaped outputs (Mermaid ER source, route map) on stdout. There is no
way to ask a focused question ("just routes", "just findings") or consume
either artifact without grep-ing it out of a larger blob. [[Roadmap]]
Milestone 6 already plans verb-style commands (`why`, `trace`, `doctor`,
`check`, `diff`) consuming the same [[CONTEXT|Project Model]] via a future
query layer — a single do-everything `analyze` doesn't compose with that
direction.

At the same time, `internal/render/er` and `internal/render/routemap` are
golden-pinned (`testdata/fixture-app.golden.{mermaid,routemap}`) — their
output is a **contract**, not a presentation choice. Anything that touches
color, width, or terminal capability must not be allowed near those goldens.

## Decision

1. **One subcommand per view.** `unlaravel routes|models|er|openapi|findings
   [path]`, each printing exactly one artifact. `unlaravel analyze [path]`
   becomes a compact summary (counts + findings digest + a hint pointing at
   the relevant view command); it keeps `--output`/`--openapi` for writing
   the full JSON contract and OpenAPI spec to a file.
2. **Renderers stay contracts; the CLI stays presentation.** `internal/render/*`
   continues to own the one deterministic, golden-pinned string per artifact.
   `internal/cli` decides, per invocation, how to present that string or the
   underlying model — this was already the stated boundary in `CLAUDE.md`
   ("`internal/cli` — cobra commands — presentation only, no analysis logic");
   this ADR makes the *output* side of that boundary explicit.
3. **Three presentation modes per view command, selected by how stdout is
   consumed:**
   - **Piped / redirected (not a TTY):** print the existing renderer output
     verbatim — `er.Render(pm)` for `er`, `routemap.Render(pm)` for `routes`.
     Byte-identical to the golden files. This is the scripting contract:
     `unlaravel routes ./app | grep POST` must keep working exactly as
     `unlaravel analyze` piped output does today.
   - **TTY:** a styled table/summary built in `internal/cli` (via
     charmbracelet/lipgloss) directly from the model fields — colored method
     verbs, dimmed middleware, dead-route emphasis. This output is **never**
     golden-pinned byte-for-byte (color-profile and width vary with the
     terminal); tests pin it only under a forced ASCII profile and fixed
     width, as a smoke check, not a contract.
   - **`--json`:** an explicit projection struct (never a bare `map`, to keep
     determinism) that passes `schema_version` through from the model and
     embeds the relevant slices verbatim, e.g. `routes` → `{"schema_version",
     "routes", "dead_routes"}`. `openapi` has no `--json` flag — its default
     output already is the OpenAPI JSON document.
4. **Streams:** human/status output goes to stdout (styled or plain per the
   above); `--json` output goes to stdout alone (nothing else may write to
   stdout in that mode); diagnostics/logs go to stderr. `NO_COLOR` and
   non-TTY both suppress styling.

## Consequences

- `analyze`'s output shrinks — a breaking change for anyone scripting the old
  full dump. Documented in the ADR, README, and `--help` text as a migration:
  replace `unlaravel analyze` piping with the specific view command.
- New golden tests are needed for the piped and `--json` modes of each view
  command, but the *existing* `testdata/fixture-app.golden.{mermaid,routemap}`
  keep serving as the piped-output contract — no duplication.
- Sets up Milestone 6's verb commands (`why`, `trace`, `doctor`, `check`,
  `diff`) to slot in as more subcommands over the same model, rather than more
  flags on an increasingly overloaded `analyze`.

## Considered and rejected

- **Flags on `analyze`** (`--show routes,er`). Rejected: flags compose worse
  than verbs for scripting (`--show` values are a private mini-language) and
  fight the Milestone 6 verb direction instead of extending it.
- **Always styled (lipgloss) output, keep renderers only for file writes.**
  Rejected: pinning styled output as the piped contract reintroduces exactly
  the nondeterminism (color profile, terminal width) the golden-file
  discipline exists to prevent, and makes the renderer packages dead code
  reachable only from `--output`.
- **Quiet-by-default `analyze` with `--full` restoring today's dump.**
  Rejected: still no way to fetch just routes or just the diagram — doesn't
  address the underlying pain, just hides it behind a flag.

## Note

This supersedes the Roadmap's earlier placeholder numbering that reserved
"ADR 0008" for a future query-layer decision (`unlaravel why`/`trace`). That
future decision will take the next free number when it lands; see
[[Roadmap]].
