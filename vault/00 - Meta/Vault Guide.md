---
tags: [meta, moc]
aliases: [Vault Guide]
---

# Vault Guide

How this Obsidian vault is organized and what belongs where. Start at [[Home]].

## Purpose

This vault is the **decision and design record** for Un(la)ravel. The repo holds *what* the code does; the vault holds *why* it's shaped that way — the judgment that makes this a portfolio piece, not just a script.

## Folder map

| Folder | Holds | Key note |
|---|---|---|
| `00 - Meta` | This guide, vault conventions | — |
| `01 - Architecture` | [[Architecture Overview]] + `ADRs/` | ADRs are the heart — see [[ADR Index]] |
| `02 - Domain` | [[CONTEXT]] (ubiquitous language) | Read first; updated *inline* as terms resolve |
| `03 - Diagrams` | [[Diagrams Index]] | Mermaid-only, version-controllable |
| `04 - Roadmap` | [[Roadmap]] | Milestones + deferred items + risks |
| `05 - Engineering Journal` | [[Engineering Journal]] | Dated narrative, newest first |
| `99 - Attachments` | Images that can't be Mermaid | Avoid unless necessary |

## Conventions

- **Wikilinks everywhere** (`[[` Page Name `]]`) — this is what makes Obsidian's graph view useful. Link liberally.
- **Mermaid for all diagrams** — renders in both Obsidian and GitHub; no binary image churn.
- **ADRs are immutable-ish.** Don't rewrite a decision; supersede it with a new ADR and set the old one's status to `superseded by ADR-NNNN`.
- **CONTEXT.md is opinionated.** One canonical term per concept; aliases listed under _Avoid_.
- **Frontmatter `tags`** drive Obsidian search and any Dataview queries on [[Home]].

## Recommended Obsidian plugins (all optional)

- **Dataview** — powers the ADR status table on [[Home]]. Everything works without it.
- **Graph view** (built-in) — visualize how decisions, terms, and diagrams interlink.

## Reading order for a newcomer

1. [[Home]] — the pitch
2. [[CONTEXT]] — the language
3. [[Architecture Overview]] — the shape + diagrams
4. [[ADR Index]] — the decisions and their rationale
5. [[Roadmap]] — what's built, what's next, what's deferred
6. [[Engineering Journal]] — the story of how we got here
