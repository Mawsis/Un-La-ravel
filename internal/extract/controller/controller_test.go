package controller

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	domain "github.com/mawsis/unlaravel/internal/model"
)

// The fixtures under testdata/ each isolate one behaviour of the extractor: the
// action-visibility rules (mixed_methods), the global-namespace FQN branch
// (global_namespace), the non-nil empty Actions guarantee (no_public_actions),
// and multiple classes in one file (two_classes). Fixture names are constants so
// a rename fails to compile here rather than silently skipping a case.
const (
	fixtureMixedMethods    = "mixed_methods.php"
	fixtureGlobalNamespace = "global_namespace.php"
	fixtureNoPublicActions = "no_public_actions.php"
	fixtureTwoClasses      = "two_classes.php"
	fixtureAnonymousClass  = "anonymous_class.php"
	fixtureTypedParams     = "typed_params.php"
)

// td returns the testdata path for a fixture file name.
func td(name string) string { return filepath.Join("testdata", name) }

// only returns the single controller in cs, failing the test if the count is not
// exactly one — most fixtures declare a single class.
func only(t *testing.T, cs []domain.Controller) domain.Controller {
	t.Helper()
	if len(cs) != 1 {
		t.Fatalf("want exactly 1 controller, got %d: %+v", len(cs), cs)
	}
	return cs[0]
}

func TestExtractMixedMethods(t *testing.T) {
	cs, err := Extract([]string{td(fixtureMixedMethods)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	c := only(t, cs)

	if c.Name != "PostController" {
		t.Errorf("Name = %q, want PostController", c.Name)
	}
	if c.FQN != `App\Http\Controllers\PostController` {
		t.Errorf("FQN = %q, want App\\Http\\Controllers\\PostController", c.FQN)
	}
	// index/store are explicitly public, show has no visibility modifier (PHP
	// default public), __invoke is the invokable action — all in source order.
	// __construct, private authorizeRequest, and protected transform are excluded.
	want := []string{"index", "show", "store", "__invoke"}
	if !reflect.DeepEqual(c.Actions, want) {
		t.Errorf("Actions = %v, want %v", c.Actions, want)
	}
}

func TestExtractGlobalNamespaceFQNHasNoLeadingSeparator(t *testing.T) {
	cs, err := Extract([]string{td(fixtureGlobalNamespace)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	c := only(t, cs)

	// A class in the global namespace qualifies to its bare short name, never a
	// leading backslash (qualify()'s empty-namespace branch, ADR 0006).
	if c.FQN != "LegacyController" {
		t.Errorf("FQN = %q, want the bare LegacyController", c.FQN)
	}
	if strings.HasPrefix(c.FQN, `\`) {
		t.Errorf("FQN %q must not carry a leading backslash", c.FQN)
	}
	if want := []string{"show"}; !reflect.DeepEqual(c.Actions, want) {
		t.Errorf("Actions = %v, want %v", c.Actions, want)
	}
}

func TestExtractControllerWithNoPublicActionsHasEmptyNonNilSlice(t *testing.T) {
	cs, err := Extract([]string{td(fixtureNoPublicActions)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	c := only(t, cs)

	// No base-class filter: the class is emitted despite having no actions.
	if c.Name != "EmptyController" {
		t.Errorf("Name = %q, want EmptyController", c.Name)
	}
	if c.Actions == nil {
		t.Fatalf("Actions must be a non-nil empty slice, got nil (would serialize as null)")
	}
	if len(c.Actions) != 0 {
		t.Errorf("Actions = %v, want empty", c.Actions)
	}
}

func TestExtractMultipleClassesInOneFile(t *testing.T) {
	cs, err := Extract([]string{td(fixtureTwoClasses)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("want 2 controllers from a two-class file, got %d", len(cs))
	}

	// Emitted in source order, each with its own FQN and actions.
	if cs[0].Name != "FirstController" || !reflect.DeepEqual(cs[0].Actions, []string{"index"}) {
		t.Errorf("class 0 = %+v, want FirstController[index]", cs[0])
	}
	if cs[1].Name != "SecondController" || !reflect.DeepEqual(cs[1].Actions, []string{"store"}) {
		t.Errorf("class 1 = %+v, want SecondController[store]", cs[1])
	}
	if cs[0].FQN != `App\Http\Controllers\FirstController` {
		t.Errorf("class 0 FQN = %q", cs[0].FQN)
	}
	if cs[1].FQN != `App\Http\Controllers\SecondController` {
		t.Errorf("class 1 FQN = %q", cs[1].FQN)
	}
}

// TestExtractAnonymousClassMethodDoesNotLeak proves the visitor's empty-name
// guards: a named controller containing an anonymous class emits only the named
// class, and the anonymous class's method never becomes an action (StmtClass
// clears "current" on the unnamed class; StmtClassMethod then skips its members).
func TestExtractAnonymousClassMethodDoesNotLeak(t *testing.T) {
	cs, err := Extract([]string{td(fixtureAnonymousClass)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	c := only(t, cs)

	if c.Name != "FooController" {
		t.Errorf("Name = %q, want FooController (the anonymous class must not be emitted)", c.Name)
	}
	if want := []string{"index"}; !reflect.DeepEqual(c.Actions, want) {
		t.Errorf("Actions = %v, want %v — the anonymous class's method must not leak", c.Actions, want)
	}
}

func TestExtractProcessesPathsInOrder(t *testing.T) {
	// Extract preserves the given path order in the emitted slice, so the caller
	// controls discovery order (ExtractDir sorts before calling Extract).
	cs, err := Extract([]string{td(fixtureGlobalNamespace), td(fixtureMixedMethods)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("want 2 controllers, got %d", len(cs))
	}
	if cs[0].Name != "LegacyController" || cs[1].Name != "PostController" {
		t.Errorf("order = [%q, %q], want [LegacyController, PostController]", cs[0].Name, cs[1].Name)
	}
}

// TestExtractWithParamsCapturesTypedActionParams proves the ActionParams side
// map records each action's typed parameter hints in declaration order, omits
// untyped parameters, and omits actions that have no typed parameters — while
// the Controller slice keeps its unchanged shape (Actions still lists index too).
func TestExtractWithParamsCapturesTypedActionParams(t *testing.T) {
	cs, params, err := ExtractWithParams([]string{td(fixtureTypedParams)})
	if err != nil {
		t.Fatalf("ExtractWithParams: %v", err)
	}
	c := only(t, cs)

	// The Controller contract is untouched: every public action still appears,
	// including index, which contributes nothing to the params map.
	if want := []string{"index", "store", "update"}; !reflect.DeepEqual(c.Actions, want) {
		t.Errorf("Actions = %v, want %v", c.Actions, want)
	}

	fqn := `App\Http\Controllers\PostController`
	got, ok := params[fqn]
	if !ok {
		t.Fatalf("ActionParams missing entry for %q; got keys %v", fqn, keysOf(params))
	}

	want := map[string][]string{
		"store":  {"StorePostRequest"},
		"update": {"UpdatePostRequest"}, // the untyped $id is omitted
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ActionParams[%q] = %v, want %v", fqn, got, want)
	}
	if _, present := got["index"]; present {
		t.Errorf("index has no typed params and must be absent, got %v", got["index"])
	}
}

// TestExtractWithParamsEmptyWhenNoTypedParams proves the map is non-nil and
// empty (never nil) when no action declares a typed parameter, so callers can
// range over it safely and no controller with only untyped/no params appears.
func TestExtractWithParamsEmptyWhenNoTypedParams(t *testing.T) {
	_, params, err := ExtractWithParams([]string{td(fixtureMixedMethods)})
	if err != nil {
		t.Fatalf("ExtractWithParams: %v", err)
	}
	if params == nil {
		t.Fatalf("ActionParams must be non-nil even with no typed params")
	}
	if len(params) != 0 {
		t.Errorf("ActionParams = %v, want empty (mixed_methods actions take no typed params)", params)
	}
}

// keysOf returns the keys of an ActionParams map for diagnostic output.
func keysOf(m ActionParams) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestExtractMissingFileIsWrappedError(t *testing.T) {
	_, err := Extract([]string{td("does_not_exist.php")})
	if err == nil {
		t.Fatalf("Extract of a missing file must return an error")
	}
	// Wrapped with the package prefix and offending path (%w chain, ADR 0003):
	// a dropped controller would turn valid routes into false dead-route findings.
	if !strings.Contains(err.Error(), "controller: extract") {
		t.Fatalf("error %q missing the wrapped context prefix", err)
	}
	if !strings.Contains(err.Error(), "does_not_exist.php") {
		t.Fatalf("error %q should name the offending path", err)
	}
}

func TestExtractEmptyPathsYieldsNoControllers(t *testing.T) {
	cs, err := Extract(nil)
	if err != nil {
		t.Fatalf("Extract(nil): %v", err)
	}
	if len(cs) != 0 {
		t.Fatalf("Extract(nil) = %v, want no controllers", cs)
	}
}

// TestExtractDirRecursesAndSorts builds a nested controller tree in a temp dir
// and confirms ExtractDir discovers files recursively (Admin/ subdir carries a
// namespace segment) and returns them in lexical path order.
func TestExtractDirRecursesAndSorts(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "PostController.php"), `<?php
namespace App\Http\Controllers;
class PostController extends Controller {
    public function index() {}
}
`)
	writeFile(t, filepath.Join(dir, "Admin", "DashboardController.php"), `<?php
namespace App\Http\Controllers\Admin;
class DashboardController {
    public function __invoke() {}
}
`)
	// A non-PHP file must be ignored by the discovery glob.
	writeFile(t, filepath.Join(dir, "notes.txt"), "ignore me")

	cs, err := ExtractDir(dir)
	if err != nil {
		t.Fatalf("ExtractDir: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("want 2 controllers from the nested tree, got %d: %+v", len(cs), cs)
	}

	// Lexical path order: "<dir>/Admin/..." sorts before "<dir>/PostController.php".
	if cs[0].FQN != `App\Http\Controllers\Admin\DashboardController` {
		t.Errorf("first FQN = %q, want the nested Admin DashboardController", cs[0].FQN)
	}
	if !reflect.DeepEqual(cs[0].Actions, []string{"__invoke"}) {
		t.Errorf("DashboardController actions = %v, want [__invoke]", cs[0].Actions)
	}
	if cs[1].FQN != `App\Http\Controllers\PostController` {
		t.Errorf("second FQN = %q, want the top-level PostController", cs[1].FQN)
	}
}

func TestExtractDirMissingDirIsWrappedError(t *testing.T) {
	_, err := ExtractDir(filepath.Join(t.TempDir(), "no-such-dir"))
	if err == nil {
		t.Fatalf("ExtractDir of a missing directory must return an error")
	}
	if !strings.Contains(err.Error(), "controller: discover controllers dir") {
		t.Fatalf("error %q missing the wrapped context prefix", err)
	}
}

// writeFile creates path (with any parent directories) and writes content,
// failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
