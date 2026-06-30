---
tags: [architecture, moc]
aliases: [Architecture Overview]
---

# Architecture Overview

> One engine builds one [[CONTEXT|Project Model]]; many [[CONTEXT|Renderers]] view it. Everything else follows from that.

Decisions behind this design: [[ADR Index]]. Language: [[CONTEXT]]. Sequencing: [[Roadmap]].

## The shape in one diagram (C4 container view)

```mermaid
flowchart TB
    subgraph laravel["📁 Target Laravel project (read-only)"]
        mig["database/migrations/*.php"]
        rt["routes/*.php"]
        ctl["app/Http/Controllers"]
        mdl["app/Models"]
        req["app/Http/Requests"]
        krn["app/Http/Kernel.php"]
    end

    subgraph engine["⚙️ Un(la)ravel engine (Go, static, single binary)"]
        ast["PHP AST parser\n(VKCOM/php-parser)"]
        sym["Symbol table\n(Phase 1: collect)"]
        ex["Extractors\n(Phase 2: resolve)"]
        pm["🧩 Project Model\n(in-memory graph)"]
    end

    json[("unlaravel.json\nversioned contract")]

    subgraph rend["🎨 Renderers (consume the JSON)"]
        er["ER diagram\n(Mermaid)"]
        oapi["OpenAPI 3 spec"]
        rep["Markdown report"]
    end

    pkg["🐘 Laravel package\nartisan unlaravel:analyze\n(thin wrapper)"]

    laravel --> ast --> sym --> ex --> pm --> json
    json --> er
    json --> oapi
    json --> rep
    json -.consumed by.-> pkg
    pkg -.shells out to.-> engine

    classDef deferred stroke-dasharray: 5 5;
```

> [!info] Why this shape
> - The **engine** is the only hard part ([[ADR 0001 - Project Model as the core abstraction|0001]]). Renderers are thin views.
> - It reads files **statically** — never boots Laravel ([[ADR 0003 - Static analysis without booting Laravel|0003]]).
> - `unlaravel.json` is the **public seam** ([[ADR 0004 - Serialized Project Model as output contract|0004]]) — CLI, package, and any future web UI consume it.
> - The package is a **thin wrapper** over the binary ([[ADR 0005 - Laravel package is thin wrapper over Go binary|0005]]).

## How an analysis flows (sequence)

```mermaid
sequenceDiagram
    actor Dev
    participant CLI as unlaravel CLI
    participant Det as detector
    participant P1 as Phase 1 (collect)
    participant P2 as Phase 2 (resolve)
    participant Mdl as Project Model
    participant R as Renderer

    Dev->>CLI: unlaravel analyze ./my-app --output model.json
    CLI->>Det: DetectLaravel(path)
    Det-->>CLI: ✓ artisan + composer.json present
    CLI->>P1: walk all *.php → build symbol table + use-maps
    P1->>P2: symbol table ready
    P2->>Mdl: resolve edges (Route→Controller, Model→Model, Route→FormRequest)
    Note over P2,Mdl: unresolved refs → dangling edges (= dead routes)
    Mdl-->>CLI: serialize → unlaravel.json
    CLI->>R: render(model)
    R-->>Dev: ER diagram / OpenAPI / report
```

## The two-phase extractor (the hard part)

See [[ADR 0006 - Two-phase extraction with symbol table|ADR 0006]]. Edges cross files, so resolution can't be per-file.

```mermaid
flowchart LR
    subgraph p1["Phase 1 — Collect (concurrent)"]
        a["parse file A"] --> sa["record FQN + use-map A"]
        b["parse file B"] --> sb["record FQN + use-map B"]
        c["parse file ..."] --> sc["..."]
    end
    st[("Symbol table\nFQN → file → AST node")]
    sa --> st
    sb --> st
    sc --> st
    subgraph p2["Phase 2 — Resolve"]
        r1["Route::post('/x', [PostController::class,'store'])"]
        r1 --> res{"resolve\nPostController"}
        res -->|found| ok["edge Route → Action"]
        res -->|not found| dangle["⚠️ dangling edge\n= dead route"]
    end
    st --> res
```

## The six MVP node types

See [[ADR 0002 - Six-node MVP scope|ADR 0002]] and [[CONTEXT]].

| Node | Source | Difficulty | Renderer it unlocks |
|---|---|---|---|
| **Schema** | `database/migrations/*.php` | 🟢 easy | ER diagram |
| **Model** (+rels) | `app/Models/*.php` | 🟡 medium | ER diagram |
| **Route** | `routes/*.php` | 🟡 med-hard (macros) | route map, OpenAPI |
| **Controller/Action** | `app/Http/Controllers` | 🟡 medium | route map, OpenAPI |
| **Middleware** | `Kernel.php` + groups | 🟡 medium | route map |
| **FormRequest** | `app/Http/Requests` | 🟡 medium | OpenAPI request bodies |

Deferred: **Resource** (response shapes) and **Hotspot** (N+1) — need real PHP expression analysis. See [[Roadmap]].

## Target package layout

```
cmd/unlaravel/main.go     entrypoint (MISSING today — Milestone 0)
internal/
  cli/                    cobra commands               (exists)
  detector/               composer + laravel detection (exists, working)
  model/                  Project Model + schemaVersion + JSON
  extract/{schema,model,route,controller,middleware,formrequest}
  render/{er,openapi,report}
testdata/                 fixture app + golden unlaravel.json + golden outputs
```

## Current reality (starting point)

> [!danger] The repo does not build today
> There is **no `cmd/unlaravel/main.go`** and nothing calls `cli.Execute()`. The `analyze` command prints **hardcoded fake `✅` lines** and never calls the (working) `detector`. Fixing this is [[Roadmap|Milestone 0]]. See [[Engineering Journal]] for the full starting-state assessment.
