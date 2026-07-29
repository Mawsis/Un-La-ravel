package middleware_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/extract/middleware"
	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// bootstrapPHP is a Laravel-11-style bootstrap/app.php trimmed to the calls the
// reader understands (issue #67): ->alias() for the alias→class map, ->group()
// and ->appendToGroup()/->prependToGroup() for group membership, ->append() and
// ->prepend() for the global stack, and ->priority() for the ordering. It mirrors
// kernelPHP's data so the 11+ path and the ≤10 path can be asserted to produce
// the same node shape from equivalent declarations.
const bootstrapPHP = `<?php

use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Middleware;

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        api: __DIR__.'/../routes/api.php',
    )
    ->withMiddleware(function (Middleware $middleware) {
        $middleware->alias([
            'auth' => \App\Http\Middleware\Authenticate::class,
            'throttle' => \Illuminate\Routing\Middleware\ThrottleRequests::class,
            'tenant' => \App\Http\Middleware\EnsureTenant::class,
        ]);

        $middleware->group('web', [
            \App\Http\Middleware\EncryptCookies::class,
            \Illuminate\Session\Middleware\StartSession::class,
        ]);

        $middleware->appendToGroup('api', [
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
        ]);

        $middleware->append(\App\Http\Middleware\TrustProxies::class);
        $middleware->prepend([
            \Illuminate\Foundation\Http\Middleware\PreventRequestsDuringMaintenance::class,
        ]);

        $middleware->priority([
            \Illuminate\Session\Middleware\StartSession::class,
            \App\Http\Middleware\Authenticate::class,
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
        ]);
    })
    ->create();
`

// writeBootstrap writes contents to bootstrap/app.php under a fresh temp project
// root and returns that root, so ReadBootstrap exercises the real on-disk path.
func writeBootstrap(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "bootstrap")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.php"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write bootstrap: %v", err)
	}
	return root
}

// TestReadBootstrap_AbsentIsNilNilError verifies a project with no
// bootstrap/app.php yields (nil, nil) — the ≤10 layout is not an error here, it
// simply contributes no declared tier from this source.
func TestReadBootstrap_AbsentIsNilNilError(t *testing.T) {
	root := t.TempDir() // no bootstrap/app.php

	k, err := middleware.ReadBootstrap(root)
	if err != nil {
		t.Fatalf("ReadBootstrap(no bootstrap) error = %v, want nil", err)
	}
	if k != nil {
		t.Errorf("ReadBootstrap(no bootstrap) = %+v, want nil", k)
	}
}

// TestReadBootstrap_NoWithMiddleware verifies a bootstrap/app.php that exists but
// configures no middleware (a bare Laravel 11 skeleton) yields (nil, nil): there
// is no declared tier to contribute, and the union collapses to backstop ∪
// applied exactly as for a project with no bootstrap file at all.
func TestReadBootstrap_NoWithMiddleware(t *testing.T) {
	root := writeBootstrap(t, `<?php

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(api: __DIR__.'/../routes/api.php')
    ->create();
`)

	k, err := middleware.ReadBootstrap(root)
	if err != nil {
		t.Fatalf("ReadBootstrap error = %v", err)
	}
	if k != nil {
		t.Errorf("ReadBootstrap(no withMiddleware) = %+v, want nil", k)
	}
}

// TestReadBootstrap_ReadsDeclaredData verifies ReadBootstrap resolves the
// alias→class map (in declaration order), group membership from all three group
// calls, the global stack from append and prepend, and the priority list.
func TestReadBootstrap_ReadsDeclaredData(t *testing.T) {
	root := writeBootstrap(t, bootstrapPHP)

	k, err := middleware.ReadBootstrap(root)
	if err != nil {
		t.Fatalf("ReadBootstrap error = %v", err)
	}
	if k == nil {
		t.Fatal("ReadBootstrap = nil, want populated declared middleware")
	}

	// Aliases are ordered as declared inside ->alias(): auth, throttle, tenant.
	if got := k.AliasOrder(); len(got) != 3 || got[0] != "auth" || got[1] != "throttle" || got[2] != "tenant" {
		t.Fatalf("AliasOrder = %v, want [auth throttle tenant]", got)
	}
	if got := k.ClassFor("auth"); got != `App\Http\Middleware\Authenticate` {
		t.Errorf("ClassFor(auth) = %q, want the resolved FQN", got)
	}
	if got := k.ClassFor("tenant"); got != `App\Http\Middleware\EnsureTenant` {
		t.Errorf("ClassFor(tenant) = %q, want the app-declared FQN", got)
	}

	// Group membership: ->group() defines "web"; ->appendToGroup() adds to "api".
	if got := k.GroupsFor(`Illuminate\Session\Middleware\StartSession`); len(got) != 1 || got[0] != "web" {
		t.Errorf("GroupsFor(StartSession) = %v, want [web]", got)
	}
	if got := k.GroupsFor(`Illuminate\Routing\Middleware\ThrottleRequests`); len(got) != 1 || got[0] != "api" {
		t.Errorf("GroupsFor(ThrottleRequests) = %v, want [api]", got)
	}

	// Global stack: ->append() takes a bare class-const, ->prepend() a list.
	if !k.IsGlobal(`App\Http\Middleware\TrustProxies`) {
		t.Error("IsGlobal(TrustProxies) = false, want true (appended)")
	}
	if !k.IsGlobal(`Illuminate\Foundation\Http\Middleware\PreventRequestsDuringMaintenance`) {
		t.Error("IsGlobal(PreventRequestsDuringMaintenance) = false, want true (prepended)")
	}
	if k.IsGlobal(`App\Http\Middleware\Authenticate`) {
		t.Error("IsGlobal(Authenticate) = true, want false")
	}

	// Priority index is the 1-based position in ->priority() (0 = not listed).
	if got := k.PriorityFor(`Illuminate\Session\Middleware\StartSession`); got != 1 {
		t.Errorf("PriorityFor(StartSession) = %d, want 1", got)
	}
	if got := k.PriorityFor(`App\Http\Middleware\Authenticate`); got != 2 {
		t.Errorf("PriorityFor(Authenticate) = %d, want 2", got)
	}
	if got := k.PriorityFor(`App\Http\Middleware\TrustProxies`); got != 0 {
		t.Errorf("PriorityFor(TrustProxies) = %d, want 0 (unlisted)", got)
	}
}

// TestReadBootstrap_WebAndApiShorthands verifies the ->web([...]) / ->api([...])
// group shorthands are read as membership of the "web" and "api" groups. Laravel
// 11's own documentation leads with these rather than
// ->appendToGroup('web', ...), so an app is more likely to write them than the
// long form; they are plain array literals, so reading them needs no guessing.
func TestReadBootstrap_WebAndApiShorthands(t *testing.T) {
	root := writeBootstrap(t, `<?php

return Application::configure()
    ->withMiddleware(function (Middleware $middleware) {
        $middleware->web(append: [
            \App\Http\Middleware\EnsureTenant::class,
        ]);
        $middleware->api([
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
        ]);
    })
    ->create();
`)

	k, err := middleware.ReadBootstrap(root)
	if err != nil {
		t.Fatalf("ReadBootstrap error = %v", err)
	}
	if k == nil {
		t.Fatal("ReadBootstrap = nil, want the shorthand groups to be read")
	}

	if got := k.GroupsFor(`App\Http\Middleware\EnsureTenant`); len(got) != 1 || got[0] != "web" {
		t.Errorf("GroupsFor(EnsureTenant) = %v, want [web] (from ->web())", got)
	}
	if got := k.GroupsFor(`Illuminate\Routing\Middleware\ThrottleRequests`); len(got) != 1 || got[0] != "api" {
		t.Errorf("GroupsFor(ThrottleRequests) = %v, want [api] (from ->api())", got)
	}
}

// TestReadBootstrap_WhollyUnreadableClosureIsNil verifies that a
// ->withMiddleware() closure from which NOTHING is statically readable collapses
// to the same (nil, nil) a project with no closure gives: "found the declaration
// site but could read nothing" and "there is no declaration site" are the same
// state — no declared tier — so callers never special-case an empty Kernel.
func TestReadBootstrap_WhollyUnreadableClosureIsNil(t *testing.T) {
	root := writeBootstrap(t, `<?php

return Application::configure()
    ->withMiddleware(function (Middleware $middleware) {
        $middleware->alias($runtimeAliases);
        $middleware->append($runtimeGlobal);
        $middleware->useKernel(\App\Http\Kernel::class);
    })
    ->create();
`)

	k, err := middleware.ReadBootstrap(root)
	if err != nil {
		t.Fatalf("ReadBootstrap error = %v", err)
	}
	if k != nil {
		t.Errorf("ReadBootstrap(unreadable closure) = %+v, want nil", k)
	}
}

// TestReadBootstrap_ExoticFormsAreSkipped verifies the best-effort boundary of
// ADR 0002: calls whose arguments are not array/class-const literals (a variable,
// a concatenation, a method we do not model) contribute nothing rather than being
// guessed, while the readable calls in the same closure still land.
func TestReadBootstrap_ExoticFormsAreSkipped(t *testing.T) {
	root := writeBootstrap(t, `<?php

return Application::configure()
    ->withMiddleware(function (Middleware $middleware) {
        $middleware->alias($dynamicAliases);
        $middleware->alias([
            'auth' => \App\Http\Middleware\Authenticate::class,
            'dynamic' => $someClass,
        ]);
        $middleware->append($runtimeMiddleware);
        $middleware->replaceInGroup('web', \A::class, \B::class);
        $middleware->group($groupName, [\App\Http\Middleware\EnsureTenant::class]);
    })
    ->create();
`)

	k, err := middleware.ReadBootstrap(root)
	if err != nil {
		t.Fatalf("ReadBootstrap error = %v", err)
	}
	if k == nil {
		t.Fatal("ReadBootstrap = nil, want the readable calls to still land")
	}

	// Only the one statically-readable alias entry survives.
	if got := k.AliasOrder(); len(got) != 1 || got[0] != "auth" {
		t.Errorf("AliasOrder = %v, want [auth] (dynamic entries skipped)", got)
	}
	// A dynamic ->append() argument contributes no global.
	if k.IsGlobal(`App\Http\Middleware\Authenticate`) {
		t.Error("IsGlobal(Authenticate) = true, want false")
	}
	// A dynamically-named group contributes no membership.
	if got := k.GroupsFor(`App\Http\Middleware\EnsureTenant`); len(got) != 0 {
		t.Errorf("GroupsFor(EnsureTenant) = %v, want none (dynamic group name)", got)
	}
}

// TestReadBootstrap_FeedsExtractSameShapeAsKernel verifies the 11+ source drives
// Extract to the identical node set the ≤10 source produces from equivalent
// declarations — the acceptance criterion that the bootstrap path yields "the
// same node shape as the ≤10 path".
func TestReadBootstrap_FeedsExtractSameShapeAsKernel(t *testing.T) {
	fromBootstrap, err := middleware.ReadBootstrap(writeBootstrap(t, bootstrapPHP))
	if err != nil {
		t.Fatalf("ReadBootstrap error = %v", err)
	}
	fromKernel, err := middleware.ReadKernel(writeKernel(t, kernelPHP))
	if err != nil {
		t.Fatalf("ReadKernel error = %v", err)
	}

	routes := []domain.Route{{Middleware: []string{"auth:sanctum", "tenant"}}}
	got := middleware.Extract(routes, fromBootstrap)
	want := middleware.Extract(routes, fromKernel)

	if len(got) != len(want) {
		t.Fatalf("bootstrap yielded %d nodes, kernel yielded %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Alias != want[i].Alias || got[i].Class != want[i].Class ||
			got[i].Origin != want[i].Origin || got[i].Global != want[i].Global ||
			got[i].Priority != want[i].Priority {
			t.Errorf("node %d: bootstrap = %+v, kernel = %+v", i, got[i], want[i])
		}
	}
}
