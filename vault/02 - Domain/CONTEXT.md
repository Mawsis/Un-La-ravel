---
tags: [domain, glossary, ubiquitous-language]
aliases: [CONTEXT, Glossary, Ubiquitous Language]
---

# Un(la)ravel — Context & Language

The ubiquitous language for Un(la)ravel. This is the **canonical glossary**: when code, docs, or conversation use a term, they must use it the way it's defined here. Terms marked _Avoid_ are aliases we deliberately don't use, to prevent ambiguity.

> [!info] This is a living document. It is updated *inline* the moment a term is resolved during a design/grilling session — not batched up later.

## Language

### Core abstraction

**Project Model**:
The single in-memory graph of a Laravel application produced by the engine — nodes (Routes, Controllers, Models, …) and the edges between them. Every renderer reads from this and nothing else.
_Avoid_: AST, parse tree (those are inputs *to* building the Project Model, not the model itself), "analysis result"

**Extractor**:
A component that reads one kind of source (migrations, route files, models, …) and populates the corresponding **nodes** in the **Project Model**.
_Avoid_: parser (too low-level), scanner, analyzer

**Renderer**:
A component that reads the finished **Project Model** and emits one output format (ER diagram, OpenAPI spec, Markdown report). Renderers never read PHP files directly.
_Avoid_: exporter, generator, formatter (used interchangeably in casual speech, but **Renderer** is canonical)

**Node**:
A typed entity in the **Project Model** (a Route, a Model, a Migration, …). The MVP has six node types.

**Edge**:
A typed relationship between two **Nodes** (Route → Controller, Model → Migration, Model → Model).

### The six MVP node types

**Schema** (a.k.a. Schema node):
The table-and-column structure of the database, extracted from `database/migrations/*.php`. The shape Laravel *would* build if migrated.
_Avoid_: Table (a Schema contains tables), Migration (the Migration is the *source*; the Schema is the *result*)

**Migration**:
A single `database/migrations/*.php` file describing a change to the **Schema**. Distinct from the **Schema** it contributes to.

**Model**:
An Eloquent model class (`app/Models/*.php`), with its inferred table and its **Relationships** to other **Models**.
_Avoid_: Entity, table (a Model *maps to* a table; it is not the table)

**Model graph** (a.k.a. model-centric diagram):
The diagram on a **Model**'s detail page: the **Model** at the center, edges to its *directly-related* **Models** labeled by **Relationship** kind (`hasMany`/`belongsTo`/…), and the columns of its mapped **Schema** table **annotated with mass-assignment state** (fillable / guarded / hidden). It is a **Renderer** over the **Model** and **Schema** nodes — *distinct from the ER diagram*, which is **table-centric** (nodes are tables with columns; a **Relationship** is drawn as an edge between table nodes). The Model graph is **model-centric** (the node *is* the Model; fillable/guarded/hidden are the overlay a plain ER can't show). It **reuses the ER's visual chrome** (node/edge boxes, the [[ADR 0009 - ER settle animation and diagram export|settle animation]]) but not its table-centric graph builder. Calling it "the ER diagram" re-introduces the **Schema**↔**Model** conflation this glossary exists to prevent.
_Avoid_: ER diagram (that is the table-centric Schema renderer), entity diagram

**Mass-assignment state**:
A per-column annotation fusing **Schema** and **Model**: whether a mapped table column is **fillable** (mass-assignable), **guarded** (protected from mass assignment), or **hidden** (excluded from serialization) per the **Model**'s `$fillable`/`$guarded`/`$hidden`. Invisible in a plain **ER diagram** (a Schema-only view); surfaced on the **Model graph** because mass-assignment protection is a correctness/security fact a reader of Laravel wants at a glance.
_Avoid_: protected (overloaded — say guarded), whitelist/blacklist

**Relationship**:
An Eloquent association a **Model** declares on another **Model**, via a `$this-><kind>(...)` call wrapped in one of the Model's own methods. The kinds we model are `hasMany`, `hasOne`, `belongsTo`, and `belongsToMany`. A Relationship carries the declaring method name, the target Model, and any explicit foreign/local key the source supplied. It is the **Edge** between two **Models**.
_Avoid_: association (acceptable casually, but **Relationship** is canonical), link, foreign key (the FK is a *column* the Relationship may reference, not the Relationship itself)

**Disagreement**:
A **Model** **Relationship** that references a table or foreign-key column absent from the **Schema**; surfaced as a finding, not an error. Because the **Schema** is extracted from **Migrations** and a **Model** is an independent *mapping* onto it, the two can disagree — a Relationship may point at a table no Migration creates, or name an explicit foreign-key column that table lacks. Reporting that disagreement is the feature.
_Avoid_: error, bug, mismatch (a Disagreement is a *finding* about two valid sources that conflict, not a failure), broken relation

**Route**:
A single HTTP endpoint declared in `routes/*.php` — method + URI + the **Controller** action and **Middleware** it binds to.
_Avoid_: Endpoint (acceptable casually, but **Route** is canonical), path

**Controller** (and **Action**):
A controller **class** in `app/Http/Controllers`, at any nesting depth — a class in a subdirectory carries that subdirectory as a namespace segment (`App\Http\Controllers\Admin\AdminDashboardController`). An **Action** is one public method on it that a **Route** points to. We say "Controller" for the class, "Action" for the method.
_Avoid_: handler

**Controller reference**:
The controller as written *at the route site* — the class-const in `[C::class, 'm']` or the class part of a legacy `"C@m"` string. It is recorded **verbatim** by the route extractor (fully-qualified, imported-short, or partially-qualified exactly as the source wrote it), never reduced to a bare short name, so that **[[ADR 0006 - Two-phase extraction with a symbol table|two-phase resolution]]** can qualify it against the route file's `use` imports. Collapsing it to the last segment and rebuilding an FQN from the **default controller namespace** (`App\Http\Controllers`) is the bug that produced false **Dead Routes** for sub-namespaced controllers: `App\Http\Controllers\Admin\AdminDashboardController` was reported not-found as `App\Http\Controllers\AdminDashboardController`. One shape remains unresolved and is a documented limitation: a *partially*-qualified reference (`Admin\C::class` under `use App\Http\Controllers;`), because it is qualified enough to skip the import map yet not a full FQN — rare in route files (ADR 0002 precision-over-coverage).
_Avoid_: controller short name (the reference is NOT reduced to a short name anymore)

**Dead Route**:
A **Route** whose **Controller** or **Action** can't be resolved — the two-phase symbol-table resolution ([[ADR 0006 - Two-phase extraction with a symbol table]]) could not match the route's controller reference to a declared **Controller** class (`missing_controller`), or matched the class but not the named **Action** method (`missing_action`). A Dead Route is a dangling **Edge** in the **Project Model**, surfaced as a **finding**, not an error — the tool reports it; it does not fail on it. Like a **Disagreement**, it is a fact about two valid sources (the route files and the controller classes) that don't line up.
_Avoid_: broken route, 404 (a 404 is a *runtime* miss; a Dead Route is a *static* dangling edge), error, bug

**Middleware**:
A request/response pipeline stage, and — as of the middleware slice — a first-class **Node** in the **Project Model**. Two facets, long distinguished in this glossary:
- its **alias** — the string a **Route** references (`auth`, `throttle`), mapped to a **class** and to zero-or-more **groups** in the **Kernel** (`app/Http/Kernel.php`, Laravel ≤10) or **bootstrap config** (`bootstrap/app.php`, Laravel 11+);
- its **application** — the alias (with any parameter, `auth:sanctum`) as declared on a **Route** or route group, already carried on `Route.Middleware`.

A Middleware node carries its **alias** (nullable — a class applied by FQN has none), its resolved **class** FQN (nullable — an alias we can't resolve to a class), the **groups** it belongs to (`web`, `api`), an **origin** (`framework` built-in / `app` declared / `unknown` applied-but-undeclared), and whether it is **global** (always-on, from the Kernel's `$middleware` stack / 11+ `->append`/`->prepend`) versus alias-applied. The node set is the **union** of Kernel-declared aliases, Laravel's built-in alias table, and every name actually **applied** on a **Route** — so the reverse "which routes apply this" index never dangles. **Priority** (`$middlewarePriority`) is modeled as an ordering.

The Route↔Middleware reverse index ("which routes apply this middleware") is **derived, never serialized** — computed by joining each **Route**'s applied middleware against the node. The join is on **base alias** (the applied string's parameter is stripped: `auth:sanctum` and `throttle:60,1` both apply `auth`/`throttle`). **Direct application** (a Route literally names the alias) and **group-transitive application** (a Route names a **group** that *contains* the alias) are two **distinct, labeled relations**, never fused into one "used by" list: a `web`-group route is shown under "applied via group `web`", not mixed into the direct list, so a reader can always tell the certain edge from the transitive one, and the distinction survives a group edit (ADR 0002 precision-over-coverage).
_Avoid_: filter, interceptor; "used by" (too coarse — say *directly applied by* or *applied via group* )

**Kernel**:
The source of the middleware alias→class→group mapping. In Laravel ≤10 it is `app/Http/Kernel.php` (the `$middlewareAliases`/`$routeMiddleware`, `$middlewareGroups`, `$middleware`, `$middlewarePriority` properties, read as array literals); in Laravel 11+ the class is gone and the equivalent lives in `bootstrap/app.php`'s `->withMiddleware(...)` closure (the `->alias([...])` array and group/global arrays). Un(la)ravel reads whichever exists — exotic fluent 11+ mutation chains beyond the array-literal forms are best-effort (ADR 0002 precision-over-coverage).
_Avoid_: config (too generic), bootstrap (that's the 11+ file, one of two Kernel forms)

**Auth state**:
A **Route**'s answer to "does its flattened **Middleware** stack authenticate?" — one of `authenticated`, `unauthenticated`, or `unknown`. Computed from the middleware names alone by matching Laravel's *conventional* auth middleware (`auth`, `auth:<guard>`, `auth.basic`, Sanctum/Passport guards). Unrecognized custom middleware is `unknown`, never guessed ([[ADR 0002 - Six-node MVP scope|precision over coverage]]): a custom guard we can't read might or might not authenticate, and pretending to know either way would either hide a real hole or cry wolf. The state is stamped onto every Route in the contract, so the dashboard reads it rather than re-classifying ([[ADR 0008 - Findings model and the doctor gate]]).
_Avoid_: protected/guarded (that's the mass-assignment sense of **guarded**), secured, logged-in

**Unauthenticated Route**:
A **Route** whose **Auth state** is `unauthenticated` — reachable without logging in. Like a **Dead Route**, it is a **finding**, not an error: the tool reports it; it does not fail on it. Split by verb, because the same missing-auth fact is graver on a route that changes state: an unauthenticated *write* (POST/PUT/PATCH/DELETE) is a `blocker`, an unauthenticated *read* a `warn`. An `unknown`-auth route is deliberately NOT a finding. An intentionally-public route is silenced through the **baseline**, not by weakening the classifier.
_Avoid_: insecure route, vulnerability, open endpoint (overstates a static fact into a runtime exploit claim)

**FormRequest**:
A validation class in `app/Http/Requests` whose `rules()` array defines the accepted request body for a **Route**. Source of OpenAPI request-body schemas.
_Avoid_: validator, request (too generic)

### The health gate (doctor)

**Finding**:
A single itemized problem in the **Project Model** — a **Dead Route**, a **Disagreement**, or an unguarded **Model**. The *category rollup* (`model.Finding`: kind + count + label) is what the dashboard verdict and `doctor` print; the *per-item* form (`findings.Item`, in-memory only, never serialized to `unlaravel.json`) is what the **Baseline** fingerprints.
_Avoid_: error, warning (a Finding is reported, never failed-on except via the gate); issue (too generic).

**Fingerprint**:
A stable, content-derived string identifying one per-item **Finding**, positional-independent: a **Dead Route** by method + URI + controller, an unguarded **Model** by class name, a **Disagreement** by model + relationship. It reads a finding's *identity*, never its slice position or source line, so a refactor that reorders or moves code does not change it.
_Avoid_: hash (implies opacity/collision concern), id (implies assigned, not derived).

**Baseline**:
A committed set of **Fingerprints** a project has chosen not to gate CI on — the findings it already knows about. Stored as versioned JSON with sorted entries (byte-deterministic). Lets a legacy project adopt the `doctor` gate without fixing history first: the gate then fires only on *new* findings. See [[ADR 0008 - Findings model and doctor]].
_Avoid_: allowlist, ignore-file (a Baseline suppresses from the *gate*, not from the *report* — the debt stays visible).

**Suppressed** / **Stale**:
A **Finding** is **Suppressed** when its **Fingerprint** is in the **Baseline**: excluded from the exit-code decision but still reported. A **Baseline** entry is **Stale** when it matches no current **Finding** (the finding was fixed): reported so the **Baseline** can be pruned, never silently dropped.
_Avoid_: ignored, hidden (Suppressed findings are still printed); obsolete (Stale is the term for a no-longer-matching entry).

### Things we deliberately do NOT model (yet)

**Resource** _(deferred — roadmap)_:
An API Resource (`app/Http/Resources`) whose `toArray()` defines a response shape. Deferred because extracting it needs real PHP expression analysis. See [[Roadmap]].

**Hotspot** _(deferred — roadmap)_:
A suspected performance problem (N+1, missing eager-load). Deferred — static detection has high false-positive risk that would make the tool look *wrong*. See [[Roadmap]].

## Relationships

- An **Extractor** populates **Nodes** in the **Project Model**
- A **Renderer** reads the **Project Model** and emits an output; it never reads source files
- A **Migration** contributes to the **Schema**; a **Model** maps to a table in the **Schema**
- A **Route** binds to exactly one **Action** (on a **Controller**) and zero-or-more **Middleware**
- A **Route** may bind to one **FormRequest** (via its **Action**'s type-hinted argument)
- A **Model** relates to other **Models** via **Relationships** (`hasMany`, `hasOne`, `belongsTo`, `belongsToMany`)
- A **Relationship** that references a table or foreign-key column the **Schema** lacks produces a **Disagreement** finding
- A **Route** whose **Controller**/**Action** edge cannot be resolved against the declared **Controller** classes produces a **Dead Route** finding
- A **Finding** reduces to a **Fingerprint**; a **Baseline** is a set of **Fingerprints** that **Suppresses** matching **Findings** from the `doctor` gate while still reporting them

## Example dialogue

> **Dev:** "When the **Route** extractor finds `Route::apiResource('posts', PostController::class)`, does it create one **Route**?"
> **Domain expert:** "No — `apiResource` expands to several **Routes** (index, store, show, update, destroy), each binding to a different **Action** on the same **Controller**. The extractor must expand the macro, not store it verbatim."

> **Dev:** "So the **Schema** comes from the **Models**?"
> **Domain expert:** "No — that's the conflation we're avoiding. The **Schema** is extracted from **Migrations**. A **Model** *maps to* a table in that **Schema**, and may disagree with it. Surfacing that disagreement is a feature, not a bug."

## Flagged ambiguities

- **"Feature"** was used to mean both *engine capability* and *user-facing output*. **Resolved:** the engine has **Extractors** and **Node types**, never "features." User-facing outputs are **Renderers**. The word "feature" is banned in architecture discussion. See [[ADR 0001 - Project Model as the core abstraction]].
- **"Schema" vs "Migration" vs "Model"** were used loosely for "the database." **Resolved:** three distinct concepts above. The **Schema** is the *result* of **Migrations**; a **Model** is a *mapping* onto it.
- **"Everything"** (as a scope) was resolved to mean "every output is a **Renderer** over one **Project Model**," not "30 parallel feature builds." See [[ADR 0002 - Six-node MVP scope]].
- **"Mismatch" / "broken relationship"** were used for the case where a **Model** **Relationship** points at something the **Schema** lacks. **Resolved:** this is a **Disagreement** — a *finding* about two independently-valid sources (the **Migration**-derived **Schema** and the **Model**) that conflict, never an "error" or "bug." The tool reports it; it does not fail on it.
- **"Broken route" / "404"** were used for a **Route** whose **Controller**/**Action** doesn't resolve. **Resolved:** this is a **Dead Route** — a *finding* (a dangling **Edge**) from the two-phase resolution ([[ADR 0006 - Two-phase extraction with a symbol table]]), never an "error." A 404 is a *runtime* miss; a Dead Route is a *static* one. The tool reports it; it does not fail on it.
