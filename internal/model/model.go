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
const CurrentSchemaVersion = "1.6.0"

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
}

// New constructs a ProjectModel for the named project, stamping it with the
// CurrentSchemaVersion. Every slice is initialized to a non-nil empty slice so
// an analysis that finds none of a given kind serializes "schemas": [],
// "models": [], "disagreements": [], "routes": [], "controllers": [],
// "dead_routes": [], "form_requests": [], and "findings": [] rather than null.
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
