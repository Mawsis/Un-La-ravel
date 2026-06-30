---
tags: [moc, home]
---

# 🔍 Un(la)ravel — Project Vault

> One engine that builds a single model of a Laravel app, and many thin renderers that turn that model into diagrams, docs, and reports.

This vault is the **single source of truth** for *why* Un(la)ravel is built the way it is. Code lives in the repo; **decisions, language, and diagrams live here.**

## 🗺️ Maps of Content

- [[Architecture Overview]] — the engine, the Project Model, the renderers
- [[CONTEXT]] — the ubiquitous language (glossary). Read this first.
- [[ADR Index]] — every architectural decision and its rationale
- [[Roadmap]] — what's in the MVP, what's deferred, and why
- [[Diagrams Index]] — C4, ER, pipeline, and sequence diagrams
- [[Engineering Journal]] — dated log of decisions as they were made

## 🎯 The one-sentence pitch

Un(la)ravel statically analyses a Laravel project — **without booting Laravel** — and "un-ravels" it into one navigable **Project Model**, then renders that model as an **ER diagram**, an **OpenAPI spec**, and a **Markdown architecture report**.

## 🧭 Why this vault exists

A portfolio reviewer spends ~90 seconds on a repo. The code shows *what*; this vault shows *judgment* — the trade-offs, the deliberate "no"s, the architecture chosen on purpose. That judgment is the differentiator.

## Status at a glance

```dataview
TABLE status, date
FROM "01 - Architecture/ADRs"
SORT file.name ASC
```

> [!note] If the Dataview block above is empty, install the **Dataview** community plugin in Obsidian. It's optional — every page works without it.
