package middleware_test

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/extract/middleware"
	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// aliases projects a middleware node set down to its aliases in order, so tests
// can assert the emitted sequence compactly.
func aliases(mws []domain.Middleware) []string {
	out := make([]string, len(mws))
	for i, m := range mws {
		out[i] = m.Alias
	}
	return out
}

// byAlias indexes a node set by alias for facet assertions.
func byAlias(mws []domain.Middleware) map[string]domain.Middleware {
	out := make(map[string]domain.Middleware, len(mws))
	for _, m := range mws {
		out[m.Alias] = m
	}
	return out
}

// TestExtract_BuiltinBackstopOnly verifies that with NO routes, the extractor
// still emits exactly the built-in alias backstop table, in canonical order,
// every node framework-origin. This is the promise that framework middleware a
// project applies but never declares always has a node (ADR 0012).
func TestExtract_BuiltinBackstopOnly(t *testing.T) {
	got := middleware.Extract(nil)

	if want := domain.BuiltinMiddlewareAliases; len(got) != len(want) {
		t.Fatalf("Extract(nil) produced %d nodes, want %d (the backstop table)", len(got), len(want))
	}
	for i, alias := range domain.BuiltinMiddlewareAliases {
		if got[i].Alias != alias {
			t.Errorf("node[%d] alias = %q, want %q (canonical backstop order)", i, got[i].Alias, alias)
		}
		if got[i].Origin != domain.OriginFramework {
			t.Errorf("built-in %q origin = %q, want %q", alias, got[i].Origin, domain.OriginFramework)
		}
		if got[i].Groups == nil {
			t.Errorf("built-in %q Groups is nil, want non-nil empty slice", alias)
		}
	}
}

// TestExtract_AppliedBuiltinDoesNotDuplicate verifies that a route applying a
// name that IS a built-in (auth, throttle) adds no second node and does not flip
// its origin to unknown — the built-in node already covers it.
func TestExtract_AppliedBuiltinDoesNotDuplicate(t *testing.T) {
	routes := []domain.Route{
		{Method: "POST", URI: "/posts", Middleware: []string{"auth"}},
		{Method: "GET", URI: "/admin", Middleware: []string{"auth:sanctum", "throttle:api"}},
	}

	got := middleware.Extract(routes)

	if len(got) != len(domain.BuiltinMiddlewareAliases) {
		t.Fatalf("Extract produced %d nodes, want %d — an applied built-in must not add a node",
			len(got), len(domain.BuiltinMiddlewareAliases))
	}
	idx := byAlias(got)
	if idx["auth"].Origin != domain.OriginFramework {
		t.Errorf("auth origin = %q, want %q — an application must not downgrade a built-in", idx["auth"].Origin, domain.OriginFramework)
	}
	if idx["throttle"].Origin != domain.OriginFramework {
		t.Errorf("throttle origin = %q, want %q", idx["throttle"].Origin, domain.OriginFramework)
	}
}

// TestExtract_AppliedUndeclaredBecomesUnknown verifies an applied name that is
// neither a built-in nor (yet) an app alias yields an unknown-origin node with
// no class, appended after the whole backstop, so the reverse index never
// dangles (ADR 0012).
func TestExtract_AppliedUndeclaredBecomesUnknown(t *testing.T) {
	routes := []domain.Route{
		{Method: "GET", URI: "/admin", Middleware: []string{"tenant"}},
	}

	got := middleware.Extract(routes)

	if len(got) != len(domain.BuiltinMiddlewareAliases)+1 {
		t.Fatalf("Extract produced %d nodes, want %d (backstop + 1 unknown)",
			len(got), len(domain.BuiltinMiddlewareAliases)+1)
	}
	last := got[len(got)-1]
	if last.Alias != "tenant" {
		t.Errorf("last node alias = %q, want %q (applied name appended after the backstop)", last.Alias, "tenant")
	}
	if last.Origin != domain.OriginUnknown {
		t.Errorf("tenant origin = %q, want %q", last.Origin, domain.OriginUnknown)
	}
	if last.Class != "" {
		t.Errorf("tenant class = %q, want empty — an unknown middleware's class is not guessed", last.Class)
	}
	if last.Groups == nil {
		t.Error("tenant Groups is nil, want non-nil empty slice")
	}
}

// TestExtract_ParameterStrippedToBaseAlias verifies a parameterized application
// (tenant:acme, tenant:beta) is counted against the base alias exactly once —
// the parameter is stripped and the two uses collapse to a single node.
func TestExtract_ParameterStrippedToBaseAlias(t *testing.T) {
	routes := []domain.Route{
		{Method: "GET", URI: "/a", Middleware: []string{"tenant:acme"}},
		{Method: "GET", URI: "/b", Middleware: []string{"tenant:beta"}},
	}

	got := middleware.Extract(routes)

	unknowns := 0
	for _, m := range got {
		if m.Origin == domain.OriginUnknown {
			unknowns++
			if m.Alias != "tenant" {
				t.Errorf("unknown alias = %q, want the base %q (parameter stripped)", m.Alias, "tenant")
			}
		}
	}
	if unknowns != 1 {
		t.Errorf("got %d unknown-origin nodes, want 1 — parameterized uses collapse to one base alias", unknowns)
	}
}

// TestExtract_AppliedFirstAppearanceOrder verifies undeclared names are appended
// in the order they first appear across routes (routes are already in discovery
// order), and each appears once. This pins the applied tier's determinism.
func TestExtract_AppliedFirstAppearanceOrder(t *testing.T) {
	routes := []domain.Route{
		{Method: "GET", URI: "/1", Middleware: []string{"zebra", "auth"}},
		{Method: "GET", URI: "/2", Middleware: []string{"alpha"}},
		{Method: "GET", URI: "/3", Middleware: []string{"zebra:x", "mango"}},
	}

	got := middleware.Extract(routes)

	// The unknown tier, in order, must be zebra, alpha, mango (first appearance;
	// auth is a built-in and does not enter the unknown tier).
	var unknownOrder []string
	for _, m := range got {
		if m.Origin == domain.OriginUnknown {
			unknownOrder = append(unknownOrder, m.Alias)
		}
	}
	want := []string{"zebra", "alpha", "mango"}
	if len(unknownOrder) != len(want) {
		t.Fatalf("unknown tier = %v, want %v", unknownOrder, want)
	}
	for i := range want {
		if unknownOrder[i] != want[i] {
			t.Errorf("unknown tier = %v, want %v (first-appearance order)", unknownOrder, want)
		}
	}
}

// TestExtract_IsDeterministic verifies two calls on the same routes produce an
// identical node sequence (no map ranged for output).
func TestExtract_IsDeterministic(t *testing.T) {
	routes := []domain.Route{
		{Method: "GET", URI: "/1", Middleware: []string{"tenant", "audit", "auth"}},
		{Method: "GET", URI: "/2", Middleware: []string{"audit", "billing"}},
	}

	first := aliases(middleware.Extract(routes))
	second := aliases(middleware.Extract(routes))

	if len(first) != len(second) {
		t.Fatalf("non-deterministic length: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("non-deterministic order at %d: %q vs %q\nfirst=%v\nsecond=%v", i, first[i], second[i], first, second)
		}
	}
}

// TestExtract_BlankAndDuplicateNamesIgnored verifies an empty middleware name
// (e.g. a stray "") is skipped and a name repeated across routes yields a single
// node — the seen set dedupes but preserves first-appearance order.
func TestExtract_BlankAndDuplicateNamesIgnored(t *testing.T) {
	routes := []domain.Route{
		{Method: "GET", URI: "/1", Middleware: []string{"tenant", "", "tenant"}},
		{Method: "GET", URI: "/2", Middleware: []string{"tenant"}},
	}

	got := middleware.Extract(routes)

	count := 0
	for _, m := range got {
		if m.Alias == "tenant" {
			count++
		}
		if m.Alias == "" {
			t.Error("Extract emitted a node with an empty alias")
		}
	}
	if count != 1 {
		t.Errorf("tenant appears %d times, want exactly 1 (deduped)", count)
	}
}
