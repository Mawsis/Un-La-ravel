---
tags: [adr]
status: accepted
date: 2026-07-09
aliases: ["ADR 0012 - Middleware as a node"]
---

# ADR 0012 — Middleware is a first-class node with a derived, non-serialized reverse index

**Middleware** becomes a first-class [[CONTEXT|Node]] in the Project Model: the
contract gains a top-level `middlewares` array
([[ADR 0004 - Serialized Project Model as output contract|ADR 0004]], contract
`1.10.0`), one entry per middleware the project knows about. The **reverse
index** — "which Routes apply this middleware" — is **derived at read-time** by a
consumer joining Routes against the nodes, and is **never serialized** onto the
node. A route's **direct** application (it named the alias) and its
**group-transitive** application (it named a group that contains the alias) are
two **distinct, labeled** relations, never merged.

## Context

Un(la)ravel already modeled a middleware's *application*: `Route.Middleware`
lists the names applied to each Route, and the [[CONTEXT|Auth state]] is derived
from them. But it had no node for the **middleware itself**. A developer could
not ask "what class does the `auth` alias resolve to?", "which groups is it in?",
"does it run on every request?", or "which routes apply it?" The alias→class→
group→global→priority mapping the HTTP **Kernel** declares was invisible, and
there was no node a future detail page could describe.

The domain glossary long separated a middleware's **alias** (declared in the
Kernel) from its **application** (named on a Route). Making the alias side a real
node turns that implicit Route→Middleware relationship into a resolvable one, and
is the last of the six MVP node types
([[ADR 0002 - Six-node MVP scope|ADR 0002]]) to gain a node.

Three questions had to be settled before building it, because each is expensive
to reverse once the contract ships.

## Decision

### 1. Middleware is a node

A new top-level `Middlewares []Middleware` slice on the Project Model (additive,
appended last, contract `1.10.0`). Each node carries: `alias` (nullable — a class
applied by FQN has none), resolved `class` FQN (nullable — an alias whose class
we have not read, or an `unknown` name we decline to guess), the `groups` it
belongs to, an `origin` (`framework` / `app` / `unknown`), a `global` flag, and
its `priority` ordering.

The **node set is a union**: Laravel's built-in alias backstop ∪ the Kernel's
declared aliases ∪ every name actually applied on a Route. An applied name that
nothing declares becomes a node with origin `unknown` and no class, so the
reverse index **never dangles**. Emit order is **tiered and map-free** — Kernel-
declared, then built-in canonical order, then applied-first-appearance —
deduped through a `seen` set but built by ordered append, so the array is
deterministic for golden-file tests (the determinism invariant).

The build lands in slices: issue #64 ships the node, the built-in backstop, and
the applied-union (the tracer bullet — no Kernel read yet); reading the Kernel's
own mapping from `app/Http/Kernel.php` (Laravel ≤10) or `bootstrap/app.php`
(Laravel 11+) follows in issues #66/#67, at which point `class` / `groups` /
`global` / `priority` fill in and the `app` origin appears.

### 2. The reverse index is derived, never serialized

"Which routes apply this middleware" is **computed by the consumer** (a renderer
or a web page) by joining each Route's applied middleware against the node,
matching on **base alias** — the parameter after the first `:` is stripped, so
`auth:sanctum` counts against `auth` and `throttle:60,1` against `throttle`. A
middleware application is recorded **only on the Route**, never copied onto the
node.

### 3. Direct and group-transitive applications are distinct labeled relations

A route that names an alias directly is a **direct** application. A route that
names a *group* containing the alias is a **group-transitive** application,
labeled with the group name. The two are kept as separate relations at the
consumer level, never merged into one list.

## Consequences

- **The reverse index cannot drift from the routes.** It is a projection of the
  live `Route.Middleware` data, recomputed on read — there is no second copy to
  fall out of sync, and no denormalized edge to maintain.
- **The contract stays minimal.** The node carries only what is intrinsic to the
  middleware; the N×M route↔middleware relation is never written to
  `unlaravel.json`, keeping the golden small and the shape stable.
- **A transitive edge is never mistaken for a direct one.** Because the split is
  in the model of the relation, not a rendering afterthought, a consumer that
  shows "routes applying `auth`" can always say *how* each route applies it.
- **No new `doctor` findings.** Auth state remains the sole middleware-derived
  finding; a "dead middleware alias" (declared alias → non-existent class) is a
  deferred future slice, so the health gate is not flooded with framework noise.
- **Determinism holds across the union.** Because the three tiers are appended in
  fixed order and never ranged from a map, adding Kernel reading later cannot
  reorder the existing framework/applied tiers.

## Considered and rejected

- **Serialize the reverse index onto each node** (`Middleware.routes: [...]`).
  Rejected: it denormalizes a relation that already lives on the Route,
  doubling the source of truth and bloating the contract with an N×M edge list
  that any consumer can derive in one pass.
- **Merge direct and group-transitive applications into one list.** Rejected: it
  discards the *why* — a developer auditing auth coverage needs to know whether a
  route authenticates because it said `auth` or because it joined a group that
  does; collapsing them hides exactly the fact the tool exists to surface.
- **Omit applied-but-undeclared names** (only emit declared/built-in aliases).
  Rejected: it leaves the reverse index dangling for any custom middleware a
  project applies but declares somewhere we can't read — the node set would be
  incomplete precisely where a developer is most likely to be confused. An
  `unknown`-origin node with no guessed class ([[ADR 0002 - Six-node MVP scope|
  precision over coverage]]) keeps the index complete without inventing facts.
