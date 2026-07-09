package analyze

// This file adds the second correlation of the analyze package: Route→Controller
// resolution and the DeadRoute findings it produces (ADR 0006, CONTEXT.md
// glossary). Where FindDisagreements correlates Models to the Schema, ResolveRoutes
// correlates the separately-extracted Routes to the separately-extracted
// Controllers, filling in each Route's resolved controller FQN and reporting the
// routes whose Controller/Action edge dangles.
//
// The two-phase design of ADR 0006 lives across three collaborators: the route
// extractor (Phase 1, produces Routes with the controller reference recorded
// VERBATIM — short, fully-qualified, or imported-short as written at the route
// site), the symbol table (Phase 1, records every declared class by FQN), and
// this resolver (Phase 2, qualifies that reference to an FQN and tests whether
// that FQN names a declared Controller). Resolution of a short name ALWAYS
// happens in the context of the route files' `use` imports, never by short name
// alone — the anti-wrong-edge invariant — with Laravel's default controller
// namespace applied only as a fallback for a short name no route file imports;
// an already-qualified reference is taken as-is.
//
// Merged import context (a deliberate, documented simplification for this
// slice): the route extractor emits a flat []model.Route without recording which
// route file each came from, so this resolver resolves every route against a
// single import map merged from ALL the given route files. Laravel route files
// (routes/web.php, routes/api.php, ...) almost never alias the same controller
// short name to two different FQNs, so a merged map is safe in practice; the one
// theoretical loss is that two route files aliasing an identical short name to
// different controllers would collide (last-writer-wins across files). That is
// out of scope for this slice and noted here rather than guarded. Per-route
// source-file tracking, if ever needed, would let each route resolve against its
// own file's imports and remove even this theoretical ambiguity.

import (
	"fmt"

	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/Mawsis/Un-La-ravel/internal/symbol"
)

// DefaultControllerNamespace is Laravel's conventional namespace for controllers
// referenced by their short name in a route file without an explicit `use`
// import. Route resolution applies it as a fallback ONLY when no route file
// imports the short name and the name is not already fully qualified; an explicit
// import and an already-qualified name always win. The default-namespace policy
// lives here in the caller, not in the symbol table, per ADR 0006.
const DefaultControllerNamespace = `App\Http\Controllers`

// ResolveRoutes performs Phase-2 Route→Controller resolution (ADR 0006). For
// each route it qualifies the controller reference recorded verbatim at the
// route site (a short name, an imported-short name, or an already-qualified
// name) to a fully-qualified name, using the `use` imports merged from the given
// routeFiles (with Laravel's DefaultControllerNamespace as a fallback), then
// checks that FQN against the extracted controllers and their action lists. It
// returns a fresh slice of routes with each resolvable route's FQN filled in,
// plus a DeadRoute finding for every route whose Controller/Action edge dangles.
//
// routeFiles are the paths whose merged imports form the resolution context —
// normally the same route files the routes were extracted from. controllers are
// the classes extracted from app/Http/Controllers, the authority on which
// controller FQNs exist and which actions each declares. sym is the project-wide
// symbol table (ADR 0006): it is consulted only to confirm that a resolved FQN
// names some declared class, distinguishing a controller that exists but lives
// outside the scanned controllers set (not flagged — see the false-positive
// policy below) from one the project does not declare at all (a dead route).
//
// It is a pure function: it reads its arguments and the parsed route files,
// allocates fresh lookups and result slices, and mutates neither the routes nor
// the controllers the caller passed in. Routes and dead routes are returned in
// the input route order, so the result is deterministic and golden-test-stable.
//
// Errors: a routeFile that cannot be read or catastrophically fails to parse
// aborts with a wrapped error, because a route file whose imports go unread would
// silently mis-resolve every short controller name it should have qualified —
// turning real edges into dead ones. Recoverable per-file syntax diagnostics do
// NOT abort (the parser is fault-tolerant, ADR 0003).
//
// False-positive policy (a deliberate decision for this slice): a route is
// flagged dead ONLY on a genuine miss.
//
//   - missing_controller — the resolved FQN is neither among the extracted
//     controllers NOR declared anywhere in the symbol table. A controller that
//     the symbol table knows about but that was not among the scanned controllers
//     (for example one outside app/Http/Controllers, or an invokable resolved to
//     a non-controller class) is treated as ALIVE, not dead: its actions are
//     unknown, so flagging it would be a false alarm. This errs toward silence,
//     accepting that a genuinely broken route to such a class goes unreported
//     rather than risk crying wolf on a real class we merely did not scan for
//     actions.
//
//   - missing_action — the resolved FQN IS a scanned controller, but the route's
//     action is not among that controller's public methods. Because we hold the
//     controller's full action list, this is checked with certainty and applies
//     to every route (verb routes and resource-expanded routes alike: a resource
//     controller is expected to declare index/store/show/update/destroy, so a
//     missing one is a real gap).
//
//   - A route whose Controller is empty (an action shape the extractor could not
//     read) is left unresolved and NOT flagged: there is no short name to resolve
//     and no action to check, so it is passed through untouched rather than
//     invented into a finding.
func ResolveRoutes(
	routes []model.Route,
	controllers []model.Controller,
	sym *symbol.Table,
	routeFiles []string,
) ([]model.Route, []model.DeadRoute, error) {
	imports, err := mergeRouteImports(routeFiles)
	if err != nil {
		return nil, nil, err
	}

	byFQN := indexControllers(controllers)

	resolved := make([]model.Route, len(routes))
	var dead []model.DeadRoute

	for i, r := range routes {
		resolved[i] = r

		if r.Controller == "" {
			// No controller reference to resolve (unreadable action shape): pass
			// the route through untouched, without a finding.
			continue
		}

		fqn := resolveController(r.Controller, imports)

		ctrl, scanned := byFQN[fqn]
		switch {
		case !scanned && !sym.IsDeclared(fqn):
			dead = append(dead, missingController(r, fqn))
		case !scanned:
			// Declared elsewhere but not among the scanned controllers: alive but
			// actions unknown, so resolve the edge and do not flag it (see policy).
			resolved[i].FQN = fqn
		case !hasAction(ctrl, r.Action):
			dead = append(dead, missingAction(r, fqn))
		default:
			resolved[i].FQN = fqn
		}
	}

	return resolved, dead, nil
}

// mergeRouteImports parses each route file once and folds its `use` imports into
// a single short-name → FQN map, the merged resolution context for every route.
// Later files win on a short-name collision (last-writer-wins); such collisions
// are the documented theoretical limitation of the merged-map approach and do not
// occur in idiomatic Laravel route files. A read or catastrophic parse failure is
// returned wrapped, because unread imports would mis-resolve short names.
func mergeRouteImports(routeFiles []string) (map[string]string, error) {
	merged := make(map[string]string)
	for _, path := range routeFiles {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("analyze: read route imports %q: %w", path, err)
		}
		for short, fqn := range phpast.UseImports(res.Root) {
			merged[short] = fqn
		}
	}
	return merged, nil
}

// resolveController maps a controller short name written at a route site to an
// FQN using the merged route-file imports, applying the anti-wrong-edge and
// default-namespace policy of ADR 0006:
//
//   - an already-qualified name (containing a namespace separator) is used as-is;
//   - a short name the route files import resolves through that import;
//   - a short name no route file imports is qualified with Laravel's
//     DefaultControllerNamespace.
//
// This mirrors symbol.Table.ResolveWithDefault, but over the merged route-file
// import map rather than a single file's imports, because the route extractor
// does not record a per-route source file for this slice.
func resolveController(short string, imports map[string]string) string {
	if isQualified(short) {
		return short
	}
	if fqn, ok := imports[short]; ok {
		return fqn
	}
	return DefaultControllerNamespace + namespaceSep + short
}

// indexControllers builds a lookup from a controller's FQN to the Controller
// itself, so a resolved route FQN can be matched to the controller that owns the
// action list. The FQN is the resolution key of ADR 0006.
func indexControllers(controllers []model.Controller) map[string]model.Controller {
	index := make(map[string]model.Controller, len(controllers))
	for _, c := range controllers {
		index[c.FQN] = c
	}
	return index
}

// hasAction reports whether the controller declares action among its public
// methods. An empty action never matches, so a route whose action could not be
// read is not spuriously reported as present.
func hasAction(c model.Controller, action string) bool {
	if action == "" {
		return false
	}
	for _, a := range c.Actions {
		if a == action {
			return true
		}
	}
	return false
}

// missingController builds the DeadRoute finding for a route whose resolved
// controller FQN names no declared class.
func missingController(r model.Route, fqn string) model.DeadRoute {
	return model.DeadRoute{
		Method:     r.Method,
		URI:        r.URI,
		Controller: r.Controller,
		Action:     r.Action,
		Reason:     fmt.Sprintf("controller %s not found", quote(fqn)),
		Kind:       model.DeadRouteMissingController,
	}
}

// missingAction builds the DeadRoute finding for a route whose controller
// resolved to a scanned class that does not declare the route's action.
func missingAction(r model.Route, fqn string) model.DeadRoute {
	return model.DeadRoute{
		Method:     r.Method,
		URI:        r.URI,
		Controller: r.Controller,
		Action:     r.Action,
		Reason:     fmt.Sprintf("action %q not found on controller %s", r.Action, quote(fqn)),
		Kind:       model.DeadRouteMissingAction,
	}
}

// quote wraps an FQN in double quotes for a DeadRoute.Reason without escaping its
// namespace separators, so a class name reads naturally as
// "App\Http\Controllers\FooController" rather than with Go's %q-doubled
// backslashes. It is used only for building the human-readable Reason; the
// machine-readable Kind and the routes' FQN field carry the raw value.
func quote(fqn string) string {
	return `"` + fqn + `"`
}

// namespaceSep is PHP's namespace separator, joining Laravel's default controller
// namespace to a short class name to form an FQN. It matches the symbol package's
// separator; defined here so this file has no magic backslash literal.
const namespaceSep = `\`

// isQualified reports whether a name already carries a namespace (contains a
// namespace separator), in which case it is an FQN and must not be resolved
// through the import map — the same rule the symbol table applies, kept local so
// the default-namespace policy is wholly owned by this caller.
func isQualified(name string) bool {
	for i := 0; i < len(name); i++ {
		if name[i] == namespaceSep[0] {
			return true
		}
	}
	return false
}
