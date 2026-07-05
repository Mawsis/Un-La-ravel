package findings

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// TestClassify_ConventionalAuthMiddleware is the tracer bullet: a route whose
// flattened middleware stack contains Laravel's plain `auth` middleware
// authenticates. This is the single most common case — the classifier's whole
// reason to exist is to answer "does this stack authenticate?" and `auth` is the
// canonical yes.
func TestClassify_ConventionalAuthMiddleware(t *testing.T) {
	got := Classify([]string{"auth"})
	if got != model.AuthAuthenticated {
		t.Errorf("Classify([auth]) = %q, want %q", got, model.AuthAuthenticated)
	}
}

// TestClassify_GuardedAndBasicAuthVariants covers the guarded forms Laravel
// writes for API auth — `auth:sanctum`, `auth:api` (the `auth:<guard>` shape) —
// and `auth.basic`. These are all conventional authentication; the fixture's
// admin group uses `auth:sanctum`, so this is not a hypothetical.
func TestClassify_GuardedAndBasicAuthVariants(t *testing.T) {
	for _, mw := range []string{"auth:sanctum", "auth:api", "auth.basic"} {
		if got := Classify([]string{mw}); got != model.AuthAuthenticated {
			t.Errorf("Classify([%s]) = %q, want %q", mw, got, model.AuthAuthenticated)
		}
	}
}

// TestClassify_EmptyStackIsUnauthenticated covers the fixture's public reads
// (`GET /posts` has no middleware at all): an empty stack cannot authenticate,
// so the route is definitively reachable without logging in.
func TestClassify_EmptyStackIsUnauthenticated(t *testing.T) {
	if got := Classify([]string{}); got != model.AuthUnauthenticated {
		t.Errorf("Classify([]) = %q, want %q", got, model.AuthUnauthenticated)
	}
	if got := Classify(nil); got != model.AuthUnauthenticated {
		t.Errorf("Classify(nil) = %q, want %q", got, model.AuthUnauthenticated)
	}
}

// TestClassify_OnlyCustomMiddlewareIsUnknown is the precision-over-coverage case
// (ADR 0002): a stack of only middleware the classifier does not recognize — no
// conventional auth middleware present — is AuthUnknown, never guessed. A custom
// `verified` or a bare `throttle:api` might front a custom auth guard or might
// not; the classifier refuses to pretend it knows.
func TestClassify_OnlyCustomMiddlewareIsUnknown(t *testing.T) {
	cases := [][]string{
		{"verified"},
		{"throttle:api"},
		{"throttle:api", "verified"},
	}
	for _, mw := range cases {
		if got := Classify(mw); got != model.AuthUnknown {
			t.Errorf("Classify(%v) = %q, want %q", mw, got, model.AuthUnknown)
		}
	}
}

// TestClassify_AuthAmongCustomMiddlewareAuthenticates covers the fixture's admin
// group stack (`auth:sanctum` alongside `throttle:api`): auth present anywhere in
// the stack wins, so the unrecognized `throttle:api` does not demote the route to
// unknown.
func TestClassify_AuthAmongCustomMiddlewareAuthenticates(t *testing.T) {
	if got := Classify([]string{"auth:sanctum", "throttle:api"}); got != model.AuthAuthenticated {
		t.Errorf("Classify([auth:sanctum throttle:api]) = %q, want %q", got, model.AuthAuthenticated)
	}
}

// TestClassify_AuthMatchIsAnchored guards the anchoring: a custom middleware whose
// name merely CONTAINS or resembles "auth" is not mistaken for authentication.
// The match is exactly `auth`, or an `auth:` / `auth.` prefix — so `deauthorize`
// and `authenticate` (neither a conventional alias) are unrecognized, and a stack
// of only them is unknown, not a false authenticated.
func TestClassify_AuthMatchIsAnchored(t *testing.T) {
	for _, name := range []string{"deauthorize", "authenticate", "author", "reauth"} {
		if got := Classify([]string{name}); got != model.AuthUnknown {
			t.Errorf("Classify([%s]) = %q, want %q (must not match auth by substring)", name, got, model.AuthUnknown)
		}
	}
}
