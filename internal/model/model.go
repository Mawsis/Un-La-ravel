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
const CurrentSchemaVersion = "1.0.0"

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
	SchemaVersion  string  `json:"schema_version"`
	ProjectName    string  `json:"project_name"`
	LaravelVersion string  `json:"laravel_version"`
	Schemas        []Table `json:"schemas"`
}

// New constructs a ProjectModel for the named project, stamping it with the
// CurrentSchemaVersion. Schemas is initialized to a non-nil empty slice so an
// analysis that finds no tables serializes "schemas": [] rather than null.
func New(projectName, laravelVersion string) *ProjectModel {
	return &ProjectModel{
		SchemaVersion:  CurrentSchemaVersion,
		ProjectName:    projectName,
		LaravelVersion: laravelVersion,
		Schemas:        []Table{},
	}
}

// AddTable appends a Table to the model in discovery order and returns the
// receiver so calls can be chained. Insertion order is meaningful and is
// preserved in the serialized output.
func (p *ProjectModel) AddTable(t Table) *ProjectModel {
	p.Schemas = append(p.Schemas, t)
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
