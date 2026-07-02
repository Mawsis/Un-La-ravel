package cli

// This file defines the --json projections for the view subcommands
// (ADR 0008): explicit structs, never bare maps, so encoding/json's struct
// field order keeps every projection deterministic — the same guarantee
// internal/model relies on for the unlaravel.json contract itself. Each
// projection passes SchemaVersion through from the analyzed model rather than
// inventing its own versioning, since it is a read-only view over that same
// contract.

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// jsonIndent matches internal/model's own indentation so --json output looks
// consistent with the unlaravel.json contract written by --output.
const jsonIndent = "  "

// routesJSON is the `unlaravel routes --json` projection.
type routesJSON struct {
	SchemaVersion string            `json:"schema_version"`
	Routes        []model.Route     `json:"routes"`
	DeadRoutes    []model.DeadRoute `json:"dead_routes"`
}

// modelsJSON is the `unlaravel models --json` projection.
type modelsJSON struct {
	SchemaVersion string        `json:"schema_version"`
	Models        []model.Model `json:"models"`
}

// erJSON is the `unlaravel er --json` projection: the same Mermaid source the
// plain-mode command prints, wrapped with the schema version so a consumer
// doesn't have to re-run analyze to know which contract produced it.
type erJSON struct {
	SchemaVersion string `json:"schema_version"`
	Mermaid       string `json:"mermaid"`
}

// findingsJSON is the `unlaravel findings --json` projection.
type findingsJSON struct {
	SchemaVersion string               `json:"schema_version"`
	Disagreements []model.Disagreement `json:"disagreements"`
	DeadRoutes    []model.DeadRoute    `json:"dead_routes"`
}

// emitJSON marshals v as indented JSON and writes it to w, followed by a
// trailing newline so piped output is a clean POSIX text stream. It is the
// single write to w in --json mode — no other output may precede or follow it
// on the same stream (ADR 0008: --json mode owns stdout exclusively).
func emitJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", jsonIndent)
	if err != nil {
		return fmt.Errorf("failed to serialize JSON output: %w", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write JSON output: %w", err)
	}
	return nil
}
