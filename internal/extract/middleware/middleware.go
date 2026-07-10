// Package middleware assembles the Middleware node set of the Project Model
// (ADR 0001, ADR 0012) — the alias side of the middleware domain made into
// first-class nodes, distinct from any Route's application of them.
//
// This is the tracer-bullet slice (issue #64): it does NOT read the HTTP Kernel
// yet. It emits a node for every Laravel built-in alias (the backstop table in
// internal/model) and for every middleware name actually applied on a Route that
// is otherwise undeclared, so the reverse index a consumer derives ("which
// routes apply this middleware") never dangles. Reading the Kernel's own
// alias→class→group→global→priority mapping — from app/Http/Kernel.php
// (Laravel ≤10) or bootstrap/app.php (Laravel 11+) — layers on in later slices
// (issues #66/#67); until then the app-declared tier is empty.
//
// It performs no I/O and no AST work: it is a pure function of the already-
// extracted routes plus the constant backstop table. The Kernel-reading tier
// will add parser-backed inputs later, all through internal/phpast.
package middleware

import (
	"strings"

	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// Extract returns the Middleware node set for a project whose routes have
// already been extracted. The set is the union of two tiers, emitted in a fixed,
// map-free order so the output is deterministic for golden-file tests
// (ADR 0012):
//
//  1. The built-in alias backstop (domain.BuiltinMiddlewareAliases), in its
//     canonical order, every node origin "framework". These exist even when no
//     route applies them, so framework middleware an app applies but never
//     declares always has a node.
//  2. Every middleware name applied on a Route (Route.Middleware) that is not
//     already covered by tier 1, in first-appearance order across the routes
//     (routes are already in discovery order), each origin "unknown" with no
//     resolved class — the app applies it but nothing we can read declares it,
//     and precision over coverage (ADR 0002) forbids guessing its class.
//
// A name is matched on its BASE ALIAS: the parameter after the first ':' is
// stripped ("auth:sanctum" → "auth", "throttle:api" → "throttle"), so a
// parameterized application is counted against the alias it parameterizes rather
// than spawning a distinct node per parameter. Blank names are ignored, and a
// name repeated across routes yields a single node (deduped via a seen set,
// while the emitted slice is built by ordered append).
//
// The Kernel-declared tier (origin "app") is empty in this slice; when Kernel
// reading lands it is prepended ahead of tier 1 in the same ordered-append
// style, and a name it declares no longer falls through to tier 2.
func Extract(routes []domain.Route) []domain.Middleware {
	// seen tracks which base aliases already have a node, so the two tiers never
	// emit a duplicate. It is used only for membership; the emitted order comes
	// entirely from the ordered appends below, never from ranging this map.
	seen := make(map[string]struct{}, len(domain.BuiltinMiddlewareAliases))
	middlewares := make([]domain.Middleware, 0, len(domain.BuiltinMiddlewareAliases))

	// Tier 1: the built-in backstop, in canonical order.
	for _, alias := range domain.BuiltinMiddlewareAliases {
		if _, dup := seen[alias]; dup {
			continue
		}
		seen[alias] = struct{}{}
		middlewares = append(middlewares, domain.NewMiddleware(alias, domain.OriginFramework))
	}

	// Tier 2: applied-but-undeclared names, in first-appearance order.
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
