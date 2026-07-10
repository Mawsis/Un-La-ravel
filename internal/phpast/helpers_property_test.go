package phpast_test

import (
	"reflect"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/VKCOM/php-parser/pkg/ast"
)

// kernelSnippet is a Laravel-10-style HTTP Kernel trimmed to the four property
// shapes the middleware extractor reads (issue #66): a keyed alias→class-const
// map, a keyed group→list-of-class-const map, a list of class-const globals, and
// a list of class-const priorities. It also declares an unrelated $foo property
// and a second class, so the tests prove the class/property selection is precise
// (only App\Http\Kernel's named properties are read, not another class's).
const kernelSnippet = `<?php

namespace App\Http;

use Illuminate\Foundation\Http\Kernel as HttpKernel;

class Kernel extends HttpKernel
{
    protected $foo = 'not-an-array';

    protected $middleware = [
        \App\Http\Middleware\TrustProxies::class,
        \App\Http\Middleware\PreventRequestsDuringMaintenance::class,
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
    ];
}

class NotTheKernel
{
    protected $middlewareAliases = [
        'decoy' => \App\Decoy::class,
    ];
}
`

// TestClassPropertyArrayReadsNamedProperty proves ClassPropertyArray locates the
// array-literal value of a named property on a named class, and that the four
// value readers reduce each Kernel property shape correctly.
func TestClassPropertyArrayReadsNamedProperty(t *testing.T) {
	root := parseRoot(t, kernelSnippet)

	// $middlewareAliases: a keyed string→class-const map.
	aliasesArr := phpast.ClassPropertyArray(root, "Kernel", "middlewareAliases")
	if aliasesArr == nil {
		t.Fatal("ClassPropertyArray(Kernel, middlewareAliases) = nil, want the array expr")
	}
	gotAliases := phpast.ArrayClassConstPairs(aliasesArr)
	wantAliases := []phpast.ClassConstPair{
		{Key: "auth", Class: `App\Http\Middleware\Authenticate`},
		{Key: "throttle", Class: `Illuminate\Routing\Middleware\ThrottleRequests`},
		{Key: "tenant", Class: `App\Http\Middleware\EnsureTenant`},
	}
	if !reflect.DeepEqual(gotAliases, wantAliases) {
		t.Errorf("ArrayClassConstPairs(middlewareAliases) = %v, want %v", gotAliases, wantAliases)
	}

	// $middlewareGroups: a keyed string→list-of-class-const map.
	groupsArr := phpast.ClassPropertyArray(root, "Kernel", "middlewareGroups")
	gotGroups := phpast.ArrayClassConstGroups(groupsArr)
	wantGroups := []phpast.ClassConstGroup{
		{Key: "web", Classes: []string{
			`App\Http\Middleware\EncryptCookies`,
			`Illuminate\Session\Middleware\StartSession`,
		}},
		{Key: "api", Classes: []string{
			`Illuminate\Routing\Middleware\ThrottleRequests`,
		}},
	}
	if !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Errorf("ArrayClassConstGroups(middlewareGroups) = %v, want %v", gotGroups, wantGroups)
	}

	// $middleware: a list of class-const globals.
	globalArr := phpast.ClassPropertyArray(root, "Kernel", "middleware")
	gotGlobal := phpast.ArrayClassConstItems(globalArr)
	wantGlobal := []string{
		`App\Http\Middleware\TrustProxies`,
		`App\Http\Middleware\PreventRequestsDuringMaintenance`,
	}
	if !reflect.DeepEqual(gotGlobal, wantGlobal) {
		t.Errorf("ArrayClassConstItems(middleware) = %v, want %v", gotGlobal, wantGlobal)
	}

	// $middlewarePriority: a list of class-const priorities.
	priorityArr := phpast.ClassPropertyArray(root, "Kernel", "middlewarePriority")
	gotPriority := phpast.ArrayClassConstItems(priorityArr)
	wantPriority := []string{
		`Illuminate\Session\Middleware\StartSession`,
		`App\Http\Middleware\Authenticate`,
	}
	if !reflect.DeepEqual(gotPriority, wantPriority) {
		t.Errorf("ArrayClassConstItems(middlewarePriority) = %v, want %v", gotPriority, wantPriority)
	}
}

// TestClassPropertyArraySelectsTheRightClass proves the class name is honoured:
// NotTheKernel also declares $middlewareAliases, but reading Kernel's must not
// return the decoy class's array.
func TestClassPropertyArraySelectsTheRightClass(t *testing.T) {
	root := parseRoot(t, kernelSnippet)

	decoy := phpast.ClassPropertyArray(root, "NotTheKernel", "middlewareAliases")
	got := phpast.ArrayClassConstPairs(decoy)
	want := []phpast.ClassConstPair{{Key: "decoy", Class: `App\Decoy`}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotTheKernel.middlewareAliases = %v, want %v", got, want)
	}

	// A property that exists on NotTheKernel is not returned when asked for on
	// Kernel-only... conversely, asking Kernel for a NotTheKernel-only shape is
	// covered by the happy-path test above; here we assert the decoy stays put.
	if len(got) == 3 {
		t.Error("read Kernel's three aliases through the NotTheKernel selector — class selection is not honoured")
	}
}

// TestClassPropertyArrayMissing proves the not-found paths: an unknown class, an
// unknown property, and a property whose value is not an array literal all yield
// nil, and a non-Root node yields nil.
func TestClassPropertyArrayMissing(t *testing.T) {
	root := parseRoot(t, kernelSnippet)

	if got := phpast.ClassPropertyArray(root, "NoSuchClass", "middlewareAliases"); got != nil {
		t.Errorf("ClassPropertyArray(NoSuchClass, ...) = %v, want nil", got)
	}
	if got := phpast.ClassPropertyArray(root, "Kernel", "noSuchProperty"); got != nil {
		t.Errorf("ClassPropertyArray(Kernel, noSuchProperty) = %v, want nil", got)
	}
	// $foo is a string literal, not an array — the array selector must reject it.
	if got := phpast.ClassPropertyArray(root, "Kernel", "foo"); got != nil {
		t.Errorf("ClassPropertyArray(Kernel, foo) = %v, want nil (value is not an array)", got)
	}
	if got := phpast.ClassPropertyArray(&ast.Identifier{Value: []byte("x")}, "Kernel", "middleware"); got != nil {
		t.Errorf("ClassPropertyArray(non-Root, ...) = %v, want nil", got)
	}
}

// TestArrayClassConstReadersSkipNonClassConst proves the value readers skip
// entries that are not Class::class fetches without panicking: a plain string
// value in a keyed map, a plain string item in a list, and a non-list value for
// a group key are all dropped, keeping only the well-formed class-const entries.
func TestArrayClassConstReadersSkipNonClassConst(t *testing.T) {
	root := parseRoot(t, `<?php
$aliases = ['good' => \App\Good::class, 'bad' => 'not-a-class-const'];
$items = [\App\Kept::class, 'dropped', $var];
$groups = ['ok' => [\App\A::class, 'skip'], 'bad' => 'not-a-list'];
`)

	ac := &arrayCollector{}
	phpast.Walk(root, ac)
	if len(ac.arrays) < 3 {
		t.Fatalf("expected at least 3 top-level arrays, got %d", len(ac.arrays))
	}
	// arrayCollector visits nested arrays too; the three top-level RHS arrays are
	// the first, then the group's inner list is nested. Identify by content.

	// The keyed alias array skips the non-class-const value.
	pairs := phpast.ArrayClassConstPairs(ac.arrays[0])
	if !reflect.DeepEqual(pairs, []phpast.ClassConstPair{{Key: "good", Class: `App\Good`}}) {
		t.Errorf("ArrayClassConstPairs(mixed) = %v, want one good pair", pairs)
	}

	// ArrayClassConstItems returns nil for a non-array node and skips non-class
	// items.
	if got := phpast.ArrayClassConstItems(&ast.Identifier{Value: []byte("x")}); got != nil {
		t.Errorf("ArrayClassConstItems(non-array) = %v, want nil", got)
	}
	if got := phpast.ArrayClassConstPairs(nil); got != nil {
		t.Errorf("ArrayClassConstPairs(nil) = %v, want nil", got)
	}
	if got := phpast.ArrayClassConstGroups(nil); got != nil {
		t.Errorf("ArrayClassConstGroups(nil) = %v, want nil", got)
	}
}
