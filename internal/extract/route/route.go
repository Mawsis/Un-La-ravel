// Package route is the Route Extractor: it turns a Laravel project's
// routes/*.php files into the model's Route nodes (ADR 0001, CONTEXT.md
// glossary). Like the schema and model extractors it is a deep Extractor —
// it reads real route-file AST via internal/phpast rather than booting Laravel
// (ADR 0003) — and produces only in-memory model values; controller-FQN
// resolution, dead-route detection, and rendering are someone else's job
// (the two-phase symbol table, ADR 0006 / ADR 0007).
//
// The package boundary is a single pure entry point, Extract: route-file paths
// in, a flat []model.Route out, with no I/O beyond reading the files it is
// given. The routes are emitted in source order with each one's enclosing group
// prefixes already applied to its URI and inherited middleware already flattened
// in (the group-flattening algorithm, ROUTE_FACTS.md), and with resource macros
// (apiResource/resource) expanded to their concrete verb routes. Controllers are
// recorded VERBATIM exactly as written at the route site — fully-qualified,
// imported-short, or partially-qualified — so phase-two resolution can qualify
// the reference against the route file's `use` imports; resolving to an FQN
// happens later, in phase two (ADR 0006, issue #63).
package route

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// routeGlob matches PHP source files within a routes directory.
const routeGlob = "*.php"

// routeFacade is the class name that registers routes — Route::get(...),
// Route::apiResource(...), and so on. A static call on any other class is not a
// route declaration and is ignored.
const routeFacade = "Route"

// legacyActionSeparator splits a legacy string action "Controller@method" into
// its controller and method halves (ROUTE_FACTS.md).
const legacyActionSeparator = "@"

// httpVerbs is the set of Route:: methods that declare a single verb route. A
// static call whose method is one of these carries a URI and an action; any
// other method name is either a resource macro (see resourceMacros) or not a
// route at all.
var httpVerbs = map[string]bool{
	"get":     true,
	"post":    true,
	"put":     true,
	"patch":   true,
	"delete":  true,
	"options": true,
}

// Extract parses each route file at the given paths and returns the routes they
// declare, flattened, as model.Route values in source order.
//
// Within a file the routes come out in source order; across files, in the order
// the paths are given (callers sort paths for determinism, as ExtractDir does).
// For each route the enclosing Route::group(...) prefixes are applied to the URI
// and the inherited middleware is flattened in (ROUTE_FACTS.md); apiResource and
// resource macros are expanded to their concrete verb routes; a per-route
// ->middleware(...) or ->name(...) modifier is attached to the route it wraps.
//
// Controllers are recorded VERBATIM exactly as written at the route site (for
// example "PostController", or "App\Http\Controllers\Admin\FooController" when
// the route writes the fully-qualified name); the Route.FQN field is left empty
// here and filled in later by phase-two symbol-table resolution (ADR 0006). Routes
// whose action could not be read (an unrecognised action shape) are still
// emitted with empty Controller/Action so the caller can surface them rather
// than silently dropping an entry point.
//
// Errors: a file that cannot be read or catastrophically fails to parse aborts
// the whole extraction with a wrapped error, because a missing or unreadable
// route file means the resulting route set would be silently incomplete.
// Recoverable per-file syntax diagnostics do NOT abort — the parser is
// fault-tolerant (ADR 0003) and a partially-valid route file still yields usable
// routes.
func Extract(paths []string) ([]model.Route, error) {
	var routes []model.Route

	for _, path := range paths {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("route: extract %q: %w", path, err)
		}

		for _, stmt := range phpast.RootStmts(res.Root) {
			routes = append(routes, routesFromStmt(stmt, groupCtx{})...)
		}
	}

	return routes, nil
}

// ExtractDir is a convenience wrapper that discovers the route files under dir
// (non-recursively, matching *.php), sorts them lexically for deterministic
// discovery order, and runs Extract. A dir that does not exist or cannot be read
// yields a wrapped error.
//
// Laravel's route files live in the project's routes/ directory (web.php,
// api.php, console.php, ...); each is just a PHP file of Route:: calls, so the
// caller points ExtractDir (or Extract, with explicit paths) at that directory.
func ExtractDir(dir string) ([]model.Route, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("route: read routes dir %q: %w", dir, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ok, _ := filepath.Match(routeGlob, e.Name()); ok {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)

	return Extract(paths)
}

// routesFromStmt extracts the routes declared by a single top-level or in-closure
// statement, applying the inherited group context. A statement that is not an
// expression statement wrapping a Route:: call (a use import, a variable
// assignment, ...) yields no routes.
func routesFromStmt(stmt phpast.Vertex, ctx groupCtx) []model.Route {
	expr := phpast.ExpressionStmt(stmt)
	if expr == nil {
		return nil
	}
	return routesFromExpr(expr, ctx)
}

// routesFromExpr extracts the routes from a (possibly chained) route expression,
// applying the inherited group context. It dispatches on the two expression
// shapes a route can take:
//
//   - a static call, Route::<verb|macro>(...), which produces the route(s)
//     directly (a verb route, or the expansion of a resource macro);
//   - a method call, ...->group(closure) / ...->middleware(x) / ...->name(x),
//     which either opens a group (recurse into the closure with combined context)
//     or modifies the single route its receiver produced.
//
// Any other expression yields no routes.
func routesFromExpr(expr phpast.Vertex, ctx groupCtx) []model.Route {
	if _, _, _, ok := staticCall(expr); ok {
		return routesFromStaticCall(expr, ctx)
	}
	if _, _, _, ok := methodCall(expr); ok {
		return routesFromMethodCall(expr, ctx)
	}
	return nil
}

// routesFromStaticCall handles a Route::<method>(...) call: a verb route, a
// resource macro to expand, or a non-route facade call to ignore. The inherited
// context supplies the URI prefix and middleware.
func routesFromStaticCall(expr phpast.Vertex, ctx groupCtx) []model.Route {
	class, call, args, _ := staticCall(expr)
	if phpast.IdentifierName(class) != routeFacade {
		return nil
	}

	method := phpast.CallName(call)
	if expansion, ok := resourceMacros[method]; ok {
		return expandResource(expansion, args, ctx)
	}
	if httpVerbs[method] {
		return []model.Route{verbRoute(method, args, ctx)}
	}
	return nil
}

// routesFromMethodCall handles a chained ...->method(...) call. When the method
// is "group" it opens a route group: the receiver chain is walked only to
// collect the group's inherited prefix and middleware, then the closure body is
// re-walked with that combined context. Otherwise the method is a per-route
// modifier (->middleware / ->name): the receiver is resolved to the route(s) it
// produced and the modifier is applied to each.
func routesFromMethodCall(expr phpast.Vertex, ctx groupCtx) []model.Route {
	recv, methodNode, args, _ := methodCall(expr)
	method := phpast.CallName(methodNode)

	if method == "group" {
		inner := collectGroupCtx(expr, ctx)
		return routesFromGroupBody(args, inner)
	}

	// Per-route modifier: build the route(s) from the receiver, then decorate.
	routes := routesFromExpr(recv, ctx)
	return applyModifier(routes, method, args)
}

// routesFromGroupBody re-walks the statements of a group's closure argument with
// the combined context, flattening the nested routes. A group whose argument is
// not a closure (an array-based group definition, out of scope) yields no routes.
func routesFromGroupBody(args []phpast.Vertex, ctx groupCtx) []model.Route {
	body := phpast.ClosureStmts(phpast.ArgExpr(args, closureArgIndex(args)))
	var routes []model.Route
	for _, stmt := range body {
		routes = append(routes, routesFromStmt(stmt, ctx)...)
	}
	return routes
}

// applyModifier decorates already-built routes with a per-route ->middleware(x)
// or ->name(x) modifier. Middleware names are appended after any inherited from
// the enclosing groups (preserving order); a name replaces the route's name.
// Modifiers on a chain that produced several routes (a decorated resource macro)
// apply to every route the receiver produced, matching Laravel's behaviour. An
// unrecognised modifier leaves the routes unchanged.
func applyModifier(routes []model.Route, method string, args []phpast.Vertex) []model.Route {
	switch method {
	case "middleware":
		names := middlewareNames(args)
		for i := range routes {
			routes[i].Middleware = append(routes[i].Middleware, names...)
		}
	case "name":
		name := phpast.FirstStringArg(args)
		for i := range routes {
			routes[i].Name = name
		}
	}
	return routes
}

// verbRoute builds a single verb route from a Route::<verb>(uri, action) call
// under the inherited context: the method is the uppercased verb, the URI is the
// route's path joined under the context prefix, the middleware is a copy of the
// context's inherited middleware, and the controller/action come from the action
// argument (an array callable or a legacy "C@m" string).
func verbRoute(verb string, args []phpast.Vertex, ctx groupCtx) model.Route {
	controller, action := parseAction(args)
	return model.Route{
		Method:     strings.ToUpper(verb),
		URI:        joinURI(ctx.prefix, routeURI(args)),
		Controller: controller,
		Action:     action,
		Middleware: ctx.middlewareCopy(),
	}
}

// expandResource turns a Route::apiResource/resource(name, C::class) call into
// its concrete verb routes under the inherited context. The base name and the
// controller reference are read once; the reference is recorded VERBATIM (issue
// #63), so a sub-namespaced resource controller keeps its namespace for phase-two
// resolution (ADR 0006) rather than collapsing to a short name. Each entry in the
// macro's route set becomes a route whose URI is base+suffix joined under the
// context prefix and whose middleware is a fresh copy of the inherited middleware.
func expandResource(expansion []resourceRoute, args []phpast.Vertex, ctx groupCtx) []model.Route {
	base := phpast.FirstStringArg(args)
	controller := phpast.NthArgClassConst(args, resourceControllerArgIndex)

	routes := make([]model.Route, 0, len(expansion))
	for _, r := range expansion {
		routes = append(routes, model.Route{
			Method:     r.method,
			URI:        joinURI(ctx.prefix, base+r.suffix),
			Controller: controller,
			Action:     r.action,
			Middleware: ctx.middlewareCopy(),
		})
	}
	return routes
}
