package er

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
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
// exercises the Renderer's externally observable behavior across both the
// schema (FK) view and the Eloquent (Model) view:
//
// Schema view:
//   - a parent table (users) with a primary key,
//   - a child table (posts) with a primary key, a foreign key that references
//     a table present in the model (users) — which must produce a relationship
//     line — and a foreign key that references an absent table (legacy_authors)
//     — which must be silently dropped from the relationships. posts.author_id
//     also carries a single-column unique index, proving FK precedence over UK:
//     the attribute line must still say FK, never UK.
//   - a join table (roles) so a belongsToMany endpoint can resolve, extended
//     with three index scenarios (see roles.Indexes below) that exercise the
//     UK marker: a single-column unique index (must mark that column UK), a
//     composite two-column unique index (must mark neither column), and the
//     posts.author_id FK+unique overlap described above.
//   - a plain non-key column to confirm ordinary attributes render.
//
// Eloquent view (the Models slice):
//   - User hasMany Post via posts(): a "one" → "many" edge (||--o{),
//   - User belongsToMany Role via roles(): a "many" ⇄ "many" edge (}o--o{),
//   - Post belongsTo User via author(): a "many" → "one" edge (}o--||),
//   - Post hasOne PinnedComment via pinned(): the target Model is NOT extracted,
//     so the Renderer must silently drop this edge — mirroring how the FK to an
//     absent table is dropped. This proves Eloquent edges to unknown Models do
//     not produce dangling Mermaid lines.
//
// The FK edge USERS→POSTS and the Eloquent hasMany edge USERS→POSTS coexist by
// design (schema view vs. application view) and are NOT deduplicated, because
// their labels differ ("references (author_id)" vs. "posts (hasMany)").
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
			// Also carries a single-column unique index (see posts.Indexes
			// below): proves FK precedence over UK, since the attribute line
			// must still render FK, never UK.
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
	posts.Indexes = []model.Index{
		{
			// Scenario (c): a unique index whose single column is ALSO the FK
			// column. FK precedence must win: author_id renders FK, not UK.
			Name:    "posts_author_id_unique",
			Columns: []string{"author_id"},
			Unique:  true,
		},
	}

	roles := model.NewTable("roles")
	roles.Columns = []model.Column{
		{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
		{Name: "name", Type: "string"},
		{Name: "slug", Type: "string"},
		{Name: "tenant_id", Type: "bigInteger"},
		{Name: "code", Type: "string"},
	}
	roles.Indexes = []model.Index{
		{
			// Scenario (a): a single-column unique index on a plain
			// (non-PK, non-FK) column. Expect a UK marker on exactly this
			// column's attribute line.
			Name:    "roles_slug_unique",
			Columns: []string{"slug"},
			Unique:  true,
		},
		{
			// Scenario (b): a composite unique index across two columns.
			// Mermaid has no first-class syntax for a composite index, so
			// neither column must gain any marker.
			Name:    "roles_tenant_id_code_unique",
			Columns: []string{"tenant_id", "code"},
			Unique:  true,
		},
	}

	userModel := model.NewModel("User")
	userModel.Table = "users"
	userModel.Relationships = []model.Relationship{
		{Kind: "hasMany", Method: "posts", Target: "Post"},
		{Kind: "belongsToMany", Method: "roles", Target: "Role"},
	}

	postModel := model.NewModel("Post")
	postModel.Table = "posts"
	postModel.Relationships = []model.Relationship{
		{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "author_id"},
		{
			// Targets a Model the analysis did not extract; the Renderer must
			// skip the edge (no dangling Mermaid line).
			Kind:   "hasOne",
			Method: "pinned",
			Target: "PinnedComment",
		},
	}

	roleModel := model.NewModel("Role")
	roleModel.Table = "roles"

	return model.New("blog", "11.x").
		AddTable(users).
		AddTable(posts).
		AddTable(roles).
		AddModel(userModel).
		AddModel(postModel).
		AddModel(roleModel)
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

// TestRenderEloquentRelationshipLines pins the exact Mermaid relationship lines
// the Eloquent (Model) view must emit, independently of the golden file. It
// guards the cardinality→kind mapping and label format so a careless golden
// regeneration cannot silently accept a wrong arrow direction or label, and it
// asserts both the additive coexistence of the FK and Eloquent USERS→POSTS edges
// and the silent dropping of an edge whose target Model was not extracted.
func TestRenderEloquentRelationshipLines(t *testing.T) {
	got := Render(buildFixtureModel())

	mustContain := []string{
		// FK (schema) view and Eloquent (application) view coexist for the same
		// table pair because their labels differ.
		`USERS ||--o{ POSTS : "references (author_id)"`,
		`USERS ||--o{ POSTS : "posts (hasMany)"`,
		// belongsToMany: zero-or-more on both endpoints.
		`USERS }o--o{ ROLES : "roles (belongsToMany)"`,
		// belongsTo: the "many" child points at the "one" parent.
		`POSTS }o--|| USERS : "author (belongsTo)"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing expected relationship line:\n\t%s\n--- got ---\n%s", want, got)
		}
	}

	// The hasOne edge targets a Model that was not extracted (PinnedComment), so
	// no line for it must appear — neither by method name nor by kind.
	mustNotContain := []string{
		"pinned",
		"hasOne",
		"PINNED_COMMENT",
	}
	for _, bad := range mustNotContain {
		if strings.Contains(got, bad) {
			t.Errorf("rendered output unexpectedly contains dropped-edge token %q:\n%s", bad, got)
		}
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
