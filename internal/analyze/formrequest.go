package analyze

// This file adds the third correlation of the analyze package: the
// Route→FormRequest link (ADR 0006, CONTEXT.md glossary). Where ResolveRoutes
// correlates Routes to Controllers and FindDisagreements correlates Models to
// the Schema, LinkFormRequests correlates the already-resolved Routes to the
// separately-extracted FormRequests, filling in each Route's FormRequest FQN so
// the OpenAPI renderer knows which route carries which request body.
//
// The link rides on the same two-phase symbol table as Route→Controller
// resolution (ADR 0006). Its input is:
//
//   - the routes already resolved by ResolveRoutes, each carrying the FQN of the
//     controller it dispatches to (the empty-FQN routes are simply skipped);
//   - the extracted FormRequests, the authority on which request-class FQNs
//     exist;
//   - the controller extractor's ActionParams side map (controller FQN → action
//     → parameter type-hint short names), which says what each action's typed
//     parameters are;
//   - the project-wide symbol table, consulted to resolve a parameter's short
//     type name to an FQN in the context of the declaring controller file's
//     `use` imports — never by short name alone (the anti-wrong-edge invariant);
//   - a controller-FQN → source-file map, so the symbol table resolves each
//     parameter type against the imports of the very file that declared the
//     action.
//
// False-positive safety (a deliberate decision, mirroring ResolveRoutes): a
// Route is linked ONLY when one of its action's typed parameters resolves to an
// FQN that is a genuinely-extracted FormRequest. A route whose action takes no
// typed parameter, or whose parameter resolves to some other class (a plain
// dependency-injected service, a route-model-bound Eloquent model, ...), gains
// no link and produces no error. The link is additive and never removes an
// already-set FormRequest.

import (
	"github.com/mawsis/unlaravel/internal/extract/controller"
	"github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/symbol"
)

// DefaultRequestNamespace is Laravel's conventional namespace for FormRequest
// classes referenced by their short name in a controller without an explicit
// `use` import. Parameter-type resolution applies it as a fallback ONLY when the
// declaring controller file does not import the short name and the name is not
// already fully qualified; an explicit import and an already-qualified name
// always win. The default-namespace policy lives here in the caller, not in the
// symbol table, per ADR 0006 — the same way DefaultControllerNamespace does for
// Route→Controller resolution.
const DefaultRequestNamespace = `App\Http\Requests`

// LinkFormRequests performs the Route→FormRequest correlation (ADR 0006). For
// each resolved route it looks up the typed parameters of the action it
// dispatches to (via actionParams, keyed by the route's controller FQN and its
// action name), resolves each parameter's short type name to an FQN in the
// context of the declaring controller file's `use` imports (with Laravel's
// DefaultRequestNamespace as a fallback), and — when a resolved type is a
// genuinely-extracted FormRequest — records that FormRequest's FQN on the route.
// It returns a fresh slice of routes with FormRequest filled in for every route
// that links one.
//
// routes are the routes already resolved by ResolveRoutes, so each linkable
// route carries the FQN of the controller it dispatches to; a route with an
// empty FQN (unresolved, or a non-controller action shape) is passed through
// untouched. formRequests are the classes extracted from app/Http/Requests, the
// authority on which request-class FQNs exist. actionParams is the controller
// extractor's side map (controller FQN → action → parameter type-hint short
// names in declaration order). sym is the project-wide symbol table (ADR 0006),
// consulted to resolve a parameter's short type name against the declaring
// controller file's imports. controllerFiles maps a controller FQN to the path
// of the file that declared it, so resolution reads the right file's imports —
// the anti-wrong-edge invariant (a short name is never resolved by itself).
//
// It is a pure function: it reads its arguments, allocates fresh lookups and a
// fresh result slice, and mutates neither the routes, the form requests, nor the
// maps the caller passed in. Routes are returned in input order, so the result
// is deterministic and golden-test-stable.
//
// False-positive policy: a route is linked ONLY on a genuine match — an action
// parameter whose resolved FQN is one of the extracted FormRequests. A route
// whose action takes no typed parameter, or whose parameters resolve to other
// classes, is passed through with no FormRequest and no error. When an action
// has several typed parameters, the first one that resolves to a FormRequest
// wins (Laravel injects at most one request class per action; a second is not a
// meaningful second request body). An already-set Route.FormRequest is left
// untouched.
func LinkFormRequests(
	routes []model.Route,
	formRequests []model.FormRequest,
	actionParams controller.ActionParams,
	sym *symbol.Table,
	controllerFiles map[string]string,
) []model.Route {
	requestFQNs := indexFormRequests(formRequests)

	linked := make([]model.Route, len(routes))
	for i, r := range routes {
		linked[i] = r

		if r.FormRequest != "" {
			// Already linked (defensive; nothing else sets this today): never
			// overwrite an existing edge.
			continue
		}
		if r.FQN == "" || r.Action == "" {
			// No resolved controller or readable action: nothing to look a
			// parameter up against.
			continue
		}

		paramTypes := actionParams[r.FQN][r.Action]
		if fqn, ok := matchFormRequest(paramTypes, requestFQNs, sym, controllerFiles[r.FQN]); ok {
			linked[i].FormRequest = fqn
		}
	}

	return linked
}

// matchFormRequest resolves each of an action's parameter type short names —
// against the declaring controller file's imports (fromFile), with Laravel's
// DefaultRequestNamespace as a fallback — and returns the FQN of the first one
// that names an extracted FormRequest. It reports ok == false when none of the
// parameters resolves to a known FormRequest, which is the common, unflagged
// case of an action that takes no request class.
//
// Resolution is delegated to the symbol table's ResolveWithDefault so the
// anti-wrong-edge invariant (ADR 0006) holds: a short name resolves through
// fromFile's own `use` imports, never by short name alone. The symbol table's
// found result is ignored here — a FormRequest is authoritative on its own
// existence via requestFQNs, and a request class outside the scanned set is
// simply not linked — so membership in requestFQNs, not the table's declared
// set, is the match test.
func matchFormRequest(
	paramTypes []string,
	requestFQNs map[string]struct{},
	sym *symbol.Table,
	fromFile string,
) (string, bool) {
	for _, short := range paramTypes {
		fqn, _ := sym.ResolveWithDefault(short, fromFile, DefaultRequestNamespace)
		if _, ok := requestFQNs[fqn]; ok {
			return fqn, true
		}
	}
	return "", false
}

// indexFormRequests builds a set of the extracted FormRequests' FQNs, the
// authority a resolved parameter type is matched against. The FQN is the linking
// key of ADR 0006.
func indexFormRequests(formRequests []model.FormRequest) map[string]struct{} {
	index := make(map[string]struct{}, len(formRequests))
	for _, fr := range formRequests {
		index[fr.FQN] = struct{}{}
	}
	return index
}
