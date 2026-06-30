---
tags: [diagrams, moc, index]
aliases: [Diagrams Index]
---

# Diagrams Index

All diagrams are **Mermaid** (renders natively in Obsidian and GitHub — no image files to maintain, version-controllable as text). This matches [[ADR 0004 - Serialized Project Model as output contract|the renderer philosophy]]: the ER renderer itself emits Mermaid.

## Architecture diagrams (live in [[Architecture Overview]])

- **C4 container view** — engine, Project Model, renderers, package
- **Analysis sequence** — request → detect → two-phase → render
- **Two-phase extractor** — symbol table + dangling-edge resolution

## What Milestone 1 will produce (target ER output)

This is the **goal artifact** for [[Roadmap|Milestone 1]] — a Mermaid ER diagram rendered from real `database/migrations/*.php`. Example for a typical blog schema:

```mermaid
erDiagram
    USERS ||--o{ POSTS : "hasMany (author_id)"
    USERS ||--o{ COMMENTS : "hasMany (user_id)"
    POSTS ||--o{ COMMENTS : "hasMany (post_id)"
    POSTS }o--|| CATEGORIES : "belongsTo (category_id)"
    POSTS }o--o{ TAGS : "belongsToMany (post_tag)"

    USERS {
        bigint id PK
        string name
        string email UK
        timestamp email_verified_at
        timestamps created_updated
    }
    POSTS {
        bigint id PK
        bigint author_id FK
        bigint category_id FK
        string title
        text body
        boolean published
    }
    COMMENTS {
        bigint id PK
        bigint post_id FK
        bigint user_id FK
        text body
    }
    CATEGORIES {
        bigint id PK
        string name
    }
    TAGS {
        bigint id PK
        string name
    }
```

> [!note] Two sources, one diagram
> Boxes + columns come from **Schema** (migrations). The relationship lines come from **Model** relationships (`hasMany`/`belongsTo`/`belongsToMany`). Where they disagree (a `belongsTo` with no matching FK column), that's a **finding** to surface, not hide — see [[CONTEXT]].

## What Milestone 3 will produce (route map sketch)

```mermaid
flowchart LR
    subgraph mw["middleware: auth, throttle:api"]
        r1["POST /api/posts"] --> a1["PostController@store"]
        r2["GET /api/posts/{post}"] --> a2["PostController@show"]
    end
    a1 -.validates with.-> fr1["StorePostRequest.rules()"]
    a1 --> m1["Post model"]
    a2 --> m1
    r3["GET /api/ghost"] -.->|⚠️ dead route| x["(missing controller)"]
```

## Conventions

- **Mermaid only.** No `.png`/`.svg` checked in unless a diagram genuinely can't be expressed in Mermaid (then it goes in [[99 - Attachments]]).
- Every diagram in this vault should also be reproducible by a **Renderer** where applicable — the vault and the tool tell the same story.
