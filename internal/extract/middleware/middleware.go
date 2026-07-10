// Package middleware assembles the Middleware node set of the Project Model
// (ADR 0001, ADR 0012) — the alias side of the middleware domain made into
// first-class nodes, distinct from any Route's application of them.
//
// The node set is a three-tier union (ADR 0012): the HTTP Kernel's declared
// aliases (origin "app", class/groups/global/priority resolved) ∪ every Laravel
// built-in alias the Kernel did not declare (the backstop table in
// internal/model, origin "framework") ∪ every middleware name actually applied
// on a Route that nothing declares (origin "unknown"), so the reverse index a
// consumer derives ("which routes apply this middleware") never dangles. Reading
// the Kernel's own alias→class→group→global→priority mapping from
// app/Http/Kernel.php (Laravel ≤10, issue #66) happens in ReadKernel (kernel.go,
// the only file here that touches the filesystem, all AST access through
// internal/phpast); Laravel 11+'s bootstrap/app.php closure layers on later
// (issue #67). A nil Kernel (no ≤10 file) collapses the union to the original
// tracer-bullet behaviour: backstop ∪ applied.
//
// Extract itself performs no I/O and no AST work: it is a pure function of the
// already-extracted routes, the constant backstop table, and the already-read
// Kernel.
package middleware

import (
	"strings"

	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// Extract returns the Middleware node set for a project whose routes have
// already been extracted and whose HTTP Kernel (Laravel ≤10) has been read into
// kernel (nil when the project has no app/Http/Kernel.php). The set is the union
// of three tiers, emitted in a fixed, map-free order so the output is
// deterministic for golden-file tests (ADR 0012):
//
//  1. The Kernel-declared aliases (kernel.AliasOrder()), in declaration order,
//     each origin "app" with its class resolved from the Kernel's alias map and
//     its groups / global flag / priority read from the Kernel's group, global,
//     and priority declarations. Empty when kernel is nil. An alias here that is
//     ALSO a Laravel built-in is emitted once, in THIS tier — the built-in tier
//     skips it — so a declared "auth" carries its resolved class rather than the
//     bare framework node.
//  2. The built-in alias backstop (domain.BuiltinMiddlewareAliases), in its
//     canonical order, for every built-in the Kernel did not declare, origin
//     "framework". These exist even when no route applies them, so framework
//     middleware an app applies but never declares always has a node.
//  3. Every middleware name applied on a Route (Route.Middleware) not already
//     covered by tiers 1–2, in first-appearance order across the routes (routes
//     are already in discovery order), each origin "unknown" with no resolved
//     class — the app applies it but nothing we can read declares it, and
//     precision over coverage (ADR 0002) forbids guessing its class.
//
// A name is matched on its BASE ALIAS: the parameter after the first ':' is
// stripped ("auth:sanctum" → "auth", "throttle:api" → "throttle"), so a
// parameterized application is counted against the alias it parameterizes rather
// than spawning a distinct node per parameter. Blank names are ignored, and a
// name repeated across routes yields a single node (deduped via a seen set,
// while the emitted slice is built by ordered append).
func Extract(routes []domain.Route, kernel *Kernel) []domain.Middleware {
	// seen tracks which base aliases already have a node, so the three tiers never
	// emit a duplicate. It is used only for membership; the emitted order comes
	// entirely from the ordered appends below, never from ranging this map.
	seen := make(map[string]struct{}, len(domain.BuiltinMiddlewareAliases))
	middlewares := make([]domain.Middleware, 0, len(domain.BuiltinMiddlewareAliases))

	// Tier 1: the Kernel-declared aliases, in declaration order, fully resolved.
	for _, alias := range kernel.AliasOrder() {
		if _, dup := seen[alias]; dup {
			continue
		}
		seen[alias] = struct{}{}
		middlewares = append(middlewares, kernelNode(alias, kernel))
	}

	// Tier 2: the built-in backstop, in canonical order, minus Kernel-declared.
	for _, alias := range domain.BuiltinMiddlewareAliases {
		if _, dup := seen[alias]; dup {
			continue
		}
		seen[alias] = struct{}{}
		middlewares = append(middlewares, domain.NewMiddleware(alias, domain.OriginFramework))
	}

	// Tier 3: applied-but-undeclared names, in first-appearance order.
	for _, r := range routes {
		for _, applied := range r.Middleware {
			alias := baseAlias(applied)
			if alias == "" {
				continue
			}
			if _, dup := seen[alias]; dup {
				continue
			}
			seen[alias] = struct{}{}
			middlewares = append(middlewares, domain.NewMiddleware(alias, domain.OriginUnknown))
		}
	}

	return middlewares
}

// kernelNode builds the origin-"app" node for a Kernel-declared alias: its class
// is resolved from the Kernel's alias map, and its groups, global flag, and
// priority are looked up by that resolved class against the Kernel's group,
// global, and priority declarations. It starts from domain.NewMiddleware so the
// non-nil-empty Groups convention holds when the class belongs to no group.
func kernelNode(alias string, kernel *Kernel) domain.Middleware {
	node := domain.NewMiddleware(alias, domain.OriginApp)
	class := kernel.ClassFor(alias)
	node.Class = class
	if groups := kernel.GroupsFor(class); len(groups) > 0 {
		// Copy so the node never aliases the Kernel's backing slice.
		node.Groups = append(node.Groups, groups...)
	}
	node.Global = kernel.IsGlobal(class)
	node.Priority = kernel.PriorityFor(class)
	return node
}

// baseAlias reduces an applied middleware string to the alias it parameterizes:
// it drops everything from the first ':' onward and trims surrounding whitespace,
// so "auth:sanctum" and "throttle:60,1" become "auth" and "throttle". A string
// with no ':' is returned trimmed and unchanged. This is the same base-alias
// join a consumer uses to build the reverse index, kept here so the node set and
// the index agree on what a name means.
func baseAlias(applied string) string {
	if i := strings.IndexByte(applied, ':'); i >= 0 {
		applied = applied[:i]
	}
	return strings.TrimSpace(applied)
}
