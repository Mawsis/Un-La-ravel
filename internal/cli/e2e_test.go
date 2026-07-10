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
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/analyze"
	"github.com/Mawsis/Un-La-ravel/internal/detector"
	"github.com/Mawsis/Un-La-ravel/internal/extract/controller"
	formrequestextract "github.com/Mawsis/Un-La-ravel/internal/extract/formrequest"
	middlewareextract "github.com/Mawsis/Un-La-ravel/internal/extract/middleware"
	modelextract "github.com/Mawsis/Un-La-ravel/internal/extract/model"
	routeextract "github.com/Mawsis/Un-La-ravel/internal/extract/route"
	"github.com/Mawsis/Un-La-ravel/internal/extract/schema"
	"github.com/Mawsis/Un-La-ravel/internal/findings"
	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/Mawsis/Un-La-ravel/internal/render/er"
	"github.com/Mawsis/Un-La-ravel/internal/render/openapi"
	"github.com/Mawsis/Un-La-ravel/internal/render/routemap"
	"github.com/Mawsis/Un-La-ravel/internal/symbol"
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
	// goldenOpenAPIRel is the committed OpenAPI 3 spec the renderer produces from
	// the assembled model — the sixth-node showpiece output pinned as a contract.
	goldenOpenAPIRel = "testdata/fixture-app.openapi.json"
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

	gotOpenAPI, err := openapi.Render(pm)
	if err != nil {
		t.Fatalf("render OpenAPI spec: %v", err)
	}
	// Render omits the trailing newline; the golden is stored with one so it is a
	// clean POSIX text file and diffs nicely, mirroring the JSON golden.
	gotOpenAPI = append(gotOpenAPI, '\n')

	assertGolden(t, filepath.Join(root, goldenJSONRel), gotJSON)
	assertGolden(t, filepath.Join(root, goldenMermaidRel), gotMermaid)
	assertGolden(t, filepath.Join(root, goldenRouteMapRel), gotRouteMap)
	assertGolden(t, filepath.Join(root, goldenOpenAPIRel), gotOpenAPI)
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

	wantTables := []string{"users", "posts", "categories", "lenses"}
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
// that the goldens encode: the four Models (in lexical discovery order
// Category, Lens, Post, User) with their relationships, and the two deliberate
// Model↔Schema Disagreements — Post.editor referencing the missing editor_id
// column, and Post.lens targeting Lens's name-mismatched inferred table with
// the issue #36 did-you-mean suggestion. A careless -update that corrupts
// model extraction or the correlation still fails here.
func TestE2E_FixtureApp_EloquentShape(t *testing.T) {
	root := repoRoot(t)
	pm := analyzeFixture(t, filepath.Join(root, fixtureAppRel))

	wantModels := []string{"Category", "Lens", "Post", "User"}
	if got := modelNames(pm); !equalStrings(got, wantModels) {
		t.Fatalf("models = %v, want %v", got, wantModels)
	}

	// Lens's inferred table is the deliberate issue #36 name mismatch: the
	// best-effort inflector reads the trailing "s" as an existing plural, so
	// the model honestly reports "lens" even though the migration created
	// "lenses" (the correction lives in the disagreement, never a rebind).
	lens := findModel(t, pm, "Lens")
	if got, want := lens.Table, "lens"; got != want {
		t.Errorf("Lens table = %q, want %q (honest inferred name)", got, want)
	}

	post := findModel(t, pm, "Post")
	wantRels := []model.Relationship{
		{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "user_id"},
		{Kind: "belongsTo", Method: "category", Target: "Category"},
		{Kind: "belongsTo", Method: "editor", Target: "User", ForeignKey: "editor_id"},
		{Kind: "belongsTo", Method: "lens", Target: "Lens"},
	}
	if got := post.Relationships; !equalRelationships(got, wantRels) {
		t.Errorf("Post relationships = %+v, want %+v", got, wantRels)
	}

	// Two Disagreements, in relationship-declaration order: Post.editor's
	// explicit editor_id FK never created on the posts table, then Post.lens
	// targeting the name-mismatched table with the did-you-mean suggestion.
	if got, want := len(pm.Disagreements), 2; got != want {
		t.Fatalf("disagreements count = %d, want %d: %+v", got, want, pm.Disagreements)
	}
	d := pm.Disagreements[0]
	if d.Model != "Post" || d.Relationship != "editor" || d.Kind != model.DisagreementMissingFKColumn {
		t.Errorf("disagreement = %+v, want Post/editor/%s", d, model.DisagreementMissingFKColumn)
	}
	d = pm.Disagreements[1]
	if d.Model != "Post" || d.Relationship != "lens" || d.Kind != model.DisagreementMissingTable {
		t.Errorf("disagreement = %+v, want Post/lens/%s", d, model.DisagreementMissingTable)
	}
	if !strings.Contains(d.Reason, `did you mean "lenses"? add protected $table = 'lenses' to Lens`) {
		t.Errorf("lens disagreement reason %q lacks the did-you-mean suggestion", d.Reason)
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
	// 12 total: 11 original + the deliberate public write POST /webhooks (issue
	// #50's unauthenticated_write blocker fixture).
	if got, want := len(pm.Routes), 12; got != want {
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
	if !equalStrings(usersIndex.Middleware, []string{"auth:sanctum", "throttle:api", "tenant"}) {
		t.Errorf("GET /admin/users middleware = %v, want [auth:sanctum throttle:api tenant]", usersIndex.Middleware)
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

// TestE2E_FixtureApp_FormRequestShape asserts the FormRequest half of the
// contract the goldens encode (ADR 0006, PRD #5 — the sixth and final MVP node):
// the single extracted FormRequest (StorePostRequest) with its five parsed
// Fields, the source-faithful preservation of the unknown `alpha_dash` rule on
// `tags`, and the Route→FormRequest link that attaches it to POST /posts via
// PostController@store's typed parameter. A careless -update that corrupts the
// rules() parse or the link still fails here.
func TestE2E_FixtureApp_FormRequestShape(t *testing.T) {
	root := repoRoot(t)
	pm := analyzeFixture(t, filepath.Join(root, fixtureAppRel))

	// Exactly one FormRequest, resolved to its conventional FQN.
	if got, want := len(pm.FormRequests), 1; got != want {
		t.Fatalf("form requests count = %d, want %d: %+v", got, want, pm.FormRequests)
	}
	fr := pm.FormRequests[0]
	if fr.Name != "StorePostRequest" || fr.FQN != `App\Http\Requests\StorePostRequest` {
		t.Errorf("form request = %s (%s), want StorePostRequest (App\\Http\\Requests\\StorePostRequest)",
			fr.Name, fr.FQN)
	}

	// Its five fields, in source-declaration order.
	wantFields := []string{"title", "body", "published", "status", "tags"}
	if got := fieldNames(fr); !equalStrings(got, wantFields) {
		t.Fatalf("StorePostRequest fields = %v, want %v", got, wantFields)
	}

	// The unknown/custom rule on `tags` (alpha_dash) must be preserved verbatim,
	// not dropped — the whole point of the unknown-rule fixture.
	tags := findField(t, fr, "tags")
	if got := ruleNames(tags); !equalStrings(got, []string{"nullable", "alpha_dash"}) {
		t.Errorf("tags rules = %v, want [nullable alpha_dash]", got)
	}

	// The Route→FormRequest link: POST /posts (posts.store) carries the request's
	// FQN, resolved through PostController@store's typed parameter (ADR 0006).
	store := findRoute(t, pm, "POST", "/posts")
	if store.FormRequest != `App\Http\Requests\StorePostRequest` {
		t.Errorf("POST /posts form_request = %q, want App\\Http\\Requests\\StorePostRequest",
			store.FormRequest)
	}
}

// TestE2E_FixtureApp_OpenAPIShape asserts the OpenAPI golden's load-bearing
// facts (FACTS BUG 1 + the rules→JSON-Schema mapping): the golden parses as JSON,
// the POST /posts Operation carries a requestBody derived from StorePostRequest,
// `title` gets maxLength 255 emitted as a NUMBER (never the string "255"),
// `status` carries the in:draft,published enum, and the unknown `alpha_dash`
// rule survives in `tags`'s description. It reads the committed golden directly
// so a careless -update that regresses the renderer is caught structurally, not
// only byte-for-byte.
func TestE2E_FixtureApp_OpenAPIShape(t *testing.T) {
	root := repoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, goldenOpenAPIRel))
	if err != nil {
		t.Fatalf("read OpenAPI golden: %v (run with -update to create it)", err)
	}

	// Parse into json.RawMessage-preserving generic maps so we can assert the
	// numeric-vs-string distinction the FACTS demand.
	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			RequestBody *struct {
				Content map[string]struct {
					Schema struct {
						Properties map[string]struct {
							Type        string          `json:"type"`
							MaxLength   json.RawMessage `json:"maxLength"`
							Enum        []string        `json:"enum"`
							Description string          `json:"description"`
						} `json:"properties"`
						Required []string `json:"required"`
					} `json:"schema"`
				} `json:"content"`
			} `json:"requestBody"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("OpenAPI golden is not valid JSON: %v", err)
	}

	if doc.OpenAPI != "3.0.3" {
		t.Errorf("openapi version = %q, want 3.0.3", doc.OpenAPI)
	}

	post, ok := doc.Paths["/posts"]["post"]
	if !ok || post.RequestBody == nil {
		t.Fatalf("POST /posts has no requestBody in the OpenAPI golden")
	}
	media, ok := post.RequestBody.Content["application/json"]
	if !ok {
		t.Fatalf("POST /posts requestBody has no application/json content")
	}
	props := media.Schema.Properties

	// title: string with maxLength emitted as a bare JSON NUMBER, not a string
	// (FACTS BUG 1). "255" (quoted) or absence both fail.
	title, ok := props["title"]
	if !ok {
		t.Fatalf("requestBody schema is missing the title property")
	}
	if got := strings.TrimSpace(string(title.MaxLength)); got != "255" {
		t.Errorf("title maxLength raw JSON = %q, want the number 255 (unquoted)", got)
	}

	// status: the in:draft,published enum survived the mapping.
	if got := props["status"].Enum; !equalStrings(got, []string{"draft", "published"}) {
		t.Errorf("status enum = %v, want [draft published]", got)
	}

	// tags: the unknown alpha_dash rule is preserved in the description, not
	// dropped (FACTS: unknown rule → description; never crash or drop).
	if got, want := props["tags"].Description, "rule: alpha_dash"; got != want {
		t.Errorf("tags description = %q, want %q", got, want)
	}

	// required lists the two required fields, in declaration order.
	if got := media.Schema.Required; !equalStrings(got, []string{"title", "body"}) {
		t.Errorf("requestBody required = %v, want [title body]", got)
	}
}

// analyzeFixture drives the same pipeline analyzeProject runs (detect → extract
// schema → extract models → correlate disagreements → extract routes → extract
// FormRequests → link → assemble model), but headlessly, returning the assembled
// Project Model. It mirrors the production orchestration so the goldens pin what
// the real binary emits.
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

	rp := resolveFixtureRoutes(t, fixtureApp)

	// Extract the FormRequests and link each resolved route to the request body
	// its action validates (ADR 0006), exactly as analyzeProject does — so the
	// goldens pin the sixth node type and the Route.form_request link.
	formRequests := extractFixtureFormRequests(t, fixtureApp)
	routes := analyze.LinkFormRequests(
		rp.routes, formRequests, rp.actionParams, rp.symbols, rp.controllerFiles,
	)

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
	for _, c := range rp.controllers {
		pm.AddController(c)
	}
	for _, r := range routes {
		// Stamp the per-route auth state from the flattened middleware exactly as
		// the engine's buildProjectModel does (issue #50), so the golden pins the
		// same "auth" field the real `unlaravel analyze` emits.
		r.Auth = findings.Classify(r.Middleware)
		pm.AddRoute(r)
	}
	for _, dr := range rp.deadRoutes {
		pm.AddDeadRoute(dr)
	}
	for _, fr := range formRequests {
		pm.AddFormRequest(fr)
	}
	// Assemble the Middleware node set from the fixture's Laravel-10 Kernel and the
	// resolved routes (issues #64/#66, ADR 0012) exactly as the engine's
	// buildProjectModel does, so the golden pins the same "middlewares" array the
	// real `unlaravel analyze` emits — the Kernel-declared aliases (origin "app",
	// resolved) unioned with the built-in backstop and the routes' applied names.
	kernel, err := middlewareextract.ReadKernel(fixtureApp)
	if err != nil {
		t.Fatalf("read HTTP Kernel: %v", err)
	}
	for _, mw := range middlewareextract.Extract(routes, kernel) {
		pm.AddMiddleware(mw)
	}
	// Compute the itemized health verdict last, over the fully assembled model,
	// exactly as the engine's buildProjectModel does (internal/findings), so the
	// golden pins the same findings array the real `unlaravel analyze` emits.
	for _, f := range findings.Verdict(pm) {
		pm.AddFinding(f)
	}
	return pm
}

// fixtureRoutePipeline bundles what the headless Route pipeline produces so the
// FormRequest link (ADR 0006) can run afterwards: the resolved routes, the
// extracted controllers, the dead-route findings, and the three inputs
// LinkFormRequests needs — the action-parameter type-hints, the project-wide
// symbol table, and the controller-FQN → source-file map. It mirrors the
// production routePipeline struct in root.go.
type fixtureRoutePipeline struct {
	routes          []model.Route
	controllers     []model.Controller
	deadRoutes      []model.DeadRoute
	actionParams    controller.ActionParams
	symbols         *symbol.Table
	controllerFiles map[string]string
}

// resolveFixtureRoutes runs the two-phase Route pipeline (ADR 0006) against the
// fixture app the same way the production extractRoutes helper does, but
// headlessly: it extracts the Controllers (with their Actions AND each action's
// parameter type-hints), builds the project-wide symbol table from the
// controller, model, AND FormRequest files, extracts the Routes (groups
// flattened, resource macros expanded), and resolves each Route against the
// controllers + symbol table — filling in resolvable FQNs and surfacing the
// deliberate Dead Route. It also returns the action-parameter type-hints and the
// controller-FQN → file map so the caller can run the FormRequest link. It
// mirrors the CLI orchestration so the goldens pin what the real binary emits.
func resolveFixtureRoutes(t *testing.T, fixtureApp string) fixtureRoutePipeline {
	t.Helper()

	controllersDir := filepath.Join(fixtureApp, "app", "Http", "Controllers")
	controllers, actionParams, err := controller.ExtractDirWithParams(controllersDir)
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

	return fixtureRoutePipeline{
		routes:          resolved,
		controllers:     controllers,
		deadRoutes:      deadRoutes,
		actionParams:    actionParams,
		symbols:         sym,
		controllerFiles: fixtureControllerFileMap(t, controllersDir),
	}
}

// extractFixtureFormRequests extracts the FormRequest nodes from the fixture
// app's app/Http/Requests directory (scanned recursively, since subdirectories
// are namespace segments), mirroring the production extractFormRequests helper.
func extractFixtureFormRequests(t *testing.T, fixtureApp string) []model.FormRequest {
	t.Helper()

	requestsDir := filepath.Join(fixtureApp, "app", "Http", "Requests")
	paths := fixturePHPFiles(t, requestsDir)

	formRequests, err := formrequestextract.Extract(paths)
	if err != nil {
		t.Fatalf("extract FormRequests from %s: %v", requestsDir, err)
	}
	return formRequests
}

// fixtureControllerFileMap builds the controller-FQN → source-file map the
// FormRequest link needs (ADR 0006): a parameter's short type name resolves
// through the `use` imports of the very file that declared its action, so the
// linker must know which file that is. It mirrors the production
// controllerFileMap helper in root.go.
func fixtureControllerFileMap(t *testing.T, controllersDir string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	for _, path := range fixturePHPFiles(t, controllersDir) {
		res, err := phpast.ParseFile(path)
		if err != nil {
			t.Fatalf("parse controller %s for the FQN→file map: %v", path, err)
		}
		namespace := phpast.NamespaceName(res.Root)
		for _, short := range phpast.DeclaredClasses(res.Root) {
			fqn := short
			if namespace != "" {
				fqn = namespace + `\` + short
			}
			files[fqn] = path
		}
	}
	return files
}

// fixtureClassFiles gathers the PHP files whose declared classes seed the symbol
// table: the controller files (scanned recursively, since subdirectories are
// namespace segments) plus the Eloquent model files AND the FormRequest files.
// Including the models mirrors the production symbol-table build so a route
// dispatching to a class outside app/Http/Controllers resolves rather than being
// flagged a false dead route; including the FormRequests lets a controller
// action's request-typed parameter resolve so the Route→FormRequest link is made
// (ADR 0006).
func fixtureClassFiles(t *testing.T, fixtureApp, controllersDir string) []string {
	t.Helper()

	files := fixturePHPFiles(t, controllersDir)

	modelFiles, err := filepath.Glob(filepath.Join(fixtureApp, "app", "Models", "*.php"))
	if err != nil {
		t.Fatalf("glob model files: %v", err)
	}
	files = append(files, modelFiles...)

	files = append(files, fixturePHPFiles(t, filepath.Join(fixtureApp, "app", "Http", "Requests"))...)
	return files
}

// fixturePHPFiles walks dir recursively and returns the paths of every *.php file
// under it. A missing directory yields no files (not a fatal error), so a fixture
// without the directory simply contributes nothing.
func fixturePHPFiles(t *testing.T, dir string) []string {
	t.Helper()

	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}

	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".php" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
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

func fieldNames(fr model.FormRequest) []string {
	names := make([]string, 0, len(fr.Fields))
	for _, f := range fr.Fields {
		names = append(names, f.Name)
	}
	return names
}

func findField(t *testing.T, fr model.FormRequest, name string) model.Field {
	t.Helper()
	for _, f := range fr.Fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("field %q not found on form request %q", name, fr.Name)
	return model.Field{}
}

func ruleNames(f model.Field) []string {
	names := make([]string, 0, len(f.Rules))
	for _, r := range f.Rules {
		names = append(names, r.Name)
	}
	return names
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
