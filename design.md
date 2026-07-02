# Un(la)ravel — Design System

This file is the design system for both user-facing surfaces: the CLI and the
`unlaravel serve` web dashboard. It documents *how things should look and
behave*, in the same way `CLAUDE.md` documents how the engine should be built.
The *why* behind the redesign that produced this document is
[[ADR 0008 - CLI subcommand-per-view presentation|ADR 0008]] in the vault;
this file is the *system* that decision implies.

Domain terms used below (Node, Renderer, Disagreement, Dead Route, …) follow
[`vault/02 - Domain/CONTEXT.md`](vault/02%20-%20Domain/CONTEXT.md) — use them
consistently in UI copy.

## Principles

1. **Every surface is a thin view over the Project Model.** The CLI and the
   dashboard never invent data — they present what `engine.Analyze` already
   returned. A UI change is never a reason to add a field to the model; if a
   view needs something the model doesn't have, that's a model change first,
   discussed on its own terms.
2. **Answer one question at a time.** A user opens Un(la)ravel wanting to know
   *one* thing — "what are the dead routes," "what does this schema look
   like." Every surface should let them get straight to that answer, not wade
   through everything to find it.
3. **Adapt to how you're being consumed, don't ask the user to declare it.**
   A TTY gets a styled table; a pipe gets plain deterministic text; `--json`
   gets a clean projection. The dashboard remembers state in the URL, not in
   a mode the user has to re-select.
4. **A finding is a fact, not an alarm.** Disagreements and Dead Routes are
   reported calmly and specifically (what, where, why) — never with
   panic-red flooding or vague "something's wrong" language. See the domain
   glossary: these are *findings*, not errors.
5. **Static means honest.** Never imply certainty the static analysis doesn't
   have. Copy says "not found," "could not resolve," "no Migration creates
   this" — not "broken" or "invalid."

---

## Part 1 — CLI

### Command grammar

```
unlaravel <view> [path] [--json]
```

| Command | Prints | `--json` shape |
|---|---|---|
| `unlaravel analyze [path]` | Compact summary: project name, Laravel version, six Node counts, one-line findings digest, a hint pointing at the relevant view command. `--output`/`--openapi` still write the full contract/spec to a file. | _(none — use `--output`)_ |
| `unlaravel routes [path]` | Route table (Method / URI / Controller@Action / Middleware), Dead Routes marked | `{"schema_version", "routes", "dead_routes"}` |
| `unlaravel models [path]` | Model cards: mass-assignment state, casts, indexes, orphan tables | `{"schema_version", "models"}` |
| `unlaravel er [path]` | Mermaid ER diagram source | `{"schema_version", "mermaid"}` |
| `unlaravel findings [path]` | Disagreements + Dead Routes, or "All clear" | `{"schema_version", "disagreements", "dead_routes"}` |
| `unlaravel openapi [path]` | The OpenAPI 3 document | _(this command's default output already is JSON — no `--json` flag)_ |
| `unlaravel serve [path]` | Starts the dashboard; optional `path` pre-analyzes it on load | _(n/a)_ |

`path` defaults to `.` everywhere it appears, matching today's `analyze`.

Migration note: `unlaravel analyze`'s output is deliberately smaller than
before. Anyone who scripted against the old full dump should switch to the
specific view command (`unlaravel er ./app` instead of grepping the diagram
out of `analyze`'s output). Piped output of each view command is
byte-identical to what the corresponding renderer always produced, so
scripts built on the *renderer* output (not on `analyze`'s decoration) are
unaffected.

### Presentation modes

Every view command picks its mode from how stdout is being consumed — the
user never has to ask for a mode:

| Mode | Trigger | Behavior |
|---|---|---|
| **Styled** | stdout is a TTY | lipgloss table/summary: colored HTTP methods, dimmed middleware, emphasized findings. Never byte-pinned — varies with terminal width/color profile. |
| **Plain** | stdout is piped/redirected | The existing `internal/render/*` output, verbatim. This is the scripting contract — byte-identical to the golden files (`testdata/fixture-app.golden.{mermaid,routemap}`). |
| **JSON** | `--json` flag | A projection struct (see table above), `json.MarshalIndent`, nothing else on stdout. |

`NO_COLOR` and non-TTY both suppress styling, same rule, no separate flag
needed. Styling never changes *what* is printed, only how — the same route
appears in all three modes, just formatted differently.

### Streams

- **stdout**: the view's output, in whichever mode above applies. In
  `--json` mode, stdout carries the JSON and nothing else — scripts can pipe
  it straight into `jq`.
- **stderr**: diagnostics only — the `serve` "listening on" line, request
  logs, verbose (`-v`) detail. Never anything a script would want to parse.
- **Exit codes**: `0` success, `1` any error (not-a-Laravel-project, engine
  failure, write failure). Findings (Disagreements, Dead Routes) do **not**
  affect the exit code — they're informational until a future `unlaravel
  check` gate is built (Roadmap Milestone 6) with its own explicit flag.

### Visual language (styled mode)

- Library: [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss).
  `fatih/color` and its emoji-heavy `✅`/`⚠️`/`🔍` style are retired.
- Palette (mirrors the web tokens below, ANSI-mapped): cyan/`accent` for
  headers and hints, green for healthy counts and "all clear," amber for
  findings, red reserved for Dead Routes and error text, dim gray for
  secondary detail (middleware, timestamps).
- No emoji. A short unicode glyph set is fine for status (`✓`, `!`, `→`) but
  used sparingly, once per line at most, never as the whole message.
- Tables: aligned columns, method verbs bold+colored, one row per item, a
  trailing summary line ("41 routes, 2 dead").
- `analyze`'s summary is a single small block, not a report — six counts, one
  findings line, one hint line. It should fit in a terminal without
  scrolling.

### Testing discipline

Styled output is a smoke test, not a contract: pin goldens only under a
forced ASCII color profile (`termenv.Ascii`) and a fixed terminal width.
Plain (piped) output reuses the *existing* renderer goldens — no duplication.
`--json` output gets its own small goldens per view, generated the same way
existing goldens are (`-update` flag, reviewed diff, never blind regen).

---

## Part 2 — Web dashboard (`unlaravel serve`)

### Architecture constraints (unchanged, load-bearing)

- **No build step.** Vanilla JS, `go:embed`, works fully offline. This
  redesign restructures the *files*, not the *architecture* — see
  `internal/web/assets/` layout below.
- **Single Project Model per view.** The dashboard fetches `/api/analyze`
  once per project and renders every view from that one in-memory object —
  no additional round-trips per tab/view.
- Dark-only for now (see Deliberate trades).

### Information architecture

Five tabs become five routed views behind a persistent sidebar, plus an
overview/entry view:

```
Overview   — entry point: path input, recent projects, project stat cards
ER Diagram — Mermaid, pan/zoom, focusable by table name
Models     — cards: mass-assignment, casts, indexes, orphan tables
Routes     — table: method/URI/controller/action/middleware, Dead Route rows
API Docs   — Swagger UI over the generated OpenAPI spec
Findings   — Disagreements + Dead Routes, or "All clear"
```

The sidebar is real navigation — `<nav aria-label="Views"><a href="#/routes">`
— not JS-only tab-switching. Every view is reachable, refreshable, and
linkable on its own.

### URL & state

```
#/<view>?path=<abs-path>&filter=<text>&sort=<field>&table=<name>
```

- The **URL is the reproducible view**: which project, which view, which
  filter/sort/focus. Refresh, back/forward, and copy-paste-share all just
  work because the state that matters lives there.
- **localStorage is convenience only**: a recent-projects list (path + name +
  last-used, capped at 8) and the last-active project, offered on the
  Overview view. Correctness of any view never depends on localStorage being
  present.
- Typing in a filter box updates the URL via a debounced `replaceState`
  (doesn't spam history); switching views or focusing an entity `pushState`s
  (so Back steps through them meaningfully).
- `unlaravel serve [path]` accepts an optional path argument that
  pre-analyzes on load, for the common case of "I'm already in the project
  directory."

### Cross-navigation (linked explorer)

The core fix for tab-silo pain: **every mention of an entity anywhere is a
link to its canonical view**, using one central link builder so the mapping
is defined once:

- A Route's Controller cell → filters the Routes view (or, later, a
  Controller detail) to that controller.
- A Model card's table name → focuses that table in the ER Diagram.
- A Disagreement or Dead Route finding → links to the Model/Route it's about.
- The ER Diagram's hint text → links back to Models.

"Focusing" a table in the ER diagram means centering the pan/zoom viewport on
it and briefly highlighting its box — implemented by matching the rendered
entity label text (not by depending on Mermaid's internal id/uuid scheme,
which is treated as vendored-library-internal and pinned to the currently
vendored Mermaid version in a code comment).

### Search

A command palette (Cmd/Ctrl+K, plus a visible sidebar button — never
shortcut-only) indexes the current Project Model client-side: routes (by
method/URI/controller), models (by name/table), tables (by name/columns),
findings (by subject). Substring match, deterministic ordering, Enter
navigates via the same link builder cross-navigation uses.

### Visual design

**Design tokens** (CSS custom properties, `css/tokens.css`) — evolved from
the current palette, not replaced:

```css
--bg: #0d1117;        --bg-elev: #161b22;      --bg-elev2: #1c2333;
--border: #2a3140;    --text: #e6edf3;         --text-dim: #9aa7b4;
--accent: #4dd0e1;    --accent-2: #7c9cff;
--green: #3fb950;     --red: #f85149;          --amber: #d29922;
--mono: ui-monospace, "SF Mono", "JetBrains Mono", Menlo, Consolas, monospace;
--sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
```

Promoted alongside the existing colors, as part of this redesign:

- **Spacing scale**: 4/8/12/16/20/24/32/48px — replace ad hoc padding values
  with these across `css/*.css`.
- **Type scale**: 11/12/13/14/16/20px, matching sizes already in use today;
  codified so new components don't invent new ones.
- **Radius**: 5px (chips/pills), 8px (inputs/buttons), 10px (cards/panels) —
  already the de facto values, now explicit.

`--text-dim` on `--bg` (`#9aa7b4` on `#0d1117`) passes WCAG AA for normal
text — keep any new secondary-text color at or above that contrast.

### Component inventory

Reuse these rather than inventing new patterns: stat card, pill
(ok/warn/danger/dim), chip (mono, for columns/casts), finding card
(amber-left-border, red for Dead Routes), table row states (default/dead),
empty-state box, error box, loading spinner, sidebar nav item, command
palette result row.

### File layout

```
internal/web/assets/
  index.html              slim shell: skip-link, <nav> sidebar, <main>
  css/
    tokens.css             design tokens (above)
    base.css                reset, typography, focus-visible, reduced-motion
    layout.css               sidebar + topbar + main grid
    components.css            cards, pills, chips, tables, findings, model cards
  js/
    main.js                  bootstrap (<script type="module">)
    router.js                 hash parsing/navigation, per URL & state above
    store.js                    localStorage recents (convenience only)
    api.js                        fetch wrapper
    state.js                       analysis result holder + change events
    dom.js                          escapeHtml + small element helpers
    links.js                         central hrefFor({kind, ...}) builder
    search.js                         Cmd+K palette
    views/
      overview.js  er.js  models.js  routes.js  swagger.js  findings.js
  vendor/                   unchanged — mermaid, svg-pan-zoom, swagger-ui, still
                             classic <script> globals loaded before js/main.js
```

`go:embed assets` already embeds recursively; no Go changes are needed for
the file split beyond a test asserting every asset `index.html` references
actually exists in the embedded filesystem.

### Accessibility baseline

Real, minimum, verifiable by hand — not a checkbox exercise:

- Sidebar links are real `<a>` elements with `aria-current="page"` on the
  active view — native keyboard and screen-reader support, no custom
  ARIA-tabs choreography.
- Skip-link to `<main>`; on view change, focus moves to that view's
  `<h2 tabindex="-1">` and `document.title` updates (e.g. "Routes —
  acme/blog — Un(la)ravel").
- One `aria-live="polite"` status region announces "Analyzing…", "Analyzed
  41 routes", and errors — replacing today's silent spinner swap.
- Tables use `<th scope="col">`; sortable headers are `<button>` elements
  inside the `<th>` with `aria-sort`.
- The command palette is `role="dialog" aria-modal="true"` with a focus trap;
  Esc restores focus to whatever opened it; the input behaves as a combobox
  over a `role="listbox"` results list.
- `:focus-visible` gets a visible ring token everywhere; any animation
  (spinner, ER highlight-on-focus) respects `prefers-reduced-motion`.

**Explicitly out of scope** (state this rather than pretend otherwise): a
screen-reader-accessible reading of the ER diagram's SVG beyond a `<title>`/
`<desc>` (the Models and Routes views serve as the accessible equivalent of
that data), Swagger UI's internal accessibility (it's a vendored third-party
app inside a framed panel), and a full WCAG audit.

### Deliberate trades

- **Swagger UI stays a light panel inside the dark shell**, not
  dark-themed. A faithful dark override of `swagger-ui.css` is roughly a
  thousand lines of community CSS with hardcoded syntax-highlighter colors —
  a large vendored surface to maintain for a tab that doesn't touch any of
  the four usability pains this redesign targets. The panel gets
  `color-scheme: light`, token-consistent border/radius/padding, and a short
  caption noting it renders in light mode. Revisit only if it becomes an
  actual complaint.
- **Dark-only, no light theme / no toggle.** The redesign's driver is
  usability (navigation, state, cross-linking), not visual identity or
  daylight/projector use cases. Revisit if that need shows up.
- **No incremental/streaming re-render.** Each `serve` analysis still fetches
  the full model in one request and re-renders views from it; this redesign
  changes navigation and presentation, not the fetch/render lifecycle.
