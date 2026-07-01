package analyze

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/symbol"
)

// controllerWith builds a model.Controller with the given FQN and public action
// names, keeping the test cases below terse.
func controllerWith(name, fqn string, actions ...string) model.Controller {
	c := model.NewController(name, fqn)
	c.Actions = append(c.Actions, actions...)
	return c
}

// writeRouteFile writes a route file with the given contents into a temp dir and
// returns its path, for exercising the merged-import resolution context.
func writeRouteFile(t *testing.T, name, php string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(php), 0o600); err != nil {
		t.Fatalf("write route file: %v", err)
	}
	return path
}

// tableWithClasses builds a symbol.Table declaring each given FQN via a
// synthesized single-class file, so IsDeclared reports true for those FQNs. The
// namespace and short name are split from the FQN.
func tableWithClasses(t *testing.T, fqns ...string) *symbol.Table {
	t.Helper()
	dir := t.TempDir()
	tbl := symbol.New()
	for i, fqn := range fqns {
		ns, short := splitFQN(fqn)
		php := "<?php\n"
		if ns != "" {
			php += "namespace " + ns + ";\n"
		}
		php += "class " + short + " {}\n"
		path := filepath.Join(dir, short+"_"+itoa(i)+".php")
		if err := os.WriteFile(path, []byte(php), 0o600); err != nil {
			t.Fatalf("write class file: %v", err)
		}
		if err := tbl.AddFile(path); err != nil {
			t.Fatalf("add class file: %v", err)
		}
	}
	return tbl
}

// splitFQN splits a fully-qualified name into (namespace, short). A bare name
// yields an empty namespace.
func splitFQN(fqn string) (ns, short string) {
	last := -1
	for i := 0; i < len(fqn); i++ {
		if fqn[i] == '\\' {
			last = i
		}
	}
	if last < 0 {
		return "", fqn
	}
	return fqn[:last], fqn[last+1:]
}

// itoa is a tiny int-to-string for unique temp filenames without importing
// strconv into the test's top matter.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// route is a terse constructor for a model.Route in the tests.
func route(method, uri, controller, action string) model.Route {
	return model.Route{Method: method, URI: uri, Controller: controller, Action: action}
}

// TestResolveRoutes drives Phase-2 resolution through its contract cases: an
// imported controller resolves and its FQN is filled in; an unimported short
// name falls back to Laravel's default namespace; a genuinely absent controller
// is flagged missing_controller; a present controller missing the action is
// flagged missing_action; a controller declared in the symbol table but not
// scanned is alive-but-unflagged; an empty-controller route is passed through
// untouched; and an already-qualified name bypasses the import map.
func TestResolveRoutes(t *testing.T) {
	// A single route file importing PostController explicitly. UserController is
	// NOT imported, exercising the default-namespace fallback.
	routeFile := writeRouteFile(t, "web.php", `<?php
use App\Http\Controllers\PostController;
Route::get('/posts', [PostController::class, 'index']);
`)

	tests := []struct {
		name        string
		routes      []model.Route
		controllers []model.Controller
		declared    []string // extra FQNs known to the symbol table but not scanned
		wantRoutes  []model.Route
		wantDead    []model.DeadRoute
	}{
		{
			name:   "imported controller resolves and fills FQN",
			routes: []model.Route{route("GET", "/posts", "PostController", "index")},
			controllers: []model.Controller{
				controllerWith("PostController", `App\Http\Controllers\PostController`, "index", "store"),
			},
			wantRoutes: []model.Route{{
				Method: "GET", URI: "/posts", Controller: "PostController", Action: "index",
				FQN: `App\Http\Controllers\PostController`,
			}},
			wantDead: nil,
		},
		{
			name:   "unimported short name falls back to the default controller namespace",
			routes: []model.Route{route("GET", "/users", "UserController", "index")},
			controllers: []model.Controller{
				controllerWith("UserController", `App\Http\Controllers\UserController`, "index"),
			},
			wantRoutes: []model.Route{{
				Method: "GET", URI: "/users", Controller: "UserController", Action: "index",
				FQN: `App\Http\Controllers\UserController`,
			}},
			wantDead: nil,
		},
		{
			name:        "absent controller yields missing_controller",
			routes:      []model.Route{route("GET", "/ghost", "GhostController", "index")},
			controllers: nil,
			wantRoutes: []model.Route{
				{Method: "GET", URI: "/ghost", Controller: "GhostController", Action: "index"},
			},
			wantDead: []model.DeadRoute{{
				Method: "GET", URI: "/ghost", Controller: "GhostController", Action: "index",
				Reason: `controller "App\Http\Controllers\GhostController" not found`,
				Kind:   model.DeadRouteMissingController,
			}},
		},
		{
			name:   "present controller missing the action yields missing_action",
			routes: []model.Route{route("POST", "/posts", "PostController", "store")},
			controllers: []model.Controller{
				controllerWith("PostController", `App\Http\Controllers\PostController`, "index"),
			},
			wantRoutes: []model.Route{
				{Method: "POST", URI: "/posts", Controller: "PostController", Action: "store"},
			},
			wantDead: []model.DeadRoute{{
				Method: "POST", URI: "/posts", Controller: "PostController", Action: "store",
				Reason: `action "store" not found on controller "App\Http\Controllers\PostController"`,
				Kind:   model.DeadRouteMissingAction,
			}},
		},
		{
			name:        "controller declared but not scanned is alive and unflagged",
			routes:      []model.Route{route("GET", "/ext", "ExternalController", "handle")},
			controllers: nil,
			declared:    []string{`App\Http\Controllers\ExternalController`},
			wantRoutes: []model.Route{{
				Method: "GET", URI: "/ext", Controller: "ExternalController", Action: "handle",
				FQN: `App\Http\Controllers\ExternalController`,
			}},
			wantDead: nil,
		},
		{
			name:        "empty controller route is passed through untouched",
			routes:      []model.Route{route("GET", "/mystery", "", "")},
			controllers: nil,
			wantRoutes: []model.Route{
				{Method: "GET", URI: "/mystery", Controller: "", Action: ""},
			},
			wantDead: nil,
		},
		{
			name:   "already-qualified controller bypasses the import map",
			routes: []model.Route{route("GET", "/admin", `App\Admin\DashboardController`, "index")},
			controllers: []model.Controller{
				controllerWith("DashboardController", `App\Admin\DashboardController`, "index"),
			},
			wantRoutes: []model.Route{{
				Method: "GET", URI: "/admin", Controller: `App\Admin\DashboardController`, Action: "index",
				FQN: `App\Admin\DashboardController`,
			}},
			wantDead: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			declared := append([]string{}, controllerFQNs(tt.controllers)...)
			declared = append(declared, tt.declared...)
			tbl := tableWithClasses(t, declared...)

			gotRoutes, gotDead, err := ResolveRoutes(tt.routes, tt.controllers, tbl, []string{routeFile})
			if err != nil {
				t.Fatalf("ResolveRoutes() error = %v", err)
			}
			if !reflect.DeepEqual(gotRoutes, tt.wantRoutes) {
				t.Errorf("routes = %#v\nwant %#v", gotRoutes, tt.wantRoutes)
			}
			if !reflect.DeepEqual(gotDead, tt.wantDead) {
				t.Errorf("dead = %#v\nwant %#v", gotDead, tt.wantDead)
			}
		})
	}
}

// controllerFQNs extracts the FQNs of the given controllers so they can be
// declared in the symbol table for a test case.
func controllerFQNs(controllers []model.Controller) []string {
	fqns := make([]string, 0, len(controllers))
	for _, c := range controllers {
		fqns = append(fqns, c.FQN)
	}
	return fqns
}

// TestResolveRoutesDoesNotMutateInputs verifies purity: the returned routes are a
// fresh slice and the caller's routes and controllers are untouched.
func TestResolveRoutesDoesNotMutateInputs(t *testing.T) {
	routeFile := writeRouteFile(t, "web.php", `<?php
use App\Http\Controllers\PostController;
`)
	routes := []model.Route{route("GET", "/posts", "PostController", "index")}
	controllers := []model.Controller{
		controllerWith("PostController", `App\Http\Controllers\PostController`, "index"),
	}
	tbl := tableWithClasses(t, `App\Http\Controllers\PostController`)

	wantRoutes := append([]model.Route(nil), routes...)
	wantControllers := append([]model.Controller(nil), controllers...)

	got, _, err := ResolveRoutes(routes, controllers, tbl, []string{routeFile})
	if err != nil {
		t.Fatalf("ResolveRoutes() error = %v", err)
	}

	if !reflect.DeepEqual(routes, wantRoutes) {
		t.Errorf("ResolveRoutes mutated the routes input: got %#v, want %#v", routes, wantRoutes)
	}
	if !reflect.DeepEqual(controllers, wantControllers) {
		t.Errorf("ResolveRoutes mutated the controllers input: got %#v, want %#v", controllers, wantControllers)
	}
	// The returned slice must be independent of the input.
	if len(got) > 0 && &got[0] == &routes[0] {
		t.Error("ResolveRoutes returned the same backing array as the input routes")
	}
}

// TestResolveRoutesUnreadableFileErrors verifies a route file that cannot be read
// aborts with a wrapped error rather than silently mis-resolving.
func TestResolveRoutesUnreadableFileErrors(t *testing.T) {
	_, _, err := ResolveRoutes(nil, nil, symbol.New(), []string{"/no/such/routes.php"})
	if err == nil {
		t.Fatal("ResolveRoutes() with an unreadable route file: want error, got nil")
	}
}
