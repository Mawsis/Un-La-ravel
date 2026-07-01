# 🔍 Un(la)ravel

**Statically analyse a Laravel project — without booting Laravel — and un-ravel it into one navigable model, then render that model as diagrams, specs, and reports.**

Un(la)ravel reads a Laravel codebase the way a compiler front-end would: it parses the PHP into an AST, builds a single in-memory **Project Model** of the application, and emits that model as a versioned `unlaravel.json` plus human-friendly views. No database connection. No `vendor/`. No running app. It works on legacy and half-broken projects — exactly the ones you most need to understand.

> Built in Go as a study in real static analysis: AST visitors, a parser-isolation layer, and a clean engine→model→renderer architecture where every output is a thin view over one core.

---

## ✨ What it does today

Point it at a Laravel project and it produces an **Entity-Relationship diagram of your actual data model** — tables and columns from your migrations, *and* the Eloquent relationships from your models — then flags where the two **disagree**.

```console
$ unlaravel analyze ./my-laravel-app --output model.json

✅ Laravel project detected (version: ^11.0)
✅ Extracted 3 table(s) from ./my-laravel-app/database/migrations
✅ Extracted 3 Eloquent model(s)
✅ Wrote analysis to model.json

⚠️  Disagreements (1):
  • Post::editor — foreign key column "editor_id" not found on table "posts"

📊 Entity-Relationship diagram (Mermaid):
  ...
🎉 Analysis complete: 3 table(s), 3 model(s) found
```

### The diagram it generates

The ER diagram is emitted as [Mermaid](https://mermaid.js.org/), so it renders right here on GitHub — and in any Markdown file, Obsidian, or [mermaid.live](https://mermaid.live):

```mermaid
erDiagram
	CATEGORIES ||--o{ POSTS : "posts (hasMany)"
	POSTS }o--|| USERS : "author (belongsTo)"
	POSTS }o--|| CATEGORIES : "category (belongsTo)"
	USERS ||--o{ POSTS : "posts (hasMany)"
	USERS {
		bigInteger id PK
		string name
		string email
		timestamp email_verified_at
		timestamp created_at
		timestamp updated_at
	}
	POSTS {
		bigInteger id PK
		bigInteger user_id FK
		string title
		text body
		boolean published
		timestamp created_at
		timestamp updated_at
		bigInteger category_id FK
	}
	CATEGORIES {
		bigInteger id PK
		string name
		string slug
		timestamp created_at
		timestamp updated_at
	}
```

The box-and-column structure comes from your **migrations**. The relationship lines come from your **Eloquent models** (`hasMany`, `belongsTo`, …). When a model relationship points at a table or foreign-key column your migrations never created, that's a **Disagreement** — surfaced as a finding, not hidden. That's the kind of latent bug that's invisible until something breaks in production.

---

## 🚀 Quick start

Requires [Go 1.18+](https://go.dev/dl/).

```bash
# Build the binary
git clone https://github.com/Mawsis/Un-La-ravel.git
cd Un-La-ravel
go build -o unlaravel ./cmd/unlaravel

# Analyze any Laravel project
./unlaravel analyze /path/to/laravel-app

# Also emit the machine-readable model
./unlaravel analyze /path/to/laravel-app --output model.json
```

Don't have a Laravel app handy? Try it on the bundled fixture:

```bash
./unlaravel analyze ./testdata/fixture-app
```

To see the diagram rendered, copy the `erDiagram` block into [mermaid.live](https://mermaid.live) or any GitHub Markdown file.

---

## 🔌 Live API docs — OpenAPI 3 → Swagger UI

Point it at a Laravel project and Un(la)ravel generates a complete **OpenAPI 3.0.3 specification** — request bodies and all — by reading your routes, controllers, and **FormRequest** validation rules statically. Drop the generated `openapi.json` into [Swagger UI](https://swagger.io/tools/swagger-ui/) (or [editor.swagger.io](https://editor.swagger.io)) and you get browsable, interactive API docs for an app you've never run.

```bash
./unlaravel analyze ./my-laravel-app --openapi openapi.json
```

Each `POST`/`PUT` route whose controller action type-hints a FormRequest gets a real request-body schema, derived from the `rules()` array. A `StorePostRequest` with:

```php
public function rules(): array {
    return [
        'title'  => 'required|string|max:255',
        'status' => ['nullable', 'in:draft,published'],
        // ...
    ];
}
```

becomes this operation in the generated spec (real output, abridged):

```json
"/posts": {
  "post": {
    "operationId": "posts.store",
    "tags": ["PostController"],
    "requestBody": {
      "required": true,
      "content": { "application/json": { "schema": {
        "type": "object",
        "properties": {
          "title":  { "type": "string", "maxLength": 255 },
          "status": { "type": "string", "nullable": true, "enum": ["draft", "published"] }
        },
        "required": ["title", "body"]
      } } }
    },
    "responses": { "200": { "description": "OK" } }
  }
}
```

Laravel validation rules map to JSON-Schema faithfully: `max:255` → `maxLength` (or `maximum` for numbers), `in:` → `enum`, `email`/`url`/`uuid`/`date` → `format`, `required`/`nullable` handled correctly. Unknown rules are preserved in the property description rather than dropped.

---

## 🗺️ Route map + dead-route detection

Un(la)ravel resolves every route to the controller action that handles it — across files, via a two-phase symbol table — flattens route-group prefixes and middleware, expands `apiResource` macros, and **flags dead routes**: routes pointing at a controller or action that no longer exists.

```console
🗺️  Routes (11):
METHOD  URI                   CONTROLLER@ACTION          MIDDLEWARE
GET     /posts                PostController@index       -
POST    /posts                PostController@store       auth
GET     /admin/comments/{id}  CommentController@show     auth:sanctum, throttle:api
DELETE  /admin/users/{id}     UserController@destroy     auth:sanctum, throttle:api  ⚠ DEAD

⚠️  Dead routes (1):
  • DELETE /admin/users/{id} — action "destroy" not found on controller "…UserController"
```

That dead route is a real bug the tool finds statically — an endpoint that would 500 in production, invisible until someone hits it.

---

## 🧩 The `unlaravel.json` contract

The engine's real output is a single, versioned JSON document — every other view is a renderer over it. A machine-readable map of any Laravel app:

```json
{
  "schema_version": "1.3.0",
  "project_name": "acme/blog",
  "laravel_version": "^11.0",
  "schemas":       [ { "name": "posts", "columns": [ … ] } ],
  "models":        [ { "name": "Post", "table": "posts", "relationships": [
    { "kind": "belongsTo", "method": "author", "target": "User", "foreign_key": "user_id" }
  ] } ],
  "disagreements": [
    { "model": "Post", "relationship": "editor", "kind": "missing_fk_column",
      "reason": "foreign key column \"editor_id\" not found on table \"posts\"" }
  ],
  "routes":        [ { "method": "POST", "uri": "/posts", "controller": "PostController",
                       "action": "store", "middleware": ["auth"], "form_request": "StorePostRequest" } ],
  "controllers":   [ { "name": "PostController", "fqn": "App\\Http\\Controllers\\PostController",
                       "actions": ["index", "show", "store"] } ],
  "dead_routes":   [ { "method": "DELETE", "uri": "/admin/users/{id}", "kind": "missing_action" } ],
  "form_requests": [ { "name": "StorePostRequest", "fields": [
    { "name": "title", "rules": [ { "name": "required" }, { "name": "max", "args": ["255"] } ] }
  ] } ]
}
```

All six node types — **Schema, Model, Route, Controller, Middleware, FormRequest** — in one document. Every renderer (ER diagram, route map, OpenAPI spec) is a thin view over this.

---

## 🏗️ How it works

One engine builds one model; many renderers view it.

```
Laravel source ──▶ PHP AST ──▶ Extractors ──▶ Project Model ──▶ unlaravel.json
  (read-only)    (VKCOM/      (6 node types)  (in-memory        (versioned
                 php-parser)                   graph)            contract)
                                                     │
                                                     ├──▶ Mermaid ER diagram
                                                     ├──▶ route map (+ dead routes)
                                                     ├──▶ OpenAPI 3 spec
                                                     └──▶ (Markdown report — roadmap)
```

- **Static, never boots Laravel.** A real PHP AST via [`VKCOM/php-parser`](https://github.com/VKCOM/php-parser), isolated behind one package so the parser can be swapped without touching extractors.
- **One Project Model, many renderers.** The hard part is built once; every output is a cheap view. Adding a format is a renderer, not a new engine.
- **No database.** The model lives in memory and serializes to JSON. Single static binary, zero runtime dependencies.

Every architectural decision is recorded as an ADR in [`vault/`](./vault) (an Obsidian vault): the [Architecture Overview](./vault/01%20-%20Architecture/Architecture%20Overview.md), seven [ADRs](./vault/01%20-%20Architecture/ADRs), and a [glossary](./vault/02%20-%20Domain/CONTEXT.md) of the domain language.

---

## 🗺️ Status & roadmap

Un(la)ravel is built in vertical slices — each one takes a node type all the way through the pipeline.

The **six-node MVP is complete** — all six node types extract through one Project Model, with three renderers over it.

| Node / capability | Status |
|---|---|
| **Schema** (tables/columns from migrations) | ✅ Done |
| **Model** (Eloquent classes + relationships) | ✅ Done |
| **Route** + **Controller** + **Middleware** (groups, `apiResource`, symbol table) | ✅ Done |
| **FormRequest** (`rules()` → request bodies) | ✅ Done |
| **ER diagram** renderer (Mermaid) | ✅ Done |
| **Route map** + **dead-route detection** | ✅ Done |
| **OpenAPI 3** spec renderer (→ Swagger UI) | ✅ Done |
| **Disagreement** findings (Model ↔ Schema) | ✅ Done |
| `unlaravel.json` versioned contract (`1.3.0`) | ✅ Done |
| Markdown architecture report | 🔜 Roadmap |
| Laravel/Composer package (`artisan unlaravel:analyze`) | 🔜 Roadmap |
| API Resource (response schemas) · N+1 / performance analysis | 🧊 Deferred |

See the full [Roadmap](./vault/04%20-%20Roadmap/Roadmap.md).

---

## 🧪 Development

```bash
go build ./...            # build everything
go test ./... -race       # run the suite (race-clean)
go test ./... -cover      # with coverage
go build -o unlaravel ./cmd/unlaravel
```

The codebase is laid out by responsibility:

```
cmd/unlaravel/      entrypoint
internal/
  detector/         is-this-a-Laravel-project + composer parsing
  phpast/           the only package that imports the PHP parser
  model/            the Project Model + versioned JSON contract
  extract/          one sub-package per node type (schema, model, …)
  render/er/        Project Model → Mermaid ER diagram
  analyze/          Model ↔ Schema correlation (Disagreement findings)
testdata/           a fixture Laravel app + golden outputs
```

---

## 📄 License

MIT — see [LICENSE](./LICENSE).
