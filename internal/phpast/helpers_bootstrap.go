package phpast

import (
	"github.com/VKCOM/php-parser/pkg/ast"
)

// This file adds the reader the middleware extractor needs for a Laravel 11+
// bootstrap/app.php (issue #67), where the ≤10 HTTP Kernel's four properties
// were replaced by imperative calls inside a `->withMiddleware(closure)` link of
// the `Application::configure()->...->create()` chain. Locating that closure is
// the only new AST shape: once its body statements are in hand, the extractor
// reads each `$middleware->alias([...])` / `->appendToGroup(...)` / `->append(...)`
// call through the existing MethodCallParts + ArgExpr + Array* helpers, so —
// like every other consumer — it never imports php-parser (ADR 0003).

// withMiddlewareMethod is the chain link that carries the middleware
// configuration closure in a Laravel 11+ bootstrap/app.php.
const withMiddlewareMethod = "withMiddleware"

// NamedArgExprs returns the argument expressions of a call that were passed
// under any of the given PHP named-argument labels (`->web(append: [...])`),
// in the order the labels are listed, skipping labels the call does not use.
// When the call passes NO named arguments at all, it falls back to the first
// POSITIONAL argument's expression, so a caller handles both `->api([...])` and
// `->api(append: [...])` through one accessor.
//
// Laravel 11's middleware configurator is the motivating shape: several of its
// methods (->web, ->api, ->group) take `append:` and `prepend:` lists that a
// real application writes as named arguments, and a caller that indexed
// positionally would read the wrong list from `->web(prepend: [...], append:
// [...])`. Returns nil when args holds nothing readable.
func NamedArgExprs(args []Vertex, names ...string) []Vertex {
	var out []Vertex
	named := false
	for _, name := range names {
		for _, a := range args {
			arg, ok := a.(*ast.Argument)
			if !ok || arg.Name == nil {
				continue
			}
			named = true
			if IdentifierName(arg.Name) == name {
				out = append(out, arg.Expr)
			}
		}
	}
	if named {
		return out
	}
	if expr := ArgExpr(args, 0); expr != nil {
		return []Vertex{expr}
	}
	return nil
}

// WithMiddlewareStmts returns the body statements, in source order, of the
// closure passed to the `->withMiddleware(...)` link of a Laravel 11+
// bootstrap/app.php's `Application::configure()->...->create()` chain.
//
// The link may sit at any depth of the chain (before or after ->withRouting,
// ->withExceptions, ->create), and the chain itself may be returned, assigned,
// or expressed at any statement position, so the search is a whole-file walk for
// the first method call named withMiddleware whose first argument is a closure.
// The RECEIVER is deliberately not checked: it is whatever the preceding link
// returned, which is unreadable statically without resolving the fluent
// builder's types.
//
// Returns nil when root is not an *ast.Root, no such link exists (a Laravel ≤10
// project, whose middleware comes from app/Http/Kernel.php instead), the link's
// argument is absent, or that argument is not a `function (...) { ... }` closure
// — an arrow function, a variable, a first-class callable, or any other exotic
// fluent form. Those are best-effort omissions rather than guesses (ADR 0002):
// the extractor contributes no app tier from them, and the built-in backstop
// still covers framework aliases.
func WithMiddlewareStmts(root Vertex) []Vertex {
	if _, ok := root.(*ast.Root); !ok {
		return nil
	}

	finder := &withMiddlewareFinder{}
	Walk(root, finder)
	return finder.stmts
}

// withMiddlewareFinder is the Visitor that finds the first ->withMiddleware()
// closure in a file. It records that closure's body statements and then ignores
// every later match, so a file with more than one such call (not a shape Laravel
// generates) reads deterministically as its first.
type withMiddlewareFinder struct {
	NullVisitor
	stmts []Vertex
	found bool
}

// ExprMethodCall inspects each `->method(...)` call for the withMiddleware link
// and, on the first match whose first argument is a closure, records the
// closure's body statements.
func (f *withMiddlewareFinder) ExprMethodCall(n *ast.ExprMethodCall) {
	if f.found {
		return
	}
	if CallName(n.Method) != withMiddlewareMethod {
		return
	}
	stmts := ClosureStmts(ArgExpr(n.Args, 0))
	if stmts == nil {
		return
	}
	f.stmts = stmts
	f.found = true
}
