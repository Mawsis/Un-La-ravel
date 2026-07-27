package phpast_test

import (
	"reflect"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// bootstrapSnippet is a Laravel-11-style bootstrap/app.php trimmed to the shapes
// the middleware extractor reads (issue #67): the `->withMiddleware(function
// (Middleware $middleware) { ... })` link of the Application::configure() chain,
// whose body configures aliases, groups, the global stack, and priority through
// `$middleware->...()` calls. Other links in the chain (->withRouting,
// ->withExceptions) and an unrelated statement inside the closure are present so
// the tests prove the selection is precise.
const bootstrapSnippet = `<?php

use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Middleware;

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        web: __DIR__.'/../routes/web.php',
        api: __DIR__.'/../routes/api.php',
    )
    ->withMiddleware(function (Middleware $middleware) {
        $middleware->alias([
            'auth' => \App\Http\Middleware\Authenticate::class,
            'tenant' => \App\Http\Middleware\EnsureTenant::class,
        ]);

        $middleware->appendToGroup('api', [
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
        ]);

        $middleware->append(\App\Http\Middleware\TrustProxies::class);

        $middleware->somethingElse();
    })
    ->withExceptions(function (Exceptions $exceptions) {
        //
    })->create();
`

// TestWithMiddlewareStmts finds the ->withMiddleware() closure body inside the
// Application::configure() chain and returns its statements in source order,
// regardless of how deep in the chain the link sits.
func TestWithMiddlewareStmts(t *testing.T) {
	root := parseRoot(t, bootstrapSnippet)

	stmts := phpast.WithMiddlewareStmts(root)
	if len(stmts) != 4 {
		t.Fatalf("WithMiddlewareStmts() returned %d statements, want 4", len(stmts))
	}

	// Each statement is an expression statement wrapping a $middleware-> call;
	// reading the method names proves both the order and the unwrapping.
	var methods []string
	for _, st := range stmts {
		recv, method, _, ok := phpast.MethodCallParts(phpast.ExpressionStmt(st))
		if !ok {
			t.Fatalf("statement is not a method call: %#v", st)
		}
		if got := phpast.VariableName(recv); got != "middleware" {
			t.Errorf("receiver = %q, want %q", got, "middleware")
		}
		methods = append(methods, phpast.CallName(method))
	}

	want := []string{"alias", "appendToGroup", "append", "somethingElse"}
	if !reflect.DeepEqual(methods, want) {
		t.Errorf("closure methods = %v, want %v", methods, want)
	}
}

// TestWithMiddlewareStmtsAbsent covers the shapes that contribute nothing: a
// file with no withMiddleware link at all, a withMiddleware whose argument is
// not a closure (an exotic fluent form we do not guess, ADR 0002), and a non-Root
// input.
func TestWithMiddlewareStmtsAbsent(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "no withMiddleware link",
			src: `<?php
return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(web: __DIR__.'/../routes/web.php')
    ->create();
`,
		},
		{
			name: "argument is not a closure",
			src: `<?php
return Application::configure(basePath: dirname(__DIR__))
    ->withMiddleware($configurator)
    ->create();
`,
		},
		{
			name: "no arguments",
			src: `<?php
return Application::configure()->withMiddleware()->create();
`,
		},
		{
			name: "empty file",
			src:  `<?php`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if stmts := phpast.WithMiddlewareStmts(parseRoot(t, tt.src)); stmts != nil {
				t.Errorf("WithMiddlewareStmts() = %v, want nil", stmts)
			}
		})
	}

	if stmts := phpast.WithMiddlewareStmts(nil); stmts != nil {
		t.Errorf("WithMiddlewareStmts(nil) = %v, want nil", stmts)
	}
}

// TestWithMiddlewareStmtsArrowFn covers Laravel's short-closure variant — an
// arrow function has no body statements to read, so it yields nothing rather
// than a parse-shaped surprise.
func TestWithMiddlewareStmtsArrowFn(t *testing.T) {
	src := `<?php
return Application::configure()
    ->withMiddleware(fn (Middleware $middleware) => $middleware->alias([]))
    ->create();
`
	if stmts := phpast.WithMiddlewareStmts(parseRoot(t, src)); stmts != nil {
		t.Errorf("WithMiddlewareStmts() = %v, want nil for an arrow function", stmts)
	}
}
