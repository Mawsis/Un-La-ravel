---
tags: [adr]
status: accepted
date: 2026-07-08
aliases: ["ADR 0012 - Middleware as a first-class node"]
---

# ADR 0012 — Middleware is a first-class Node; its reverse index is derived, and group-transitive application is a distinct labeled edge

**Middleware** becomes a first-class **[[CONTEXT|Node]]** in the Project Model —
alongside Route, Controller, Model, Schema, and FormRequest — carrying the
Kernel's alias→class→group→global→priority mapping. The **Route↔Middleware
reverse index** ("which routes apply this middleware") is **derived at
read-time, never serialized**. Direct application (a Route names an alias) and
**group-transitive** application (a Route names a group that *contains* the
alias) are modeled as **two distinct, labeled relations**, never fused into one
"used by" list.

## Context

Un(la)ravel already captured middleware *applications* — the string names on each
Route (`Route.Middleware`) — and derived **Auth state** from them
([[ADR 0002 - Six-node MVP scope|precision over coverage]]). What it lacked was
the **Middleware node itself**: the alias→class→group mapping declared in the
**Kernel**, and a hub that answers "what does `auth` actually resolve to, and
which routes run it?"

Three forces shaped the decision:

- **Two incompatible Laravel layouts.** In Laravel ≤10 the mapping lives in
  `app/Http/Kernel.php` (the `$middlewareAliases`/`$routeMiddleware`,
  `$middlewareGroups`, `$middleware`, `$middlewarePriority` array-literal
  properties). In Laravel 11+ the Kernel class is gone; the equivalent lives in
  `bootstrap/app.php`'s `->withMiddleware(...)` closure. The tool must run on any
  version ([[ADR 0003 - Static analysis without booting Laravel|static, no boot]]).
- **The reverse index invites redundancy.** Materializing "applied by these
  routes" onto each node duplicates data already carried on the Route, and a Go
  map through which a union is deduped threatens the **determinism** invariant
  ([[ADR 0004 - Serialized Project Model as output contract]]).
- **Group membership invites a wrong edge.** A Route applying the `web` group
  transitively runs every middleware `web` contains. Folding that into a per-alias
  "used by" count would make a `web`-group route look like it names `auth`
  directly — an inferred edge that overstates and breaks silently when the group
  is edited. This is exactly the false-positive class ADR 0002 defers.

## Decision

- **A Middleware node** is added as a top-level `Middlewares []Middleware` slice
  on the Project Model. Each node carries: `alias` (nullable), resolved `class`
  FQN (nullable), the `groups` it belongs to, an `origin` (`framework` built-in /
  `app` declared / `unknown` applied-but-undeclared), a `global` flag (from the
  always-on `$middleware` stack / 11+ `->append`/`->prepend`), and its
  `$middlewarePriority` ordering. The contract bumps (additive: the slice is
  appended last).
- **Sources, in order of static tractability:** the ≤10 **Kernel** array-literal
  properties; the 11+ `bootstrap/app.php` `->alias([...])`/group/global arrays;
  and a **built-in alias backstop table** (`auth`, `guest`, `throttle`, `can`,
  `verified`, `signed`, …) so a node exists for framework middleware the app
  never declares. Exotic fluent 11+ mutation chains beyond the array-literal
  forms are best-effort (ADR 0002).
- **The node set is a union** of Kernel-declared aliases ∪ built-in table ∪ every
  name **applied** on a Route — so the reverse index never dangles. Emit order is
  **tiered and map-free**: Kernel source-declaration order, then built-in
  canonical-table order, then applied-first-appearance order, deduped through a
  `seen` set but built by ordered append (determinism invariant).
- **The reverse index is derived, not serialized.** Consumers join each Route's
  applied middleware against the node, matching on **base alias** (the applied
  string's parameter is stripped: `auth:sanctum` applies `auth`). A middleware
  application is recorded in exactly one place — the Route — never copied onto the
  node.
- **Direct vs. group-transitive are two labeled relations.** The node's page shows
  "directly applied by" (routes that literally named the alias) and, separately,
  "applied via group `web`" (routes that named a containing group) — never merged.
- **The slice is descriptive-only.** It produces **no new `doctor` findings** and
  does not touch the health gate; **Auth state** remains the sole middleware-
  derived finding. A "dead middleware alias" (declared alias → non-existent class)
  is a real dangle but is deferred to its own slice for the same false-positive
  scrutiny Dead Routes received (ADR 0002).

## Consequences

- **Works on any Laravel version** without booting it — the same node shape backs
  both the ≤10 Kernel and 11+ bootstrap layouts, and the built-in backstop means
  `auth` resolves even in an app that declares nothing.
- **One source of truth for an application.** The Route carries the applied name;
  the node carries the declaration; the join is computed, so the two can never
  drift or double-count. The contract stays minimal.
- **The tool never overstates.** A reader can always tell a certain edge (direct)
  from a transitive one (via group), and the distinction survives a group edit —
  the honesty ADR 0002 is built to protect.
- **Cost deferred deliberately:** partially-qualified controller references (the
  slice-(a) known limitation), exotic 11+ fluent middleware chains, and the
  dead-middleware-alias finding are all documented deferrals, not silent gaps.
