// Package model defines the Project Model — the single in-memory
// representation of an analyzed Laravel application (ADR 0001) — and its
// stable JSON serialization, which is the versioned public output contract
// emitted as unlaravel.json (ADR 0004).
//
// This package is the seam every Renderer reads from. It is pure data plus
// JSON only: it MUST NOT import the PHP parser, the CLI, the detector, or any
// I/O. Extractors populate these structs; Renderers consume the serialized
// JSON, defined against this shape rather than against the Go structs.
//
// There is no database (ADR 0007): the model lives in memory during a run and
// serializes to JSON.
package model

import (
	"encoding/json"
	"fmt"
)

// CurrentSchemaVersion is the version of the JSON output contract this build
// produces. It is written into every serialized ProjectModel as schema_version
// (ADR 0004) so consumers can detect incompatibilities. Bump it following
// semantic versioning when the JSON shape changes. Defined once here; never
// hardcode the literal elsewhere.
//
// Bumped to 1.1.0 when the Eloquent slice added the "models" and
// "disagreements" arrays to the contract (ADR 0004): the shape grew
// backward-compatibly, so consumers can detect the richer output.
//
// Bumped to 1.2.0 when the Routes slice added the "routes", "controllers", and
// "dead_routes" arrays: another backward-compatible growth of the contract.
//
// Bumped to 1.3.0 when the FormRequest slice added the "form_requests" array
// and the optional "form_request" field on each Route linking it to its
// request class: another backward-compatible growth of the contract.
//
// Bumped to 1.4.0 when the Eloquent slice added "fillable", "guarded", and
// "casts" to each Model, and the Schema slice added "indexes" to each Table:
// another backward-compatible growth of the contract.
//
// Bumped to 1.5.0 when the ER-diagram engine swap (issue #21) added the
// structured ER graph to the served ER response: the web /api/er and
// /api/analyze payloads now carry an "er" object (nodes + edges over the
// tables and relationships) alongside the existing Mermaid string, the data
// contract the new browser renderer draws. The ProjectModel struct itself is
// unchanged — this bump versions the served renderer output, growing the
// contract backward-compatibly so consumers can detect the richer response.
//
// Bumped to 1.6.0 when the findings slice added the "findings" array: an
// itemized health verdict (dead routes, model↔schema disagreements, unguarded
// models) computed server-side by internal/findings and serialized into the
// contract. It lifts the verdict that previously lived only in the browser
// (issue #27) into the Project Model, so the CLI's `doctor` command, the
// unlaravel.json contract, and the dashboard all read ONE source of truth
// rather than each re-deriving it. Another backward-compatible growth: the
// array is appended last, so existing consumers are unaffected.
//
// Bumped to 1.7.0 when ER edge reconciliation (issue #36) added "origin" and
// "unresolved" to each edge of the served ER graph. Origin ("schema" or
// "eloquent") names the pass that produced the edge, replacing the browser's
// label-sniffing; unresolved marks an edge whose inferred endpoint table did
// not exist and was retargeted to its unambiguous singular/plural sibling, so
// the renderer can style the repaired edge distinctly. The same slice
// guarantees every emitted edge endpoint is a node — a dangling edge is
// reconciled or dropped, never served. The ProjectModel struct itself is
// unchanged; like 1.5.0 this bump versions the served renderer output,
// growing the contract backward-compatibly.
//
// Bumped to 1.8.0 when Finding severity (issue #47) added "severity" to each
// entry of the "findings" array: a machine-readable level ("blocker", "warn",
// or "info") derived from the finding's kind through the single SeverityFor
// table in internal/model/findings.go — an unguarded model is a blocker, a dead
// route or Model↔Schema disagreement is a warning. It lets the dashboard group
// findings by severity and a CI gate rank them without re-deriving severity from
// kind. A backward-compatible growth: the key is inserted after "kind", every
// existing finding gains it, and consumers that ignore it are unaffected.
//
// Bumped to 1.9.0 when auth coverage (issue #50) added the per-route "auth" field
// and two new finding kinds. Each Route now carries "auth" — "authenticated",
// "unauthenticated", or "unknown" — computed from its flattened middleware by the
// classifier in internal/findings (conventional Laravel auth middleware only;
// unrecognized custom middleware is "unknown", never guessed — precision over
// coverage, ADR 0002). The "findings" array gains two kinds: "unauthenticated_write"
// (a blocker: a POST/PUT/PATCH/DELETE route reachable without auth) and
// "unauthenticated_read" (a warning: a non-mutating route reachable without auth),
// which link to the new Auth view rather than the findings view. A
// backward-compatible growth: the new route key is appended after "middleware",
// the new finding kinds append after the existing categories, and consumers that
// ignore them are unaffected.
//
// Bumped to 1.10.0 when the controller-FQN fix (issue #63) changed the DOCUMENTED
// MEANING of each Route's "controller" field: it now carries the controller
// reference VERBATIM as written at the route site — fully-qualified
// (App\Http\Controllers\Admin\FooController), imported-short (FooController), or
// partially-qualified — rather than the bare last segment the extractor used to
// collapse it to. Two-phase resolution (ADR 0006) qualifies that verbatim
// reference against the route file's `use` imports, so sub-namespaced controllers
// resolve to their true FQN and are no longer falsely reported as dead routes.
// The field's key and type are unchanged (still a string named "controller"), but
// its value shape changes for any route that names a namespaced controller, so
// the golden files change and the version is bumped to signal it. A route that
// wrote a bare short name is byte-for-byte identical to before; a route that wrote
// a namespaced reference now serializes the full reference instead of the short
// name.
//
// Bumped to 1.11.0 when middleware became a first-class node (issue #64, ADR
// 0012): the contract gains a top-level "middlewares" array, appended last.
// Each entry carries the middleware's "alias" (omitted when applied by class
// with no alias), resolved "class" FQN (omitted when unread/unknown), the
// "groups" it belongs to (non-nil), its "origin" ("framework" / "app" /
// "unknown"), a "global" flag, and its "priority" ordering. The node set is the
// union of Laravel's built-in alias backstop (origin "framework") and every
// middleware name actually applied on a Route that is otherwise undeclared
// (origin "unknown", base alias with the parameter stripped), so the derived
// reverse index ("which routes apply this middleware") never dangles. Emit order
// is tiered and map-free — built-in canonical order, then applied-first-
// appearance — so the array is deterministic for golden-file tests. Reading the
// Kernel's own alias→class→group→global→priority mapping layers on in later
// slices; until then class/groups/global/priority carry their zero values. A
// backward-compatible growth: the array is appended last, so existing consumers
// are unaffected.
//
// Bumped to 1.12.0 when the Laravel ≤10 Kernel reader (issue #66) began
// populating the middleware node fields that 1.11.0 introduced but always left
// at their zero values. On a project with an app/Http/Kernel.php, each declared
// alias now resolves: "class" carries the FQN the alias maps to, "groups"
// reflects the alias's class membership in $middlewareGroups, "global" is true
// when that class is in the global $middleware stack, and "priority" is its
// 1-based position in $middlewarePriority. A new "app"-origin tier (the origin
// vocabulary 1.11.0 already reserved) is emitted FIRST in the tiered order —
// Kernel-declared, then the built-in backstop for aliases the Kernel did not
// declare, then applied-but-undeclared names — so a declared built-in such as
// "auth" appears once, carrying its resolved class, rather than as a bare
// framework node. No struct field is added or removed and the emit order stays
// map-free and deterministic: this is a backward-compatible enrichment of
// existing fields, so consumers reading the 1.11.0 shape are unaffected. A
// project without a ≤10 Kernel (Laravel 11+, whose bootstrap/app.php reader is a
// later slice) is unchanged — the fields stay at their zero values.
//
// Bumped to 1.13.0 when the Model node gained "hidden" (issue #65): the
// `protected $hidden` array a model declares, the columns Laravel strips when
// the model is serialized to an API response. It is read as an array literal by
// the model extractor and lands beside "fillable" and "guarded", carrying the
// SAME load-bearing nil-vs-empty semantics those two established — `null` means
// the property was never declared, `[]` means the source explicitly wrote
// `protected $hidden = [];` — so a consumer can tell an omission from a
// deliberate "nothing here is secret". A backward-compatible growth: the key is
// inserted after "guarded", every existing model gains it, and consumers that
// ignore it are unaffected. A prefactor for the Model page's mass-assignment
// overlay (issue #70), landed as contract + extractor before the page reads it.
const CurrentSchemaVersion = "1.13.0"

// jsonIndent is the indentation used for the serialized contract. Two spaces
// keeps golden-file diffs small and deterministic.
const jsonIndent = "  "

// ProjectModel is the top-level Project Model: the typed result of analyzing
// one Laravel application. It is the root of the JSON output contract.
//
// Field order in the struct is the field order in the emitted JSON. Slices
// preserve insertion order (Schemas in discovery order, each Table's Columns
// in source-declaration order); nothing here is sorted, and no Go map is
// serialized, so the output is deterministic for golden-file tests.
type ProjectModel struct {
	SchemaVersion  string         `json:"schema_version"`
	ProjectName    string         `json:"project_name"`
	LaravelVersion string         `json:"laravel_version"`
	Schemas        []Table        `json:"schemas"`
	Models         []Model        `json:"models"`
	Disagreements  []Disagreement `json:"disagreements"`
	Routes         []Route        `json:"routes"`
	Controllers    []Controller   `json:"controllers"`
	DeadRoutes     []DeadRoute    `json:"dead_routes"`
	FormRequests   []FormRequest  `json:"form_requests"`
	Findings       []Finding      `json:"findings"`
	Middlewares    []Middleware   `json:"middlewares"`
}

// New constructs a ProjectModel for the named project, stamping it with the
// CurrentSchemaVersion. Every slice is initialized to a non-nil empty slice so
// an analysis that finds none of a given kind serializes "schemas": [],
// "models": [], "disagreements": [], "routes": [], "controllers": [],
// "dead_routes": [], "form_requests": [], "findings": [], and
// "middlewares": [] rather than null.
func New(projectName, laravelVersion string) *ProjectModel {
	return &ProjectModel{
		SchemaVersion:  CurrentSchemaVersion,
		ProjectName:    projectName,
		LaravelVersion: laravelVersion,
		Schemas:        []Table{},
		Models:         []Model{},
		Disagreements:  []Disagreement{},
		Routes:         []Route{},
		Controllers:    []Controller{},
		DeadRoutes:     []DeadRoute{},
		FormRequests:   []FormRequest{},
		Findings:       []Finding{},
		Middlewares:    []Middleware{},
	}
}

// AddTable appends a Table to the model in discovery order and returns the
// receiver so calls can be chained. Insertion order is meaningful and is
// preserved in the serialized output.
func (p *ProjectModel) AddTable(t Table) *ProjectModel {
	p.Schemas = append(p.Schemas, t)
	return p
}

// AddModel appends a Model to the model in discovery order and returns the
// receiver so calls can be chained. Insertion order is meaningful and is
// preserved in the serialized output.
func (p *ProjectModel) AddModel(m Model) *ProjectModel {
	p.Models = append(p.Models, m)
	return p
}

// AddDisagreement appends a Disagreement finding in the order it was detected
// and returns the receiver so calls can be chained. Insertion order is
// preserved in the serialized output.
func (p *ProjectModel) AddDisagreement(d Disagreement) *ProjectModel {
	p.Disagreements = append(p.Disagreements, d)
	return p
}

// AddRoute appends a Route to the model in discovery order and returns the
// receiver so calls can be chained. Insertion order is meaningful and is
// preserved in the serialized output.
func (p *ProjectModel) AddRoute(r Route) *ProjectModel {
	p.Routes = append(p.Routes, r)
	return p
}

// AddController appends a Controller to the model in discovery order and
// returns the receiver so calls can be chained. Insertion order is meaningful
// and is preserved in the serialized output.
func (p *ProjectModel) AddController(c Controller) *ProjectModel {
	p.Controllers = append(p.Controllers, c)
	return p
}

// AddDeadRoute appends a DeadRoute finding in the order it was detected and
// returns the receiver so calls can be chained. Insertion order is preserved in
// the serialized output.
func (p *ProjectModel) AddDeadRoute(d DeadRoute) *ProjectModel {
	p.DeadRoutes = append(p.DeadRoutes, d)
	return p
}

// AddFormRequest appends a FormRequest to the model in discovery order and
// returns the receiver so calls can be chained. Insertion order is meaningful
// and is preserved in the serialized output.
func (p *ProjectModel) AddFormRequest(f FormRequest) *ProjectModel {
	p.FormRequests = append(p.FormRequests, f)
	return p
}

// AddFinding appends a Finding to the itemized health verdict in the order it
// was emitted and returns the receiver so calls can be chained. The emit order
// is the fixed understand-then-judge order (dead routes → disagreements →
// unguarded) and is preserved in the serialized output.
func (p *ProjectModel) AddFinding(f Finding) *ProjectModel {
	p.Findings = append(p.Findings, f)
	return p
}

// AddMiddleware appends a Middleware node in the extractor's tiered emit order
// (built-in canonical order, then applied-first-appearance) and returns the
// receiver so calls can be chained. Insertion order is meaningful and is
// preserved in the serialized output (ADR 0012's determinism requirement).
func (p *ProjectModel) AddMiddleware(m Middleware) *ProjectModel {
	p.Middlewares = append(p.Middlewares, m)
	return p
}

// ToJSON serializes the ProjectModel to stable, 2-space-indented JSON — the
// unlaravel.json output contract (ADR 0004). The output is deterministic:
// insertion order is preserved and no map is marshaled, so identical models
// always produce byte-identical JSON, which golden-file tests rely on.
func (p *ProjectModel) ToJSON() ([]byte, error) {
	out, err := json.MarshalIndent(p, "", jsonIndent)
	if err != nil {
		return nil, fmt.Errorf("model: serialize project model to JSON: %w", err)
	}
	return out, nil
}
