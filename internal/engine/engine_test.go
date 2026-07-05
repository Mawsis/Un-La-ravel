// Package engine_test exercises the Analyze function — the full analysis pipeline
// extracted from the CLI — against the committed fixture app and against
// non-Laravel directories to confirm error paths.
//
// These tests prove that the extracted engine produces the same node counts that
// the CLI e2e tests pin against fixture-app.golden.json, so both consumers
// (CLI and web server) share identical pipeline behaviour.
package engine_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// repoRoot walks up from this file's directory until it finds go.mod, returning
// the module root so testdata paths resolve regardless of which directory
// `go test` is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	// __FILE__ gives us the source file location at compile time.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(thisFile)
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

// TestAnalyze_FixtureApp_NodeCounts verifies that Analyze returns a ProjectModel
// whose node counts match the fixture-app.golden.json contract exactly.
// Any regression in the pipeline's detect → schema → models → disagreements →
// routes (two-phase) → formrequests → link → build chain is caught here.
func TestAnalyze_FixtureApp_NodeCounts(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q) returned unexpected error: %v", fixtureApp, err)
	}
	if pm == nil {
		t.Fatal("engine.Analyze returned nil model without error")
	}

	// schema_version must be the current contract version.
	if pm.SchemaVersion == "" {
		t.Error("ProjectModel.SchemaVersion is empty")
	}

	// From fixture-app.golden.json:
	//   "schemas": 4 tables  (users, posts, categories, lenses)
	//   "models": 4 models   (Category, Lens, Post, User)
	//   "routes": 11 routes
	//   "controllers": 4 controllers
	//   "dead_routes": 1
	//   "form_requests": 1
	//   "disagreements": 2 (Post.editor missing_fk_column; Post.lens
	//     missing_table with the issue #36 did-you-mean suggestion)
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"schemas (tables)", len(pm.Schemas), 4},
		{"models", len(pm.Models), 4},
		{"routes", len(pm.Routes), 11},
		{"controllers", len(pm.Controllers), 4},
		{"dead_routes", len(pm.DeadRoutes), 1},
		{"form_requests", len(pm.FormRequests), 1},
		{"disagreements", len(pm.Disagreements), 2},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("count = %d, want %d", tc.got, tc.want)
			}
		})
	}
}

// TestAnalyze_FixtureApp_Findings verifies the engine computes the itemized
// health verdict (internal/findings) into ProjectModel.Findings, in the fixed
// understand-then-judge order. The fixture app carries every category — 1 dead
// route, 2 disagreements (Post.editor's missing FK column and Post.lens's
// name-mismatched table, issue #36), 1 unguarded model (a Model that wrote
// `protected $guarded = []`) — so all three findings fire, proving the engine
// runs Verdict last, over the fully assembled model.
func TestAnalyze_FixtureApp_Findings(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q): %v", fixtureApp, err)
	}

	want := []model.Finding{
		{Kind: model.FindingDeadRoutes, Count: 1, Label: "1 dead route", View: "findings"},
		{Kind: model.FindingDisagreements, Count: 2, Label: "2 disagreements", View: "findings"},
		{Kind: model.FindingUnguarded, Count: 1, Label: "1 unguarded model", View: "findings"},
	}
	if len(pm.Findings) != len(want) {
		t.Fatalf("Findings = %+v, want %+v", pm.Findings, want)
	}
	for i := range want {
		if pm.Findings[i] != want[i] {
			t.Errorf("Findings[%d] = %+v, want %+v", i, pm.Findings[i], want[i])
		}
	}
}

// TestAnalyze_FixtureApp_ProjectMeta verifies project-level metadata so the
// detector → model wiring is confirmed alongside the extraction counts.
func TestAnalyze_FixtureApp_ProjectMeta(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q): %v", fixtureApp, err)
	}

	if got, want := pm.ProjectName, "acme/blog"; got != want {
		t.Errorf("ProjectName = %q, want %q", got, want)
	}
	if got, want := pm.LaravelVersion, "^11.0"; got != want {
		t.Errorf("LaravelVersion = %q, want %q", got, want)
	}
}

// TestAnalyze_FixtureApp_FormRequestLinked verifies the Route→FormRequest link
// (ADR 0006) is present on POST /posts, confirming that the two-phase route
// pipeline and the formrequest linker are correctly wired in the engine.
func TestAnalyze_FixtureApp_FormRequestLinked(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q): %v", fixtureApp, err)
	}

	var storeRoute string
	for _, r := range pm.Routes {
		if r.Method == "POST" && r.URI == "/posts" {
			storeRoute = r.FormRequest
			break
		}
	}
	const wantFQN = `App\Http\Requests\StorePostRequest`
	if storeRoute != wantFQN {
		t.Errorf("POST /posts form_request = %q, want %q", storeRoute, wantFQN)
	}
}

// TestAnalyze_NonLaravelDir verifies that Analyze returns a non-nil error (and
// no panic) when the target directory exists but is not a Laravel project.
func TestAnalyze_NonLaravelDir(t *testing.T) {
	// t.TempDir() creates a real, empty directory — definitely not a Laravel project.
	notLaravel := t.TempDir()

	pm, err := engine.Analyze(notLaravel)
	if err == nil {
		t.Errorf("expected error for non-Laravel directory %q, got nil (model=%+v)", notLaravel, pm)
	}
	// The model should be nil on error.
	if pm != nil {
		t.Errorf("expected nil model on error, got %+v", pm)
	}
}

// TestAnalyze_NonExistentPath verifies that Analyze returns a non-nil error and
// does not panic when given a path that does not exist on disk.
func TestAnalyze_NonExistentPath(t *testing.T) {
	// Construct a path that is virtually guaranteed not to exist.
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist", "at-all")

	pm, err := engine.Analyze(nonExistent)
	if err == nil {
		t.Errorf("expected error for non-existent path %q, got nil (model=%+v)", nonExistent, pm)
	}
	if pm != nil {
		t.Errorf("expected nil model on error, got %+v", pm)
	}
}

// TestAnalyze_EmptyStringPath verifies that Analyze returns an error (not a
// panic) when called with an empty string, which is neither a directory nor a
// valid Laravel project path.
func TestAnalyze_EmptyStringPath(t *testing.T) {
	pm, err := engine.Analyze("")
	if err == nil {
		t.Errorf("expected error for empty path, got nil (model=%+v)", pm)
	}
	if pm != nil {
		t.Errorf("expected nil model on error, got %+v", pm)
	}
}

// TestAnalyze_MinimalLaravelProject verifies that Analyze succeeds on a
// bare-minimum Laravel project (artisan + composer.json with laravel/framework)
// that has no migrations, models, routes, or requests directories.  This
// exercises the "missing optional directories → empty results" paths inside
// extractSchema, extractModels, extractRoutes, and extractFormRequests, and
// also exercises the projectName fallback that uses the directory name when the
// composer.json carries no "name" field.
func TestAnalyze_MinimalLaravelProject(t *testing.T) {
	dir := t.TempDir()

	// artisan file is required by the detector.
	if err := os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/usr/bin/env php\n"), 0o644); err != nil {
		t.Fatalf("create artisan: %v", err)
	}
	// composer.json with laravel/framework but deliberately no "name" field, so
	// projectName falls back to the directory base name.
	composerJSON := `{"require":{"laravel/framework":"^10.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "composer.json"), []byte(composerJSON), 0o644); err != nil {
		t.Fatalf("create composer.json: %v", err)
	}

	pm, err := engine.Analyze(dir)
	if err != nil {
		t.Fatalf("engine.Analyze on minimal project: %v", err)
	}
	if pm == nil {
		t.Fatal("engine.Analyze returned nil model for minimal project")
	}

	// All node arrays should be initialized (non-nil) but empty — never null in
	// the JSON output.
	if pm.Schemas == nil {
		t.Error("Schemas slice is nil, want empty non-nil slice")
	}
	if pm.Models == nil {
		t.Error("Models slice is nil, want empty non-nil slice")
	}
	if pm.Routes == nil {
		t.Error("Routes slice is nil, want empty non-nil slice")
	}
	if pm.FormRequests == nil {
		t.Error("FormRequests slice is nil, want empty non-nil slice")
	}
	if len(pm.Schemas) != 0 {
		t.Errorf("expected 0 schemas, got %d", len(pm.Schemas))
	}
	if len(pm.Routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(pm.Routes))
	}

	// The laravel version should be picked up from composer.json.
	if pm.LaravelVersion == "" {
		t.Error("LaravelVersion is empty, want non-empty from composer.json require")
	}

	// ProjectName should be the directory base name (fallback path) since the
	// composer.json we wrote has no "name" field.
	if pm.ProjectName == "" {
		t.Error("ProjectName is empty, expected directory base name fallback")
	}
}
