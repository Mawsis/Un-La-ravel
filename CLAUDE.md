# Un(la)ravel — Laravel Project Analysis Tool

Un(la)ravel statically analyzes a Laravel project — no booting, no `artisan`,
no database connection, no `vendor/` required — and builds a single **Project
Model** of its schema, Eloquent models, routes, controllers, and form
requests. Every output (ER diagram, OpenAPI spec, route map, the interactive
dashboard) is a thin renderer over that one model.

For the *why* behind these decisions, see the Obsidian vault at `vault/`:
[Architecture Overview](vault/01%20-%20Architecture/Architecture%20Overview.md),
the [ADR Index](vault/01%20-%20Architecture/ADRs/ADR%20Index.md), the domain
[glossary](vault/02%20-%20Domain/CONTEXT.md), and the
[Roadmap](vault/04%20-%20Roadmap/Roadmap.md). This file is the *what* and the
conventions a change must respect.

## Architecture

```
Laravel source ──▶ PHP AST ──▶ Extractors ──▶ Project Model ──▶ unlaravel.json
  (read-only)    (VKCOM/      (one per node)  (in-memory         (versioned
                 php-parser)                   graph)             contract)
                                                     │
                                                     ├──▶ Mermaid ER diagram
                                                     ├──▶ route map (+ dead routes)
                                                     ├──▶ OpenAPI 3 spec
                                                     └──▶ unlaravel serve (web dashboard)
```

```
cmd/unlaravel/      entrypoint
internal/
  cli/              cobra commands — presentation only, no analysis logic
  engine/           Analyze(path) → *model.ProjectModel; the one pipeline entry point
  detector/         is-this-a-Laravel-project + composer.json parsing
  phpast/           the ONLY package that imports the PHP parser (isolation layer)
  symbol/           two-phase symbol table for cross-file reference resolution
  extract/          one sub-package per node type (schema, model, route, controller, formrequest)
  analyze/          correlation passes: disagreements, route resolution, form-request linking
  model/            the Project Model types + versioned JSON contract (unlaravel.json)
  render/           one sub-package per output view (er, openapi, routemap)
  web/              unlaravel serve — localhost dashboard, JSON API over the same model
testdata/           fixture Laravel app + golden outputs (pinned byte-for-byte)
vault/              ADRs, domain glossary, roadmap — the decision record
```

`engine.Analyze` is the only pipeline entry point; both the CLI and the web
server call it and share the identical output. Nothing outside
`internal/phpast` touches the PHP parser directly.

## Hard conventions

These are load-bearing, not style preferences. A change that violates one of
these needs an ADR, not a one-line justification in a commit message.

- **Determinism.** No Go map is ever marshaled into `unlaravel.json` or a
  renderer's output. Struct field order is emitted-JSON order; slices
  preserve source/discovery order. This is what makes golden-file tests
  possible.
- **The JSON contract is versioned.** Any shape change to `ProjectModel`
  bumps `model.CurrentSchemaVersion` (semver) with a doc-comment paragraph
  explaining the addition, in `internal/model/model.go`. Never hardcode the
  version literal elsewhere.
- **Non-nil empty slices.** Every `[]T` field on the model initializes to
  `[]T{}`, not `nil`, so an empty result serializes as `[]` rather than
  `null`. (One deliberate exception may exist where nil vs. empty is
  semantically meaningful — e.g. Eloquent's `$guarded = []`; if you introduce
  one, document it loudly in the same doc comment.)
- **Static only.** The engine never shells out to `artisan`, never connects
  to a database, never requires `vendor/`. This is why it works on legacy and
  half-broken projects. See ADR 0003.
- **Parser isolation.** All AST access goes through `internal/phpast`. If an
  extractor needs a new AST traversal, add a helper there — don't reach into
  `VKCOM/php-parser` node types directly from an extractor package.
- **Golden-file discipline.** `testdata/fixture-app` plus its
  `fixture-app.golden.*` files are the end-to-end contract test. Regenerate
  goldens deliberately (most test files support a `-update` flag) and review
  the diff — never regenerate blind.
- **Vertical slices.** One PR ships one node type, one renderer, or one
  command all the way through — not a partial layer across many. This is how
  the six-node MVP got built without a long half-working stretch.
- **Precision over coverage.** When a static heuristic risks false positives
  (e.g. N+1 detection), it stays deferred until the false-positive rate is
  provably low, rather than shipping something that makes the tool look
  wrong. See ADR 0002.

## Development commands

```bash
go build -o unlaravel ./cmd/unlaravel   # build the binary
go build ./...                          # build everything
go test ./... -race -cover              # full suite, race-clean, with coverage
gofmt -l cmd/ internal/                 # formatting check (CI-enforced)
go vet ./...                            # static analysis (CI-enforced)

./unlaravel analyze ./testdata/fixture-app              # try it on the bundled fixture
./unlaravel analyze ./testdata/fixture-app --output model.json --openapi openapi.json
./unlaravel serve                                       # interactive dashboard on :4448
```

## Working conventions

- Commit messages: `<type>: <description>` (feat, fix, refactor, docs, test,
  chore, perf, ci), explaining *why* over *what*.
- One PR per vertical slice, following the existing merged-PR history for
  scope and description style.
- A contract change (new/changed field on `ProjectModel`) is not complete
  until the version bump, the golden-file regeneration, and the affected
  renderer tests all land in the same PR.
- New ADR-worthy decisions get a file in `vault/01 - Architecture/ADRs/`,
  following the existing `NNNN-kebab-case.md` numbering, plus a row in the
  ADR Index.
