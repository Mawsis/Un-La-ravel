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
	//   "routes": 14 routes (12 prior + the two sub-namespaced admin/dashboard
	//     routes added for issue #63, which resolve to
	//     App\Http\Controllers\Admin\AdminDashboardController — inline-FQN and
	//     imported-short — and are NOT dead)
	//   "controllers": 5 controllers (4 prior + Admin\AdminDashboardController)
	//   "dead_routes": 1 (still only the deliberate UserController@destroy
	//     missing_action; the admin routes resolve cleanly, proving #63's fix)
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
		{"routes", len(pm.Routes), 14},
		{"controllers", len(pm.Controllers), 5},
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
// `protected $guarded = []`), 1 unauthenticated write (the public POST /webhooks
// added for issue #50) and 3 unauthenticated reads (GET /posts, GET /posts/{id},
// GET /legacy — the middleware-less reads) — so every category fires, proving the
// engine runs Verdict last, over the fully assembled model.
func TestAnalyze_FixtureApp_Findings(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q): %v", fixtureApp, err)
	}

	// Severity is derived through the single model.SeverityFor table — the same
	// source the producer uses — so this expectation can't drift from the mapping
	// (dead routes/disagreements/unauthenticated reads → warn, unguarded and
	// unauthenticated writes → blocker). Auth findings link to the auth view.
	want := []model.Finding{
		{Kind: model.FindingDeadRoutes, Severity: model.SeverityFor(model.FindingDeadRoutes), Count: 1, Label: "1 dead route", View: "findings"},
		{Kind: model.FindingDisagreements, Severity: model.SeverityFor(model.FindingDisagreements), Count: 2, Label: "2 disagreements", View: "findings"},
		{Kind: model.FindingUnguarded, Severity: model.SeverityFor(model.FindingUnguarded), Count: 1, Label: "1 unguarded model", View: "findings"},
		{Kind: model.FindingUnauthenticatedWrite, Severity: model.SeverityFor(model.FindingUnauthenticatedWrite), Count: 1, Label: "1 unauthenticated write route", View: "auth"},
		{Kind: model.FindingUnauthenticatedRead, Severity: model.SeverityFor(model.FindingUnauthenticatedRead), Count: 3, Label: "3 unauthenticated read routes", View: "auth"},
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

// TestAnalyze_FixtureApp_RouteAuthStamped verifies the engine stamps each Route's
// Auth field from its flattened middleware (issue #50) — the per-route auth state
// the contract carries so the dashboard reads coverage rather than re-classifying
// (ADR 0008). The fixture's `auth`/`auth:sanctum` routes are authenticated and its
// middleware-less reads are unauthenticated; asserting a few known routes proves
// the classifier runs over the assembled routes.
func TestAnalyze_FixtureApp_RouteAuthStamped(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q): %v", fixtureApp, err)
	}

	// key a route by "METHOD URI" to look up its stamped auth state.
	auth := map[string]string{}
	for _, r := range pm.Routes {
		auth[r.Method+" "+r.URI] = r.Auth
	}

	want := map[string]string{
		"GET /posts":           model.AuthUnauthenticated, // no middleware
		"POST /posts":          model.AuthAuthenticated,   // ->middleware('auth')
		"GET /admin/users":     model.AuthAuthenticated,   // group auth:sanctum
		"POST /admin/comments": model.AuthAuthenticated,   // group auth:sanctum
	}
	for route, wantAuth := range want {
		if got := auth[route]; got != wantAuth {
			t.Errorf("route %q auth = %q, want %q", route, got, wantAuth)
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

// TestAnalyze_FixtureApp_Middlewares verifies the engine assembles the
// Middleware node set from the fixture's Laravel-10 app/Http/Kernel.php (issues
// #64/#66, ADR 0012): the three-tier union in tiered emit order. The fixture
// Kernel declares six aliases (auth, auth.basic, guest, throttle, verified,
// tenant), so those emit FIRST as origin "app" in declaration order, each with a
// resolved class; then the built-in backstop for the aliases the Kernel did NOT
// declare, origin "framework"; then no applied-unknown tier at all, because the
// only applied name that used to dangle — `tenant` — is now Kernel-declared. This
// proves the app tier sorts first, absorbs declared built-ins (auth, throttle
// appear once, in the app tier), resolves class/groups/global/priority, and that
// a name the Kernel declares no longer falls through to the unknown tier.
func TestAnalyze_FixtureApp_Middlewares(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q): %v", fixtureApp, err)
	}

	// Tier 1: the Kernel-declared aliases, in the Kernel's declaration order.
	kernelTier := []string{"auth", "auth.basic", "guest", "throttle", "verified", "tenant"}
	// Tier 2: the built-in backstop MINUS the aliases the Kernel already declared.
	kernelSet := map[string]bool{}
	for _, a := range kernelTier {
		kernelSet[a] = true
	}
	var frameworkTier []string
	for _, a := range model.BuiltinMiddlewareAliases {
		if !kernelSet[a] {
			frameworkTier = append(frameworkTier, a)
		}
	}
	// Tier 3 is empty: `tenant` is now Kernel-declared, so nothing dangles.
	wantAliases := append(append([]string{}, kernelTier...), frameworkTier...)

	if len(pm.Middlewares) != len(wantAliases) {
		t.Fatalf("Middlewares = %d nodes, want %d\ngot: %+v", len(pm.Middlewares), len(wantAliases), pm.Middlewares)
	}
	for i, wantAlias := range wantAliases {
		if pm.Middlewares[i].Alias != wantAlias {
			t.Errorf("Middlewares[%d].Alias = %q, want %q (tiered emit order)", i, pm.Middlewares[i].Alias, wantAlias)
		}
	}

	idx := map[string]model.Middleware{}
	for _, m := range pm.Middlewares {
		idx[m.Alias] = m
	}

	// Kernel-declared `auth` resolves to its class, origin "app", and — because
	// its class is in $middlewarePriority — carries a non-zero priority.
	auth := idx["auth"]
	if auth.Origin != model.OriginApp {
		t.Errorf("auth origin = %q, want %q (Kernel-declared)", auth.Origin, model.OriginApp)
	}
	if auth.Class != `App\Http\Middleware\Authenticate` {
		t.Errorf("auth class = %q, want the resolved FQN", auth.Class)
	}
	if auth.Priority == 0 {
		t.Errorf("auth priority = 0, want its position in $middlewarePriority")
	}

	// `throttle`'s resolved class is in the "api" group.
	throttle := idx["throttle"]
	if g := throttle.Groups; len(g) != 1 || g[0] != "api" {
		t.Errorf("throttle Groups = %v, want [api] (from $middlewareGroups)", g)
	}

	// `tenant` is now Kernel-declared (origin "app") with its resolved class —
	// no longer an unknown-origin node.
	tenant := idx["tenant"]
	if tenant.Origin != model.OriginApp || tenant.Class != `App\Http\Middleware\EnsureTenant` {
		t.Errorf("tenant = {origin:%q class:%q}, want {app, ...EnsureTenant}", tenant.Origin, tenant.Class)
	}

	// A built-in the Kernel did NOT declare stays framework-origin with no class.
	signed := idx["signed"]
	if signed.Origin != model.OriginFramework || signed.Class != "" {
		t.Errorf("signed = {origin:%q class:%q}, want {framework, \"\"}", signed.Origin, signed.Class)
	}

	// No origin-"unknown" node survives: every applied name resolves.
	for _, m := range pm.Middlewares {
		if m.Origin == model.OriginUnknown {
			t.Errorf("unexpected unknown-origin node %q — the Kernel declares tenant", m.Alias)
		}
	}

	// The tiered order holds: the last app node precedes the first framework node.
	if pm.Middlewares[0].Origin != model.OriginApp {
		t.Errorf("first node origin = %q, want %q (app tier first)", pm.Middlewares[0].Origin, model.OriginApp)
	}
}

// TestAnalyze_FixtureApp11_Middlewares is the end-to-end proof of the Laravel
// 11+ path (issue #67): the same three-tier Middleware union, assembled for a
// project that has NO app/Http/Kernel.php and configures middleware in
// bootstrap/app.php's ->withMiddleware() closure instead.
//
// fixture-app11 declares four aliases (auth, guest, throttle, tenant) via
// ->alias(), so those emit FIRST as origin "app" in declaration order with
// resolved classes; then the built-in backstop for the undeclared aliases; then
// `audit.log` — applied in routes/api.php, declared nowhere, and not a Laravel
// built-in — as the one origin-"unknown" node, proving the applied tier still
// fires on 11+.
func TestAnalyze_FixtureApp11_Middlewares(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, "testdata", "fixture-app11")

	pm, err := engine.Analyze(fixtureApp)
	if err != nil {
		t.Fatalf("engine.Analyze(%q): %v", fixtureApp, err)
	}

	// Tier 1: the ->alias() map, in its declaration order.
	declaredTier := []string{"auth", "guest", "throttle", "tenant"}
	declaredSet := map[string]bool{}
	for _, a := range declaredTier {
		declaredSet[a] = true
	}
	// Tier 2: the built-in backstop minus what bootstrap/app.php declared.
	var frameworkTier []string
	for _, a := range model.BuiltinMiddlewareAliases {
		if !declaredSet[a] {
			frameworkTier = append(frameworkTier, a)
		}
	}
	// Tier 3: `audit.log` is applied but declared nowhere.
	wantAliases := append(append([]string{}, declaredTier...), frameworkTier...)
	wantAliases = append(wantAliases, "audit.log")

	if len(pm.Middlewares) != len(wantAliases) {
		t.Fatalf("Middlewares = %d nodes, want %d\ngot: %+v", len(pm.Middlewares), len(wantAliases), pm.Middlewares)
	}
	for i, wantAlias := range wantAliases {
		if pm.Middlewares[i].Alias != wantAlias {
			t.Errorf("Middlewares[%d].Alias = %q, want %q (tiered emit order)", i, pm.Middlewares[i].Alias, wantAlias)
		}
	}

	idx := map[string]model.Middleware{}
	for _, m := range pm.Middlewares {
		idx[m.Alias] = m
	}

	// `auth` resolves from ->alias(), and its class is in ->priority().
	auth := idx["auth"]
	if auth.Origin != model.OriginApp {
		t.Errorf("auth origin = %q, want %q (bootstrap-declared)", auth.Origin, model.OriginApp)
	}
	if auth.Class != `App\Http\Middleware\Authenticate` {
		t.Errorf("auth class = %q, want the resolved FQN", auth.Class)
	}
	if auth.Priority != 2 {
		t.Errorf("auth priority = %d, want 2 (its position in ->priority())", auth.Priority)
	}

	// `throttle`'s class joins the "api" group via ->appendToGroup().
	throttle := idx["throttle"]
	if g := throttle.Groups; len(g) != 1 || g[0] != "api" {
		t.Errorf("throttle Groups = %v, want [api] (from ->appendToGroup)", g)
	}

	// `tenant` is app-origin with its class — the applied name no longer dangles.
	tenant := idx["tenant"]
	if tenant.Origin != model.OriginApp || tenant.Class != `App\Http\Middleware\EnsureTenant` {
		t.Errorf("tenant = {origin:%q class:%q}, want {app, ...EnsureTenant}", tenant.Origin, tenant.Class)
	}

	// `audit.log` is applied but undeclared: unknown origin, class not guessed.
	auditLog := idx["audit.log"]
	if auditLog.Origin != model.OriginUnknown || auditLog.Class != "" {
		t.Errorf("audit.log = {origin:%q class:%q}, want {unknown, \"\"}", auditLog.Origin, auditLog.Class)
	}

	// A built-in that bootstrap/app.php did NOT declare stays framework-origin.
	signed := idx["signed"]
	if signed.Origin != model.OriginFramework || signed.Class != "" {
		t.Errorf("signed = {origin:%q class:%q}, want {framework, \"\"}", signed.Origin, signed.Class)
	}
}

// TestAnalyze_FixtureApp11_NoKernelFile guards the premise of the test above:
// fixture-app11 must have NO app/Http/Kernel.php, or it would be exercising the
// ≤10 reader (which wins when both layouts are present) and silently stop
// covering the bootstrap path it exists to prove.
func TestAnalyze_FixtureApp11_NoKernelFile(t *testing.T) {
	kernel := filepath.Join(repoRoot(t), "testdata", "fixture-app11", "app", "Http", "Kernel.php")
	if _, err := os.Stat(kernel); !os.IsNotExist(err) {
		t.Fatalf("fixture-app11 must not contain %s — the 11+ fixture would stop covering the bootstrap path", kernel)
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
