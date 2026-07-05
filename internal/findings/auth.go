package findings

import (
	"strings"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// Classify answers the one question the auth-coverage slice is built on (issue
// #50): does a route's already-group-flattened middleware stack authenticate?
// It is a pure function over the middleware names — no AST, no I/O — returning
// one of model.AuthAuthenticated, model.AuthUnauthenticated, or model.AuthUnknown.
//
// Precision over coverage (ADR 0002): the classifier matches only Laravel's
// CONVENTIONAL auth middleware. An unrecognized custom middleware is reported as
// unknown, never guessed to authenticate or not — a false "authenticated" would
// hide a real hole, and a false "unauthenticated" would cry wolf on a route that
// a custom guard actually protects.
func Classify(middleware []string) string {
	// A conventional auth middleware anywhere in the stack settles it: the route
	// authenticates regardless of whatever else is layered on.
	for _, name := range middleware {
		if isAuthMiddleware(name) {
			return model.AuthAuthenticated
		}
	}
	// No auth middleware found. Distinguish two very different states: an EMPTY
	// stack is definitively unauthenticated (nothing there could guard it), but a
	// non-empty stack of only UNRECOGNIZED middleware is unknown — that custom
	// middleware might front a custom auth guard, and precision over coverage
	// (ADR 0002) forbids guessing either way.
	if len(middleware) == 0 {
		return model.AuthUnauthenticated
	}
	return model.AuthUnknown
}

// isAuthMiddleware reports whether a single middleware name is one of Laravel's
// conventional authentication middlewares:
//
//   - `auth`          — the default session/guard auth middleware
//   - `auth:<guard>`  — a guarded form, e.g. `auth:sanctum`, `auth:api` (the
//     part after the colon names the guard, Sanctum/Passport included)
//   - `auth.basic`    — HTTP Basic auth
//
// Matching is anchored: the name must be exactly `auth` or begin with `auth:` /
// `auth.` so an unrelated custom middleware whose name merely contains "auth"
// (for example `deauthorize`) is not mistaken for authentication.
func isAuthMiddleware(name string) bool {
	return name == "auth" ||
		strings.HasPrefix(name, "auth:") ||
		strings.HasPrefix(name, "auth.")
}
