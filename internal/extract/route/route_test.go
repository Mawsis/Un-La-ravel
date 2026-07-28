package route_test

// External-behavior tests for the Route Extractor. They import the package as
// route_test (black-box) and assert only on the public contract: given one or
// more route-file paths, Extract returns the expected flat []domain.Route — each
// route's Method, fully resolved URI, controller reference (recorded VERBATIM,
// not reduced to a short name), Action, Middleware (in inherited-then-local
// order), and Name — in source order.
//
// The AST-walk internals and unexported helpers are deliberately untouched; only
// the observable route slice is pinned down. Fixtures live under testdata/ and
// mirror the model/schema extractors' file-based convention. Controller FQN
// resolution and dead-route detection are phase two and out of scope here, so
// every expected Route leaves FQN empty.
//
// The domain types package (internal/model) and the extractor package under test
// (internal/extract/route) are imported side by side, so the domain package is
// aliased domain and the extractor keeps its plain name.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	extractroute "github.com/Mawsis/Un-La-ravel/internal/extract/route"
	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// rt is a terse constructor for an expected Route. The variadic middleware keeps
// the no-middleware cases (the majority) compact. The extractor's contract is
// that Middleware is always a non-nil slice (middlewareCopy returns an empty,
// non-nil slice when a route inherits none), so rt normalises an empty variadic
// to []string{} rather than nil to match that exactly under reflect.DeepEqual.
// Name is set separately by the caller when a route carries one, so the common
// unnamed case stays a single call.
func rt(method, uri, controller, action string, middleware ...string) domain.Route {
	if middleware == nil {
		middleware = []string{}
	}
	return domain.Route{
		Method:     method,
		URI:        uri,
		Controller: controller,
		Action:     action,
		Middleware: middleware,
	}
}

// named returns a copy of r with its route name set, keeping rt's signature
// small while still letting the ->name(...) cases assert the exact name. It
// copies rather than mutating so table rows stay independent values.
func named(r domain.Route, name string) domain.Route {
	r.Name = name
	return r
}

func TestExtract(t *testing.T) {
	tests := []struct {
		name    string   // sub-test name
		files   []string // testdata filenames, in extraction order
		pattern string   // human note on the behavior this row pins down
		want    []domain.Route
	}{
		{
			name:    "simple_verb_array_callable",
			files:   []string{"simple_verb.php"},
			pattern: "Route::get('/posts', [PostController::class, 'index']) → one GET route, array-callable action split, no middleware",
			want: []domain.Route{
				rt("GET", "/posts", "PostController", "index"),
			},
		},
		{
			name:    "param_uri_preserved",
			files:   []string{"param_uri.php"},
			pattern: "'/posts/{id}' → the {id} placeholder is carried through URI joining untouched",
			want: []domain.Route{
				rt("GET", "/posts/{id}", "PostController", "show"),
			},
		},
		{
			name:    "chained_middleware_and_name",
			files:   []string{"chained_middleware_name.php"},
			pattern: "->middleware('auth')->name('posts.store') attach to the one route the chain produced, not a sibling",
			want: []domain.Route{
				named(rt("POST", "/posts", "PostController", "store", "auth"), "posts.store"),
			},
		},
		{
			name:    "legacy_string_action",
			files:   []string{"legacy_string.php"},
			pattern: "'LegacyController@show' legacy string action is split on '@' into controller + action; short name kept as written",
			want: []domain.Route{
				rt("GET", "/legacy", "LegacyController", "show"),
			},
		},
		{
			name:    "inline_fqn_array_callable_verbatim",
			files:   []string{"inline_fqn.php"},
			pattern: "[App\\Http\\Controllers\\Admin\\AdminDashboardController::class, 'index'] → the fully-qualified reference is recorded verbatim, NOT collapsed to the short name (issue #63)",
			want: []domain.Route{
				rt("GET", "/admin", `App\Http\Controllers\Admin\AdminDashboardController`, "index"),
			},
		},
		{
			name:    "resource_macro_fqn_verbatim",
			files:   []string{"resource_fqn.php"},
			pattern: "Route::apiResource('comments', App\\Http\\Controllers\\Admin\\CommentController::class) → the sub-namespaced controller survives verbatim on every expanded route (issue #63)",
			want: []domain.Route{
				rt("GET", "/comments", `App\Http\Controllers\Admin\CommentController`, "index"),
				rt("POST", "/comments", `App\Http\Controllers\Admin\CommentController`, "store"),
				rt("GET", "/comments/{id}", `App\Http\Controllers\Admin\CommentController`, "show"),
				rt("PUT", "/comments/{id}", `App\Http\Controllers\Admin\CommentController`, "update"),
				rt("DELETE", "/comments/{id}", `App\Http\Controllers\Admin\CommentController`, "destroy"),
			},
		},
		{
			name:    "legacy_string_action_preserves_namespace",
			files:   []string{"legacy_namespaced.php"},
			pattern: "'Admin\\AdminDashboardController@show' → the namespace segment is preserved through the '@' split, not dropped (issue #63)",
			want: []domain.Route{
				rt("GET", "/admin/legacy", `Admin\AdminDashboardController`, "show"),
			},
		},
		{
			name:    "group_prefix_and_middleware",
			files:   []string{"group_prefix_middleware.php"},
			pattern: "single group: prefix 'admin' joins the URI and ['auth:sanctum','throttle:api'] flatten into the inner route",
			want: []domain.Route{
				rt("GET", "/admin/users", "UserController", "index", "auth:sanctum", "throttle:api"),
			},
		},
		{
			name:    "nested_groups_compose",
			files:   []string{"nested_groups.php"},
			pattern: "nested groups: prefixes concatenate (admin+settings) and middleware accumulate outer→inner ('auth' then 'verified')",
			want: []domain.Route{
				rt("GET", "/admin/settings/general", "SettingController", "show", "auth", "verified"),
			},
		},
		{
			name:    "api_resource_five_routes",
			files:   []string{"api_resource.php"},
			pattern: "Route::apiResource → the five API routes (no create/edit) in Laravel's canonical order with correct method/URI/action",
			want: []domain.Route{
				rt("GET", "/comments", "CommentController", "index"),
				rt("POST", "/comments", "CommentController", "store"),
				rt("GET", "/comments/{id}", "CommentController", "show"),
				rt("PUT", "/comments/{id}", "CommentController", "update"),
				rt("DELETE", "/comments/{id}", "CommentController", "destroy"),
			},
		},
		{
			name:    "resource_seven_routes",
			files:   []string{"resource.php"},
			pattern: "Route::resource → the seven web routes: the five API actions plus create+edit HTML pages, in registration order",
			want: []domain.Route{
				rt("GET", "/photos", "PhotoController", "index"),
				rt("GET", "/photos/create", "PhotoController", "create"),
				rt("POST", "/photos", "PhotoController", "store"),
				rt("GET", "/photos/{id}", "PhotoController", "show"),
				rt("GET", "/photos/{id}/edit", "PhotoController", "edit"),
				rt("PUT", "/photos/{id}", "PhotoController", "update"),
				rt("DELETE", "/photos/{id}", "PhotoController", "destroy"),
			},
		},
		{
			name:    "group_root_path_and_ignored_calls",
			files:   []string{"group_root_and_ignored.php"},
			pattern: "a '/' own-path under prefix 'admin' collapses to '/admin'; non-route facade calls (Cache::get) and non-verb Route:: methods (Route::pattern) are ignored",
			want: []domain.Route{
				rt("GET", "/admin", "DashboardController", "index", "auth"),
			},
		},
		{
			name:    "multiple_files_source_order",
			files:   []string{"simple_verb.php", "legacy_string.php"},
			pattern: "across files routes come out in the order the paths are given (Extract does not reorder)",
			want: []domain.Route{
				rt("GET", "/posts", "PostController", "index"),
				rt("GET", "/legacy", "LegacyController", "show"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			paths := make([]string, len(tc.files))
			for i, f := range tc.files {
				paths[i] = filepath.Join("testdata", f)
			}

			got, err := extractroute.Extract(paths)
			if err != nil {
				t.Fatalf("Extract(%v) returned error: %v", tc.files, err)
			}

			if msg := diffRoutes(got, tc.want); msg != "" {
				t.Errorf("Extract(%v) mismatch [%s]:\n%s", tc.files, tc.pattern, msg)
			}
		})
	}
}

// TestExtractDir exercises the directory-discovery wrapper independently of the
// per-file cases: it points ExtractDir at a directory holding two route files
// and asserts they are discovered, sorted lexically, and flattened together.
// (api_resource.php sorts before chained_middleware_name.php, pinning the sort.)
func TestExtractDir(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "b_chained.php", mustRead(t, filepath.Join("testdata", "chained_middleware_name.php")))
	writeFixture(t, dir, "a_simple.php", mustRead(t, filepath.Join("testdata", "simple_verb.php")))

	got, err := extractroute.ExtractDir(dir)
	if err != nil {
		t.Fatalf("ExtractDir(%q) returned error: %v", dir, err)
	}

	// a_simple.php sorts before b_chained.php, so the GET /posts route precedes
	// the POST /posts route regardless of the OS's raw directory order.
	want := []domain.Route{
		rt("GET", "/posts", "PostController", "index"),
		named(rt("POST", "/posts", "PostController", "store", "auth"), "posts.store"),
	}
	if msg := diffRoutes(got, want); msg != "" {
		t.Errorf("ExtractDir(%q) mismatch:\n%s", dir, msg)
	}
}

// TestExtractMissingFile pins the error contract: an unreadable path aborts the
// whole extraction with a wrapped error (a missing route file means the route
// set would be silently incomplete, so it must not be swallowed).
func TestExtractMissingFile(t *testing.T) {
	_, err := extractroute.Extract([]string{filepath.Join("testdata", "does_not_exist.php")})
	if err == nil {
		t.Fatal("Extract on a missing file: got nil error, want a wrapped read error")
	}
	if !strings.Contains(err.Error(), "route:") {
		t.Errorf("Extract error %q: want it wrapped with the package prefix %q", err, "route:")
	}
}

// diffRoutes returns "" when got and want are element-wise equal, else a
// human-readable description of the first divergence (length, or the first
// differing route). It leans on reflect.DeepEqual for the exact-struct match so
// every field — Method, URI, Controller, Action, Middleware, Name, FQN — is
// asserted, and only formats a message on failure.
func diffRoutes(got, want []domain.Route) string {
	if reflect.DeepEqual(got, want) {
		return ""
	}
	if len(got) != len(want) {
		return "route count = " + itoa(len(got)) + ", want " + itoa(len(want)) +
			"\n got: " + describeRoutes(got) +
			"\nwant: " + describeRoutes(want)
	}
	for i := range got {
		if !reflect.DeepEqual(got[i], want[i]) {
			return "route[" + itoa(i) + "] = " + describeRoute(got[i]) +
				",\n        want " + describeRoute(want[i])
		}
	}
	return ""
}

// describeRoutes renders a route slice as one route per line for the count-
// mismatch message, so a missing or extra expansion is easy to spot.
func describeRoutes(rs []domain.Route) string {
	var b strings.Builder
	for _, r := range rs {
		b.WriteString("\n  - ")
		b.WriteString(describeRoute(r))
	}
	return b.String()
}

// describeRoute renders a single route compactly: verb, URI, controller@action,
// then the optional name and middleware, matching how the route facts read.
func describeRoute(r domain.Route) string {
	s := r.Method + " " + r.URI + " -> " + r.Controller + "@" + r.Action
	if r.Name != "" {
		s += " name=" + r.Name
	}
	if len(r.Middleware) > 0 {
		s += " mw=[" + strings.Join(r.Middleware, ",") + "]"
	}
	if r.FQN != "" {
		s += " fqn=" + r.FQN
	}
	return s
}

// mustRead loads a fixture file's bytes for the ExtractDir temp-dir test or
// fails the test — a missing fixture is a test-setup bug, not a product error.
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", path, err)
	}
	return b
}

// writeFixture writes contents into dir/name for the ExtractDir temp-dir test,
// failing on any I/O error so a broken test setup surfaces immediately.
func writeFixture(t *testing.T, dir, name string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), contents, 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", name, err)
	}
}

// itoa is a tiny dependency-free int-to-string, mirroring the model/schema test
// helpers so the diff formatting stays free of fmt noise.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
