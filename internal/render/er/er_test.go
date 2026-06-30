package er

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mawsis/unlaravel/internal/model"
)

// update regenerates the golden file when the test is run with -update:
//
//	go test ./internal/render/er -run TestRenderGolden -update
//
// The committed golden is the source of truth; -update only exists to
// regenerate it intentionally after a deliberate change to the Renderer.
var update = flag.Bool("update", false, "regenerate golden files")

// goldenPath is the on-disk location of the expected Mermaid output.
const goldenPath = "testdata/golden.mermaid"

// buildFixtureModel constructs a small but representative Project Model that
// exercises the Renderer's externally observable behavior:
//   - a parent table (users) with a primary key,
//   - a child table (posts) with a primary key, a foreign key that references
//     a table present in the model (users) — which must produce a relationship
//     line — and a foreign key that references an absent table (legacy_authors)
//     — which must be silently dropped from the relationships,
//   - a plain non-key column to confirm ordinary attributes render.
//
// It is a pure in-memory fixture: no files, no parser, no I/O.
func buildFixtureModel() *model.ProjectModel {
	users := model.NewTable("users")
	users.Columns = []model.Column{
		{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
		{Name: "name", Type: "string"},
	}

	posts := model.NewTable("posts")
	posts.Columns = []model.Column{
		{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
		{
			Name:         "author_id",
			Type:         "bigInteger",
			IsForeignKey: true,
			References:   &model.ForeignKeyRef{Table: "users", Column: "id"},
		},
		{
			// References a table that is NOT in the model; the Renderer must
			// skip the relationship line for it (no dangling Mermaid edge).
			Name:         "legacy_author_id",
			Type:         "bigInteger",
			IsForeignKey: true,
			References:   &model.ForeignKeyRef{Table: "legacy_authors", Column: "id"},
		},
		{Name: "title", Type: "string"},
	}

	return model.New("blog", "11.x").
		AddTable(users).
		AddTable(posts)
}

// TestRenderGolden asserts external behavior: a fixed Project Model renders to
// exactly the bytes stored in the golden file. This pins the Mermaid output
// contract so any unintended change to the Renderer is caught as a diff.
func TestRenderGolden(t *testing.T) {
	got := Render(buildFixtureModel())

	if *update {
		writeGolden(t, got)
		t.Logf("updated golden file: %s", goldenPath)
	}

	want := readGolden(t)
	if got != want {
		t.Errorf("rendered Mermaid does not match golden file.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// readGolden loads the expected output, failing the test with a clear,
// actionable message if it is missing (it must be regenerated with -update).
func readGolden(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with -update to generate it)", goldenPath, err)
	}
	return string(data)
}

// writeGolden persists the rendered output as the new golden, creating the
// testdata directory if needed.
func writeGolden(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatalf("create golden dir: %v", err)
	}
	if err := os.WriteFile(goldenPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write golden file %s: %v", goldenPath, err)
	}
}
