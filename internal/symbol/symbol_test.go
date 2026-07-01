package symbol

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// mustRoot parses PHP source into an AST root, failing the test on a
// catastrophic parse error. Recoverable syntax diagnostics are ignored, as they
// are throughout the codebase (ADR 0003).
func mustRoot(t *testing.T, src string) phpast.Vertex {
	t.Helper()
	res, err := phpast.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return res.Root
}

// addSrc is a test helper that adds a file's source under a path key.
func addSrc(t *testing.T, tbl *Table, path, src string) {
	t.Helper()
	tbl.Add(path, mustRoot(t, src))
}

func TestAddRecordsDeclaredClassByFQN(t *testing.T) {
	tbl := New()
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController extends Controller {}
`)

	if !tbl.IsDeclared(`App\Http\Controllers\PostController`) {
		t.Fatalf("expected PostController declared by FQN")
	}
	if tbl.IsDeclared("PostController") {
		t.Fatalf("short name must not count as declared")
	}
}

func TestAddGlobalNamespaceClassHasNoLeadingSeparator(t *testing.T) {
	tbl := New()
	addSrc(t, tbl, "Legacy.php", `<?php
class LegacyController {}
`)

	if !tbl.IsDeclared("LegacyController") {
		t.Fatalf("global-namespace class should be declared by its bare name")
	}
	if tbl.IsDeclared(`\LegacyController`) {
		t.Fatalf("global-namespace class must not carry a leading backslash")
	}
}

func TestResolveViaImport(t *testing.T) {
	tbl := New()
	// The controller is declared in its own file...
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController extends Controller {}
`)
	// ...and the route file imports it.
	addSrc(t, tbl, "routes/web.php", `<?php
use App\Http\Controllers\PostController;
`)

	fqn, found := tbl.Resolve("PostController", "routes/web.php")
	if fqn != `App\Http\Controllers\PostController` {
		t.Fatalf("fqn = %q, want App\\Http\\Controllers\\PostController", fqn)
	}
	if !found {
		t.Fatalf("expected resolved edge to a declared class")
	}
}

func TestResolveUnimportedIsDeadWithoutDefault(t *testing.T) {
	tbl := New()
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController extends Controller {}
`)
	addSrc(t, tbl, "routes/web.php", `<?php
// no use import for PostController
`)

	// Plain Resolve never guesses a namespace: the bare short name is not a
	// declared class, so the edge is dead.
	fqn, found := tbl.Resolve("PostController", "routes/web.php")
	if fqn != "PostController" {
		t.Fatalf("fqn = %q, want the unresolved short name", fqn)
	}
	if found {
		t.Fatalf("unimported short name must not resolve without a default namespace")
	}
}

func TestResolveWithDefaultAppliesFallback(t *testing.T) {
	tbl := New()
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController extends Controller {}
`)
	addSrc(t, tbl, "routes/web.php", `<?php
// no use import
`)

	fqn, found := tbl.ResolveWithDefault("PostController", "routes/web.php", `App\Http\Controllers`)
	if fqn != `App\Http\Controllers\PostController` {
		t.Fatalf("fqn = %q, want default-qualified FQN", fqn)
	}
	if !found {
		t.Fatalf("expected the default-namespace fallback to resolve a declared class")
	}
}

func TestResolveWithDefaultPrefersImportOverDefault(t *testing.T) {
	tbl := New()
	// Declared in a NON-default namespace...
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Custom;
class PostController {}
`)
	// ...and imported from there. The import must win over the default.
	addSrc(t, tbl, "routes/web.php", `<?php
use App\Custom\PostController;
`)

	fqn, found := tbl.ResolveWithDefault("PostController", "routes/web.php", `App\Http\Controllers`)
	if fqn != `App\Custom\PostController` {
		t.Fatalf("fqn = %q, want the imported FQN (import beats default)", fqn)
	}
	if !found {
		t.Fatalf("expected the imported class to resolve")
	}
}

func TestResolveDeadRouteToMissingController(t *testing.T) {
	tbl := New()
	// Only PostController exists; CommentController is never declared.
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController {}
`)
	addSrc(t, tbl, "routes/web.php", `<?php
use App\Http\Controllers\CommentController;
`)

	fqn, found := tbl.Resolve("CommentController", "routes/web.php")
	if fqn != `App\Http\Controllers\CommentController` {
		t.Fatalf("fqn = %q, want the imported (but undeclared) FQN", fqn)
	}
	if found {
		t.Fatalf("a route to a missing controller must be a dead edge")
	}
}

// TestAntiWrongEdge is the core ADR 0006 correctness property: two classes with
// the SAME short name in DIFFERENT namespaces must each resolve via the
// importing file's own use-map, never collapsing into one wrong edge.
func TestAntiWrongEdge(t *testing.T) {
	tbl := New()
	// Two distinct PostController classes.
	addSrc(t, tbl, "web/PostController.php", `<?php
namespace App\Http\Controllers\Web;
class PostController {}
`)
	addSrc(t, tbl, "api/PostController.php", `<?php
namespace App\Http\Controllers\Api;
class PostController {}
`)
	// Two route files, each importing a different one.
	addSrc(t, tbl, "routes/web.php", `<?php
use App\Http\Controllers\Web\PostController;
`)
	addSrc(t, tbl, "routes/api.php", `<?php
use App\Http\Controllers\Api\PostController;
`)

	webFQN, webFound := tbl.Resolve("PostController", "routes/web.php")
	apiFQN, apiFound := tbl.Resolve("PostController", "routes/api.php")

	if webFQN != `App\Http\Controllers\Web\PostController` || !webFound {
		t.Fatalf("web route resolved to %q (found=%v), want the Web controller", webFQN, webFound)
	}
	if apiFQN != `App\Http\Controllers\Api\PostController` || !apiFound {
		t.Fatalf("api route resolved to %q (found=%v), want the Api controller", apiFQN, apiFound)
	}
	if webFQN == apiFQN {
		t.Fatalf("same short name must not collapse to one FQN across files")
	}
}

func TestResolveHonoursAlias(t *testing.T) {
	tbl := New()
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController {}
`)
	addSrc(t, tbl, "routes/web.php", `<?php
use App\Http\Controllers\PostController as Posts;
`)

	fqn, found := tbl.Resolve("Posts", "routes/web.php")
	if fqn != `App\Http\Controllers\PostController` || !found {
		t.Fatalf("alias Posts resolved to %q (found=%v), want the aliased FQN", fqn, found)
	}
}

func TestResolveAlreadyQualifiedBypassesImports(t *testing.T) {
	tbl := New()
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController {}
`)
	// fromFile has an unrelated import; the qualified name must be used verbatim.
	addSrc(t, tbl, "routes/web.php", `<?php
use App\Other\Thing;
`)

	fqn, found := tbl.Resolve(`App\Http\Controllers\PostController`, "routes/web.php")
	if fqn != `App\Http\Controllers\PostController` || !found {
		t.Fatalf("qualified name resolved to %q (found=%v), want it unchanged and declared", fqn, found)
	}
}

func TestResolveFromUnknownFileIsDead(t *testing.T) {
	tbl := New()
	addSrc(t, tbl, "PostController.php", `<?php
namespace App\Http\Controllers;
class PostController {}
`)

	// A file never Add-ed has no import map; a bare short name cannot resolve.
	fqn, found := tbl.Resolve("PostController", "routes/never-added.php")
	if fqn != "PostController" || found {
		t.Fatalf("resolve from unknown file = (%q, %v), want unresolved dead edge", fqn, found)
	}
}

func TestAddIsIdempotentPerFile(t *testing.T) {
	tbl := New()
	src := `<?php
namespace App\Http\Controllers;
class PostController {}
`
	addSrc(t, tbl, "PostController.php", src)
	addSrc(t, tbl, "PostController.php", src) // re-add same path

	if !tbl.IsDeclared(`App\Http\Controllers\PostController`) {
		t.Fatalf("re-adding the same file must keep its declaration")
	}
}

func TestAddNilRootContributesNothing(t *testing.T) {
	tbl := New()
	tbl.Add("broken.php", nil) // non-Root vertex tolerated

	if _, found := tbl.Resolve("Anything", "broken.php"); found {
		t.Fatalf("a nil root must contribute no declarations or imports")
	}
}
