package model

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// buildKnownModel constructs a fixed ProjectModel used by the golden-file test.
// It exercises every interesting JSON shape: a stamped schema version, a table
// with a primary-key column and a nullable foreign-key column carrying a
// References target, and a second table with no columns (to prove empty
// collections serialize as [] rather than null).
func buildKnownModel() *ProjectModel {
	users := NewTable("users")
	users.Columns = append(users.Columns,
		Column{
			Name:         "id",
			Type:         "bigInteger",
			Nullable:     false,
			IsPrimaryKey: true,
			IsForeignKey: false,
		},
		Column{
			Name:         "team_id",
			Type:         "bigInteger",
			Nullable:     true,
			IsPrimaryKey: false,
			IsForeignKey: true,
			References:   &ForeignKeyRef{Table: "teams", Column: "id"},
		},
	)

	pm := New("blog", "11.x")
	pm.AddTable(users)
	pm.AddTable(NewTable("posts"))
	return pm
}

// TestNewStampsSchemaVersion verifies the constructor sets the versioned
// contract field (ADR 0004) to CurrentSchemaVersion without the caller doing
// anything, and initializes Schemas to a non-nil empty slice.
func TestNewStampsSchemaVersion(t *testing.T) {
	pm := New("blog", "11.x")

	if pm.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("New() SchemaVersion = %q, want %q", pm.SchemaVersion, CurrentSchemaVersion)
	}
	if pm.Schemas == nil {
		t.Error("New() Schemas is nil, want non-nil empty slice")
	}
	if len(pm.Schemas) != 0 {
		t.Errorf("New() Schemas has %d entries, want 0", len(pm.Schemas))
	}
}

// TestNewTableHasNonNilColumns verifies NewTable initializes a non-nil empty
// Columns slice so a column-less table serializes "columns": [] not null.
func TestNewTableHasNonNilColumns(t *testing.T) {
	tbl := NewTable("posts")

	if tbl.Name != "posts" {
		t.Errorf("NewTable() Name = %q, want %q", tbl.Name, "posts")
	}
	if tbl.Columns == nil {
		t.Error("NewTable() Columns is nil, want non-nil empty slice")
	}
}

// TestAddTablePreservesDiscoveryOrder verifies AddTable appends in call order
// and returns the receiver for chaining.
func TestAddTablePreservesDiscoveryOrder(t *testing.T) {
	pm := New("blog", "11.x")

	got := pm.AddTable(NewTable("first")).AddTable(NewTable("second"))

	if got != pm {
		t.Error("AddTable() did not return the receiver for chaining")
	}
	if len(pm.Schemas) != 2 {
		t.Fatalf("AddTable() produced %d schemas, want 2", len(pm.Schemas))
	}
	if pm.Schemas[0].Name != "first" || pm.Schemas[1].Name != "second" {
		t.Errorf("AddTable() order = [%q, %q], want [first, second]",
			pm.Schemas[0].Name, pm.Schemas[1].Name)
	}
}

// TestToJSONMatchesGolden serializes the known model and asserts it is
// byte-for-byte equal to internal/model/testdata/golden.json (trailing
// whitespace ignored). This locks the public output contract (ADR 0004),
// including snake_case keys, struct field ordering, and the omitempty behavior
// of references.
func TestToJSONMatchesGolden(t *testing.T) {
	pm := buildKnownModel()

	got, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() returned error: %v", err)
	}

	wantRaw, err := os.ReadFile(filepath.Join("testdata", "golden.json"))
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	want := bytes.TrimRight(wantRaw, "\n")
	got = bytes.TrimRight(got, "\n")

	if !bytes.Equal(got, want) {
		t.Errorf("ToJSON() did not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestToJSONIsDeterministic verifies repeated serialization of the same model
// yields byte-identical output, which golden-file consumers rely on (no maps,
// insertion order preserved).
func TestToJSONIsDeterministic(t *testing.T) {
	pm := buildKnownModel()

	first, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() first call error: %v", err)
	}
	second, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() second call error: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("ToJSON() is not deterministic: two calls produced different bytes")
	}
}

// TestToJSONOmitsReferencesWhenAbsent verifies a non-foreign-key column has no
// "references" key in the serialized output (omitempty on a nil pointer).
func TestToJSONOmitsReferencesWhenAbsent(t *testing.T) {
	tbl := NewTable("users")
	tbl.Columns = append(tbl.Columns, Column{Name: "id", Type: "bigInteger", IsPrimaryKey: true})
	pm := New("blog", "11.x").AddTable(tbl)

	out, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	if bytes.Contains(out, []byte("references")) {
		t.Errorf("ToJSON() emitted a references key for a non-FK column:\n%s", out)
	}

	// And the JSON must still be valid and round-trippable.
	var rt ProjectModel
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("serialized output is not valid JSON: %v", err)
	}
}

// TestSchemaVersionPresentInJSON verifies the schema_version key is present and
// carries CurrentSchemaVersion in the serialized contract.
func TestSchemaVersionPresentInJSON(t *testing.T) {
	out, err := New("blog", "11.x").ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	var decoded struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decoding serialized output: %v", err)
	}
	if decoded.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("serialized schema_version = %q, want %q", decoded.SchemaVersion, CurrentSchemaVersion)
	}
}
