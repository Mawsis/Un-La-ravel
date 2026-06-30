package cli

// End-to-end / integration test (PRD user story 25): it exercises the WHOLE
// analysis chain — detect → parse → extract → model → JSON → render — against a
// real fixture Laravel app on disk and pins the two public outputs (the
// unlaravel.json contract, ADR 0004, and the Mermaid ER diagram) to committed
// golden files. If any link in the chain regresses, the golden comparison
// fails, so this single test guards the seam the rest of the tool is built on.
//
// Regenerate the goldens after an intentional output change with:
//
//	go test ./internal/cli -run TestE2E -update
//
// Review the resulting diff carefully — the goldens ARE the contract.

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mawsis/unlaravel/internal/detector"
	"github.com/mawsis/unlaravel/internal/extract/schema"
	"github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/render/er"
)

// updateGolden, set by `-update`, rewrites the golden files instead of asserting
// against them. It is opt-in so a normal `go test` run can never mutate them.
var updateGolden = flag.Bool("update", false, "regenerate golden files instead of comparing")

const (
	// fixtureAppRel is the fixture Laravel app, relative to the repo root.
	fixtureAppRel = "testdata/fixture-app"
	// goldenJSONRel / goldenMermaidRel are the committed golden outputs.
	goldenJSONRel    = "testdata/fixture-app.golden.json"
	goldenMermaidRel = "testdata/fixture-app.golden.mermaid"
)

// TestE2E_FixtureApp_Pipeline runs the full pipeline against the fixture app and
// asserts both public outputs match their golden files byte-for-byte.
func TestE2E_FixtureApp_Pipeline(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	pm := analyzeFixture(t, fixtureApp)

	gotJSON, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("serialize project model to JSON: %v", err)
	}
	// ToJSON omits the trailing newline; goldens are stored with one so they are
	// clean POSIX text files and diff nicely.
	gotJSON = append(gotJSON, '\n')

	gotMermaid := []byte(er.Render(pm))

	assertGolden(t, filepath.Join(root, goldenJSONRel), gotJSON)
	assertGolden(t, filepath.Join(root, goldenMermaidRel), gotMermaid)
}

// TestE2E_FixtureApp_ModelShape asserts the structural facts that make the
// golden files meaningful, so a careless `-update` that silently corrupts the
// extraction still fails here. It checks the canonical Schema set (users, posts,
// categories) and that the cross-file ALTER merged category_id into posts.
func TestE2E_FixtureApp_ModelShape(t *testing.T) {
	root := repoRoot(t)
	pm := analyzeFixture(t, filepath.Join(root, fixtureAppRel))

	if got, want := pm.ProjectName, "acme/blog"; got != want {
		t.Errorf("project name = %q, want %q", got, want)
	}
	// The detector surfaces the raw composer constraint for laravel/framework
	// ("^11.0" here); it does not normalize to a bare major. This pins the
	// detector→model wiring exactly as the existing (out-of-scope) detector
	// behaves, so the e2e test documents the real contract rather than an
	// idealized one.
	if got, want := pm.LaravelVersion, "^11.0"; got != want {
		t.Errorf("laravel version = %q, want %q", got, want)
	}

	wantTables := []string{"users", "posts", "categories"}
	gotTables := tableNames(pm)
	if !equalStrings(gotTables, wantTables) {
		t.Fatalf("tables = %v, want %v", gotTables, wantTables)
	}

	posts := findTable(t, pm, "posts")
	// The ALTER migration (add_category_id_to_posts_table) must have merged
	// category_id onto the end of the existing posts table.
	wantPostCols := []string{
		"id", "user_id", "title", "body", "published",
		"created_at", "updated_at", "category_id",
	}
	if got := columnNames(posts); !equalStrings(got, wantPostCols) {
		t.Errorf("posts columns = %v, want %v", got, wantPostCols)
	}
	if last := posts.Columns[len(posts.Columns)-1]; last.Name != "category_id" {
		t.Errorf("expected category_id merged last onto posts, got %q", last.Name)
	}
}

// analyzeFixture drives the same pipeline analyzeProject runs (detect → extract →
// assemble model), but headlessly, returning the assembled Project Model.
func analyzeFixture(t *testing.T, fixtureApp string) *model.ProjectModel {
	t.Helper()

	project, err := detector.DetectLaravel(fixtureApp)
	if err != nil {
		t.Fatalf("detect Laravel project at %s: %v", fixtureApp, err)
	}

	migrationsDir := filepath.Join(fixtureApp, "database", "migrations")
	tables, err := schema.ExtractDir(migrationsDir)
	if err != nil {
		t.Fatalf("extract schema from %s: %v", migrationsDir, err)
	}

	pm := model.New(project.ComposerAnalysis.ProjectName, project.Version)
	for _, tb := range tables {
		pm.AddTable(tb)
	}
	return pm
}

// assertGolden compares got against the file at path. With -update it writes got
// to path and reports success; otherwise it fails on any byte difference.
func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()

	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		t.Logf("updated golden %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create it)", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden %s\n--- got ---\n%s\n--- want ---\n%s",
			path, got, want)
	}
}

// repoRoot walks up from the test's working directory until it finds go.mod,
// returning the module root so testdata paths resolve no matter which package
// directory `go test` is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod above %s", dir)
		}
		dir = parent
	}
}

func tableNames(pm *model.ProjectModel) []string {
	names := make([]string, 0, len(pm.Schemas))
	for _, t := range pm.Schemas {
		names = append(names, t.Name)
	}
	return names
}

func columnNames(t model.Table) []string {
	names := make([]string, 0, len(t.Columns))
	for _, c := range t.Columns {
		names = append(names, c.Name)
	}
	return names
}

func findTable(t *testing.T, pm *model.ProjectModel, name string) model.Table {
	t.Helper()
	for _, tb := range pm.Schemas {
		if tb.Name == name {
			return tb
		}
	}
	t.Fatalf("table %q not found in model", name)
	return model.Table{}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
