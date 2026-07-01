package route

import (
	"strings"

	"github.com/mawsis/unlaravel/internal/phpast"
)

// This file holds the AST-shape reading for route extraction. It names no
// php-parser types — every read goes through internal/phpast (ADR 0003) — so it
// is the seam between the parser and the group-flattening / action-parsing logic
// in route.go. The functions here are thin, pure adapters: given the parts of a
// route expression, they return the plain Go values (URIs, verb names, middleware
// names, controller/action pairs) the extractor assembles into model.Route.

// uriArgIndex is the position of the URI string in a verb call
// Route::get(uri, action): the first argument.
const uriArgIndex = 0

// actionArgIndex is the position of the action in a verb call
// Route::get(uri, action): the second argument (an array callable or a legacy
// "C@m" string).
const actionArgIndex = 1

// resourceControllerArgIndex is the position of the controller reference in a
// resource macro Route::apiResource(name, C::class): the second argument.
const resourceControllerArgIndex = 1

// arrayCallableControllerIndex and arrayCallableActionIndex are the positions of
// the controller class-const and the action string within an array callable
// [C::class, 'method'].
const (
	arrayCallableControllerIndex = 0
	arrayCallableActionIndex     = 1
)

// staticCall decomposes a Route::<method>(...) static call into its class,
// method, and argument nodes, reporting ok=false when the expression is not a
// static call. It is a readability alias over phpast.StaticCallParts local to
// this package.
func staticCall(expr phpast.Vertex) (class, call phpast.Vertex, args []phpast.Vertex, ok bool) {
	return phpast.StaticCallParts(expr)
}

// methodCall decomposes a chained ...->method(...) call into its receiver,
// method, and argument nodes, reporting ok=false when the expression is not a
// method call. It is a readability alias over phpast.MethodCallParts local to
// this package.
func methodCall(expr phpast.Vertex) (recv, method phpast.Vertex, args []phpast.Vertex, ok bool) {
	return phpast.MethodCallParts(expr)
}

// collectGroupCtx walks the receiver chain feeding a Route::...->group(closure)
// call and folds each prefix(...) and middleware(...) link into the inherited
// context, returning the combined context the closure body should run under.
//
// The chain is walked outermost-first (fold the receiver before this link) so
// that prefixes concatenate and middleware accumulates in source-declaration
// order, matching how Laravel composes a group. It handles both link shapes the
// chain mixes: the trailing ->prefix(...) / ->middleware(...) method calls AND
// the Route::prefix(...) / Route::middleware(...) STATIC call that opens the
// chain (for example Route::middleware([...])->prefix('admin')->group(...), where
// the middleware lives on the static-call base, not a method link). A link whose
// name is neither prefix nor middleware (a stray ->name on a group, say) folds
// through unchanged; a plain Route:: base contributes nothing and terminates the
// recursion.
func collectGroupCtx(expr phpast.Vertex, ctx groupCtx) groupCtx {
	if recv, methodNode, args, ok := methodCall(expr); ok {
		ctx = collectGroupCtx(recv, ctx)
		return foldGroupLink(ctx, phpast.CallName(methodNode), args)
	}
	if class, call, args, ok := staticCall(expr); ok && phpast.IdentifierName(class) == routeFacade {
		return foldGroupLink(ctx, phpast.CallName(call), args)
	}
	return ctx
}

// foldGroupLink folds a single prefix(...) or middleware(...) link — from either
// a method call or the static-call base — into the context. Any other link name
// leaves the context unchanged.
func foldGroupLink(ctx groupCtx, name string, args []phpast.Vertex) groupCtx {
	switch name {
	case "prefix":
		return ctx.withPrefix(phpast.FirstStringArg(args))
	case "middleware":
		return ctx.withMiddleware(middlewareNames(args))
	default:
		return ctx
	}
}

// routeURI reads the URI path from a verb call's arguments (the first string
// argument). Returns "" when the call has no string URI.
func routeURI(args []phpast.Vertex) string {
	return phpast.NthStringArg(args, uriArgIndex)
}

// parseAction reads the controller short name and action from a verb call's
// action argument (ROUTE_FACTS.md). Two shapes are recognised:
//
//   - an array callable [Controller::class, 'method'] — the controller short
//     name is the last segment of the class-const, the action the string element;
//   - a legacy string "Controller@method" — split once on '@' into controller and
//     action.
//
// An unrecognised or missing action yields two empty strings, so the route is
// still emitted (with an empty controller/action the caller can surface) rather
// than dropped.
func parseAction(args []phpast.Vertex) (controller, action string) {
	if c, a, ok := arrayCallableAction(args); ok {
		return c, a
	}
	if c, a, ok := legacyStringAction(args); ok {
		return c, a
	}
	return "", ""
}

// arrayCallableAction reads an [Controller::class, 'method'] action. It reports
// ok=false when the action argument is not an array (so the caller can try the
// legacy-string shape). The controller is reduced to its last backslash segment
// so a fully-qualified [App\...\C::class, 'm'] still yields the short name.
func arrayCallableAction(args []phpast.Vertex) (controller, action string, ok bool) {
	items := phpast.ArrayItems(phpast.ArgExpr(args, actionArgIndex))
	if len(items) <= arrayCallableActionIndex {
		return "", "", false
	}
	controller = lastSegment(phpast.ClassConstClass(items[arrayCallableControllerIndex]))
	action = phpast.StringLiteral(items[arrayCallableActionIndex])
	if controller == "" && action == "" {
		return "", "", false
	}
	return controller, action, true
}

// legacyStringAction reads a legacy "Controller@method" string action, splitting
// once on '@'. It reports ok=false when the action argument is not a string.
// A string without '@' is treated as a bare controller with an empty action.
func legacyStringAction(args []phpast.Vertex) (controller, action string, ok bool) {
	s := phpast.NthStringArg(args, actionArgIndex)
	if s == "" {
		return "", "", false
	}
	parts := strings.SplitN(s, legacyActionSeparator, 2)
	controller = parts[0]
	if len(parts) == 2 {
		action = parts[1]
	}
	return controller, action, true
}

// middlewareNames reads the middleware names from a ->middleware(...) call's
// arguments, in source order. It accepts every shape Laravel does — a single
// string ->middleware('auth'), several strings ->middleware('auth', 'verified'),
// and an array ->middleware(['auth', 'throttle:api']) — by collecting every
// string argument and expanding any array argument's string elements. Order is
// preserved; nothing is sorted or de-duplicated, because determinism here is
// source order.
func middlewareNames(args []phpast.Vertex) []string {
	var names []string
	for i := range args {
		expr := phpast.ArgExpr(args, i)
		if items := phpast.ArrayItems(expr); items != nil {
			for _, it := range items {
				if s := phpast.StringLiteral(it); s != "" {
					names = append(names, s)
				}
			}
			continue
		}
		if s := phpast.StringLiteral(expr); s != "" {
			names = append(names, s)
		}
	}
	return names
}

// closureArgIndex returns the index of the closure argument in a ->group(...)
// call. Laravel's group closure is the last argument (Route::prefix(...)->group(
// $attributes, closure) is not a shape this slice handles; the closure-only
// ->group(closure) form is), so the final argument is scanned first.
func closureArgIndex(args []phpast.Vertex) int {
	for i := len(args) - 1; i >= 0; i-- {
		if phpast.ClosureStmts(phpast.ArgExpr(args, i)) != nil {
			return i
		}
	}
	return len(args) - 1
}

// lastSegment returns the final backslash-delimited segment of a PHP class
// reference — its bare short name (e.g. "App\Http\Controllers\PostController" ->
// "PostController"). A name with no backslash is returned unchanged. Controllers
// are recorded by short name at the route site; FQN resolution is phase two.
func lastSegment(name string) string {
	if i := strings.LastIndex(name, `\`); i >= 0 {
		return name[i+1:]
	}
	return name
}
