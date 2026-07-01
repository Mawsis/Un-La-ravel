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

	"github.com/mawsis/unlaravel/internal/analyze"
	"github.com/mawsis/unlaravel/internal/detector"
	"github.com/mawsis/unlaravel/internal/extract/controller"
	modelextract "github.com/mawsis/unlaravel/internal/extract/model"
	routeextract "github.com/mawsis/unlaravel/internal/extract/route"
	"github.com/mawsis/unlaravel/internal/extract/schema"
	"github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/render/er"
	"github.com/mawsis/unlaravel/internal/render/routemap"
	"github.com/mawsis/unlaravel/internal/symbol"
)

// updateGolden, set by `-update`, rewrites the golden files instead of asserting
// against them. It is opt-in so a normal `go test` run can never mutate them.
var updateGolden = flag.Bool("update", false, "regenerate golden files instead of comparing")

const (
	// fixtureAppRel is the fixture Laravel app, relative to the repo root.
	fixtureAppRel = "testdata/fixture-app"
	// goldenJSONRel / goldenMermaidRel / goldenRouteMapRel are the committed
	// golden outputs.
	goldenJSONRel     = "testdata/fixture-app.golden.json"
	goldenMermaidRel  = "testdata/fixture-app.golden.mermaid"
	goldenRouteMapRel = "testdata/fixture-app.golden.routemap"
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
	gotRouteMap := []byte(routemap.Render(pm))

	assertGolden(t, filepath.Join(root, goldenJSONRel), gotJSON)
	assertGolden(t, filepath.Join(root, goldenMermaidRel), gotMermaid)
	assertGolden(t, filepath.Join(root, goldenRouteMapRel), gotRouteMap)
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

// TestE2E_FixtureApp_EloquentShape asserts the Eloquent half of the contract
// that the goldens encode: the three Models (in lexical discovery order
// Category, Post, User) with their relationships, and the single deliberate
// Model↔Schema Disagreement (Post.editor referencing the missing editor_id
// column). A careless -update that corrupts model extraction or the correlation
// still fails here.
func TestE2E_FixtureApp_EloquentShape(t *testing.T) {
	root := repoRoot(t)
	pm := analyzeFixture(t, filepath.Join(root, fixtureAppRel))

	wantModels := []string{"Category", "Post", "User"}
	if got := modelNames(pm); !equalStrings(got, wantModels) {
		t.Fatalf("models = %v, want %v", got, wantModels)
	}

	post := findModel(t, pm, "Post")
	wantRels := []model.Relationship{
		{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "user_id"},
		{Kind: "belongsTo", Method: "category", Target: "Category"},
		{Kind: "belongsTo", Method: "editor", Target: "User", ForeignKey: "editor_id"},
	}
	if got := post.Relationships; !equalRelationships(got, wantRels) {
		t.Errorf("Post relationships = %+v, want %+v", got, wantRels)
	}

	// Exactly one Disagreement: Post.editor's explicit editor_id FK never
	// created on the posts table.
	if got, want := len(pm.Disagreements), 1; got != want {
		t.Fatalf("disagreements count = %d, want %d: %+v", got, want, pm.Disagreements)
	}
	d := pm.Disagreements[0]
	if d.Model != "Post" || d.Relationship != "editor" || d.Kind != model.DisagreementMissingFKColumn {
		t.Errorf("disagreement = %+v, want Post/editor/%s", d, model.DisagreementMissingFKColumn)
	}
}

// TestE2E_FixtureApp_RouteShape asserts the route/controller half of the
// contract the goldens encode (ADR 0006): the four extracted Controllers with
// their Actions, the eleven Routes with group prefixes applied and the
// apiResource macro expanded, and the single deliberate Dead Route
// (DELETE /admin/users/{id} → UserController@destroy, a missing_action because
// UserController resolves but declares no destroy method). A careless -update
// that corrupts route extraction, symbol resolution, or dead-route detection
// still fails here.
func TestE2E_FixtureApp_RouteShape(t *testing.T) {
	root := repoRoot(t)
	pm := analyzeFixture(t, filepath.Join(root, fixtureAppRel))

	// Four controllers, in recursive-discovery order.
	wantControllers := []string{
		"App\\Http\\Controllers\\CommentController",
		"App\\Http\\Controllers\\Controller",
		"App\\Http\\Controllers\\PostController",
		"App\\Http\\Controllers\\UserController",
	}
	if got := controllerFQNs(pm); !equalStrings(got, wantControllers) {
		t.Fatalf("controller FQNs = %v, want %v", got, wantControllers)
	}

	// UserController declares only index — the absence of destroy is what makes
	// the dead route dead.
	user := findController(t, pm, "App\\Http\\Controllers\\UserController")
	if got := user.Actions; !equalStrings(got, []string{"index"}) {
		t.Errorf("UserController actions = %v, want [index]", got)
	}

	// The apiResource on comments must have expanded to the five REST routes.
	if got, want := len(pm.Routes), 11; got != want {
		t.Fatalf("routes count = %d, want %d", got, want)
	}

	// A grouped route carries its inherited prefix and middleware, and resolves.
	usersIndex := findRoute(t, pm, "GET", "/admin/users")
	if usersIndex.Controller != "UserController" || usersIndex.Action != "index" {
		t.Errorf("GET /admin/users = %s@%s, want UserController@index",
			usersIndex.Controller, usersIndex.Action)
	}
	if usersIndex.FQN != "App\\Http\\Controllers\\UserController" {
		t.Errorf("GET /admin/users FQN = %q, want resolved UserController", usersIndex.FQN)
	}
	if !equalStrings(usersIndex.Middleware, []string{"auth:sanctum", "throttle:api"}) {
		t.Errorf("GET /admin/users middleware = %v, want [auth:sanctum throttle:api]", usersIndex.Middleware)
	}

	// Exactly one dead route: DELETE /admin/users/{id} → UserController@destroy,
	// kind missing_action (the class resolves; the method does not exist).
	if got, want := len(pm.DeadRoutes), 1; got != want {
		t.Fatalf("dead routes count = %d, want %d: %+v", got, want, pm.DeadRoutes)
	}
	dr := pm.DeadRoutes[0]
	if dr.Method != "DELETE" || dr.URI != "/admin/users/{id}" ||
		dr.Action != "destroy" || dr.Kind != model.DeadRouteMissingAction {
		t.Errorf("dead route = %+v, want DELETE /admin/users/{id} destroy %s",
			dr, model.DeadRouteMissingAction)
	}
}

// analyzeFixture drives the same pipeline analyzeProject runs (detect → extract
// schema → extract models → correlate disagreements → assemble model), but
// headlessly, returning the assembled Project Model. It mirrors the production
// orchestration so the goldens pin what the real binary emits.
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

	modelsDir := filepath.Join(fixtureApp, "app", "Models")
	models, err := modelextract.ExtractDir(modelsDir)
	if err != nil {
		t.Fatalf("extract models from %s: %v", modelsDir, err)
	}

	disagreements := analyze.FindDisagreements(models, tables)

	routes, controllers, deadRoutes := resolveFixtureRoutes(t, fixtureApp)

	pm := model.New(project.ComposerAnalysis.ProjectName, project.Version)
	for _, tb := range tables {
		pm.AddTable(tb)
	}
	for _, m := range models {
		pm.AddModel(m)
	}
	for _, d := range disagreements {
		pm.AddDisagreement(d)
	}
	for _, c := range controllers {
		pm.AddController(c)
	}
	for _, r := range routes {
		pm.AddRoute(r)
	}
	for _, dr := range deadRoutes {
		pm.AddDeadRoute(dr)
	}
	return pm
}

// resolveFixtureRoutes runs the two-phase Route pipeline (ADR 0006) against the
// fixture app the same way the production extractRoutes helper does, but
// headlessly: it extracts the Controllers (with their Actions), builds the
// project-wide symbol table from the controller and model files, extracts the
// Routes (groups flattened, resource macros expanded), and resolves each Route
// against the controllers + symbol table — filling in resolvable FQNs and
// surfacing the deliberate Dead Route. It mirrors the CLI orchestration so the
// goldens pin what the real binary emits for routes.
func resolveFixtureRoutes(t *testing.T, fixtureApp string) ([]model.Route, []model.Controller, []model.DeadRoute) {
	t.Helper()

	controllersDir := filepath.Join(fixtureApp, "app", "Http", "Controllers")
	controllers, err := controller.ExtractDir(controllersDir)
	if err != nil {
		t.Fatalf("extract controllers from %s: %v", controllersDir, err)
	}

	routesDir := filepath.Join(fixtureApp, "routes")
	routes, err := routeextract.ExtractDir(routesDir)
	if err != nil {
		t.Fatalf("extract routes from %s: %v", routesDir, err)
	}

	sym, err := symbol.Collect(fixtureClassFiles(t, fixtureApp, controllersDir))
	if err != nil {
		t.Fatalf("build symbol table: %v", err)
	}

	routeFiles, err := filepath.Glob(filepath.Join(routesDir, "*.php"))
	if err != nil {
		t.Fatalf("glob route files in %s: %v", routesDir, err)
	}

	resolved, deadRoutes, err := analyze.ResolveRoutes(routes, controllers, sym, routeFiles)
	if err != nil {
		t.Fatalf("resolve routes: %v", err)
	}
	return resolved, controllers, deadRoutes
}

// fixtureClassFiles gathers the PHP files whose declared classes seed the symbol
// table: the controller files (scanned recursively, since subdirectories are
// namespace segments) plus the Eloquent model files. Including the models mirrors
// the production symbol-table build so a route dispatching to a class outside
// app/Http/Controllers resolves rather than being flagged a false dead route.
func fixtureClassFiles(t *testing.T, fixtureApp, controllersDir string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(controllersDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".php" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk controllers %s: %v", controllersDir, err)
	}

	modelFiles, err := filepath.Glob(filepath.Join(fixtureApp, "app", "Models", "*.php"))
	if err != nil {
		t.Fatalf("glob model files: %v", err)
	}
	files = append(files, modelFiles...)
	return files
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

func modelNames(pm *model.ProjectModel) []string {
	names := make([]string, 0, len(pm.Models))
	for _, m := range pm.Models {
		names = append(names, m.Name)
	}
	return names
}

func findModel(t *testing.T, pm *model.ProjectModel, name string) model.Model {
	t.Helper()
	for _, m := range pm.Models {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("model %q not found in project model", name)
	return model.Model{}
}

func equalRelationships(a, b []model.Relationship) bool {
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

func controllerFQNs(pm *model.ProjectModel) []string {
	names := make([]string, 0, len(pm.Controllers))
	for _, c := range pm.Controllers {
		names = append(names, c.FQN)
	}
	return names
}

func findController(t *testing.T, pm *model.ProjectModel, fqn string) model.Controller {
	t.Helper()
	for _, c := range pm.Controllers {
		if c.FQN == fqn {
			return c
		}
	}
	t.Fatalf("controller %q not found in project model", fqn)
	return model.Controller{}
}

func findRoute(t *testing.T, pm *model.ProjectModel, method, uri string) model.Route {
	t.Helper()
	for _, r := range pm.Routes {
		if r.Method == method && r.URI == uri {
			return r
		}
	}
	t.Fatalf("route %s %s not found in project model", method, uri)
	return model.Route{}
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
