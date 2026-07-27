package middleware_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/extract/middleware"
	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// kernelPHP is a Laravel-10-style app/Http/Kernel.php trimmed to what the reader
// needs: a global $middleware stack, keyed $middlewareGroups, a keyed
// $middlewareAliases alias→class map (including one app-only alias, `tenant`,
// that is NOT a Laravel built-in), and a $middlewarePriority list. It exercises
// alias→class resolution, group membership, the global flag, and priority index.
const kernelPHP = `<?php

namespace App\Http;

use Illuminate\Foundation\Http\Kernel as HttpKernel;

class Kernel extends HttpKernel
{
    protected $middleware = [
        \App\Http\Middleware\TrustProxies::class,
        \Illuminate\Foundation\Http\Middleware\PreventRequestsDuringMaintenance::class,
    ];

    protected $middlewareGroups = [
        'web' => [
            \App\Http\Middleware\EncryptCookies::class,
            \Illuminate\Session\Middleware\StartSession::class,
        ],
        'api' => [
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
        ],
    ];

    protected $middlewareAliases = [
        'auth' => \App\Http\Middleware\Authenticate::class,
        'throttle' => \Illuminate\Routing\Middleware\ThrottleRequests::class,
        'tenant' => \App\Http\Middleware\EnsureTenant::class,
    ];

    protected $middlewarePriority = [
        \Illuminate\Session\Middleware\StartSession::class,
        \App\Http\Middleware\Authenticate::class,
        \Illuminate\Routing\Middleware\ThrottleRequests::class,
    ];
}
`

// writeKernel writes kernelPHP to app/Http/Kernel.php under a fresh temp project
// root and returns that root, so ReadKernel exercises the real on-disk path.
func writeKernel(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "app", "Http")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Kernel.php"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write kernel: %v", err)
	}
	return root
}

// TestReadKernel_AbsentIsNilNilError verifies a project with no
// app/Http/Kernel.php yields (nil, nil) — a missing Kernel is not an error, it
// simply contributes no declared tier (the backstop still covers built-ins).
func TestReadKernel_AbsentIsNilNilError(t *testing.T) {
	root := t.TempDir() // no app/Http/Kernel.php

	k, err := middleware.ReadKernel(root)
	if err != nil {
		t.Fatalf("ReadKernel(no kernel) error = %v, want nil", err)
	}
	if k != nil {
		t.Errorf("ReadKernel(no kernel) = %+v, want nil", k)
	}
}

// TestReadKernel_ReadsDeclaredData verifies ReadKernel resolves the alias→class
// map (in declaration order), the group memberships, the global stack, and the
// priority list from a real Kernel file.
func TestReadKernel_ReadsDeclaredData(t *testing.T) {
	root := writeKernel(t, kernelPHP)

	k, err := middleware.ReadKernel(root)
	if err != nil {
		t.Fatalf("ReadKernel error = %v", err)
	}
	if k == nil {
		t.Fatal("ReadKernel = nil, want a populated Kernel")
	}

	// Aliases are ordered as declared: auth, throttle, tenant.
	if got := k.AliasOrder(); len(got) != 3 || got[0] != "auth" || got[1] != "throttle" || got[2] != "tenant" {
		t.Fatalf("AliasOrder = %v, want [auth throttle tenant]", got)
	}
	if got := k.ClassFor("auth"); got != `App\Http\Middleware\Authenticate` {
		t.Errorf("ClassFor(auth) = %q, want the resolved FQN", got)
	}
	if got := k.ClassFor("tenant"); got != `App\Http\Middleware\EnsureTenant` {
		t.Errorf("ClassFor(tenant) = %q, want the app-declared FQN", got)
	}

	// Group membership is by resolved class: auth's class is not in a group, but
	// throttle's class (ThrottleRequests) is in the "api" group.
	if got := k.GroupsFor(`Illuminate\Routing\Middleware\ThrottleRequests`); len(got) != 1 || got[0] != "api" {
		t.Errorf("GroupsFor(ThrottleRequests) = %v, want [api]", got)
	}
	// StartSession is a "web" group member and also appears in priority.
	if got := k.GroupsFor(`Illuminate\Session\Middleware\StartSession`); len(got) != 1 || got[0] != "web" {
		t.Errorf("GroupsFor(StartSession) = %v, want [web]", got)
	}

	// Global stack: TrustProxies is global; Authenticate is not.
	if !k.IsGlobal(`App\Http\Middleware\TrustProxies`) {
		t.Error("IsGlobal(TrustProxies) = false, want true")
	}
	if k.IsGlobal(`App\Http\Middleware\Authenticate`) {
		t.Error("IsGlobal(Authenticate) = true, want false")
	}

	// Priority index is 1-based position in $middlewarePriority (0 = not listed).
	// StartSession is first (1), Authenticate second (2); TrustProxies unlisted (0).
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

// TestReadKernel_RouteMiddlewareFallback verifies the legacy Laravel ≤8 property
// name $routeMiddleware is read when $middlewareAliases is absent (they are the
// same alias map under two names across framework versions).
func TestReadKernel_RouteMiddlewareFallback(t *testing.T) {
	const legacy = `<?php
namespace App\Http;
class Kernel extends \Illuminate\Foundation\Http\Kernel
{
    protected $routeMiddleware = [
        'auth' => \App\Http\Middleware\Authenticate::class,
        'legacy' => \App\Http\Middleware\Legacy::class,
    ];
}
`
	root := writeKernel(t, legacy)

	k, err := middleware.ReadKernel(root)
	if err != nil {
		t.Fatalf("ReadKernel error = %v", err)
	}
	if k == nil {
		t.Fatal("ReadKernel(legacy) = nil, want populated from $routeMiddleware")
	}
	if got := k.AliasOrder(); len(got) != 2 || got[0] != "auth" || got[1] != "legacy" {
		t.Errorf("AliasOrder = %v, want [auth legacy] from $routeMiddleware", got)
	}
	if got := k.ClassFor("legacy"); got != `App\Http\Middleware\Legacy` {
		t.Errorf("ClassFor(legacy) = %q, want the resolved FQN", got)
	}
}

// TestReadKernel_DuplicatePriorityKeepsSourcePositions pins the priority
// numbering against a $middlewarePriority that repeats a class — the case that
// distinguishes index-based positioning from a running counter. A class's
// position is its index in the SOURCE list, so a duplicate keeps its first
// (highest) position AND still consumes its slot: in [A, B, A, C], C is 4, not
// 3. Consumers read these numbers as "where this sits in the declared ordering",
// so they must line up with what the source actually wrote.
func TestReadKernel_DuplicatePriorityKeepsSourcePositions(t *testing.T) {
	const dupes = `<?php
namespace App\Http;
class Kernel extends \Illuminate\Foundation\Http\Kernel
{
    protected $middlewarePriority = [
        \App\A::class,
        \App\B::class,
        \App\A::class,
        \App\C::class,
    ];
}
`
	k, err := middleware.ReadKernel(writeKernel(t, dupes))
	if err != nil {
		t.Fatalf("ReadKernel error = %v", err)
	}
	if k == nil {
		t.Fatal("ReadKernel = nil, want a populated Kernel")
	}

	want := map[string]int{`App\A`: 1, `App\B`: 2, `App\C`: 4}
	for class, wantPos := range want {
		if got := k.PriorityFor(class); got != wantPos {
			t.Errorf("PriorityFor(%s) = %d, want %d (index in the source list)", class, got, wantPos)
		}
	}
}

// TestExtract_KernelDeclaredTierFirst verifies the tiered emit order with a
// Kernel present (ADR 0012): Kernel-declared aliases sort FIRST (in declaration
// order, origin "app", class resolved), then the built-in backstop for aliases
// the Kernel did not declare, then applied-but-undeclared names. An alias the
// Kernel declares that is ALSO a built-in (auth, throttle) appears once, in the
// Kernel tier, origin "app" with its resolved class — not duplicated in the
// framework tier.
func TestExtract_KernelDeclaredTierFirst(t *testing.T) {
	root := writeKernel(t, kernelPHP)
	k, err := middleware.ReadKernel(root)
	if err != nil {
		t.Fatalf("ReadKernel error = %v", err)
	}

	routes := []domain.Route{
		{Method: "GET", URI: "/admin", Middleware: []string{"auth:sanctum", "throttle:api", "tenant", "audit"}},
	}

	got := middleware.Extract(routes, k)

	// The first three nodes are the Kernel-declared aliases, in declaration order.
	if len(got) < 3 {
		t.Fatalf("got %d nodes, want at least the 3 Kernel-declared", len(got))
	}
	wantKernelTier := []string{"auth", "throttle", "tenant"}
	for i, alias := range wantKernelTier {
		if got[i].Alias != alias {
			t.Errorf("node[%d] alias = %q, want %q (Kernel tier, declaration order)", i, got[i].Alias, alias)
		}
		if got[i].Origin != domain.OriginApp {
			t.Errorf("Kernel-declared %q origin = %q, want %q", alias, got[i].Origin, domain.OriginApp)
		}
		if got[i].Class == "" {
			t.Errorf("Kernel-declared %q class is empty, want the resolved FQN", alias)
		}
	}

	idx := byAlias(got)
	// tenant is Kernel-declared now (not unknown) with its resolved class.
	if idx["tenant"].Origin != domain.OriginApp || idx["tenant"].Class != `App\Http\Middleware\EnsureTenant` {
		t.Errorf("tenant = {origin:%q class:%q}, want {app, ...EnsureTenant} (Kernel resolved it)",
			idx["tenant"].Origin, idx["tenant"].Class)
	}
	// throttle's resolved class is a "api" group member: the node reflects it.
	if g := idx["throttle"].Groups; len(g) != 1 || g[0] != "api" {
		t.Errorf("throttle Groups = %v, want [api] (from $middlewareGroups)", g)
	}
	// auth's class is in $middlewarePriority at position 2.
	if idx["auth"].Priority != 2 {
		t.Errorf("auth Priority = %d, want 2 (position in $middlewarePriority)", idx["auth"].Priority)
	}
	// A built-in the Kernel did NOT declare still appears, framework-origin.
	if idx["signed"].Origin != domain.OriginFramework {
		t.Errorf("signed origin = %q, want %q (built-in not in Kernel)", idx["signed"].Origin, domain.OriginFramework)
	}
	// The applied-but-undeclared `audit` is still an unknown node, appended last.
	if idx["audit"].Origin != domain.OriginUnknown {
		t.Errorf("audit origin = %q, want %q (applied, undeclared)", idx["audit"].Origin, domain.OriginUnknown)
	}
	// No alias appears twice: auth/throttle are Kernel-declared, not also framework.
	seen := map[string]int{}
	for _, m := range got {
		seen[m.Alias]++
	}
	for _, a := range []string{"auth", "throttle", "tenant"} {
		if seen[a] != 1 {
			t.Errorf("alias %q appears %d times, want exactly 1 (Kernel tier absorbs the built-in)", a, seen[a])
		}
	}
}

// TestExtract_NilKernelIsBackstopPlusApplied verifies passing a nil Kernel
// reproduces the tracer-bullet behaviour exactly: backstop + applied union, no
// app tier. This is the contract the ≤10-Kernel reader must not regress.
func TestExtract_NilKernelIsBackstopPlusApplied(t *testing.T) {
	routes := []domain.Route{
		{Method: "GET", URI: "/admin", Middleware: []string{"tenant"}},
	}

	got := middleware.Extract(routes, nil)

	if len(got) != len(domain.BuiltinMiddlewareAliases)+1 {
		t.Fatalf("Extract(routes, nil) = %d nodes, want %d (backstop + tenant)",
			len(got), len(domain.BuiltinMiddlewareAliases)+1)
	}
	last := got[len(got)-1]
	if last.Alias != "tenant" || last.Origin != domain.OriginUnknown {
		t.Errorf("last node = {alias:%q origin:%q}, want {tenant, unknown}", last.Alias, last.Origin)
	}
}
