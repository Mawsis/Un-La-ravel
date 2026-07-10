package model

// This file defines the Middleware Node type of the Project Model (ADR 0001,
// ADR 0012): a first-class node for a request/response pipeline stage, distinct
// from its *application* on a Route.
//
// The domain glossary long separated a middleware's ALIAS (declared in the HTTP
// Kernel — the `auth` in `$middlewareAliases`) from its APPLICATION (named on a
// Route or group — the `auth` in `->middleware('auth')`). Un(la)ravel already
// modeled the application side (Route.Middleware, and the Auth state derived
// from it) but had no node for the middleware itself. This node is the alias
// side made real: it answers "what class does `auth` resolve to?", "which
// groups is it in?", "does it run on every request?", and "is it framework code
// or mine?".
//
// This is a pure data type: no parsing, no I/O, no Kernel reading. The extractor
// (internal/extract/middleware) populates these values; the reverse index
// ("which routes apply this middleware") is DERIVED at read-time by consumers
// joining Routes against these nodes, never serialized onto the node (ADR 0012).
//
// Scope note (issue #64, the tracer bullet): this slice emits a node for every
// Laravel built-in alias (the backstop table below) and for every middleware
// name actually applied on a Route, so the reverse index never dangles. Reading
// the Kernel's own alias→class→group→global→priority mapping — from
// app/Http/Kernel.php (Laravel ≤10) or bootstrap/app.php (Laravel 11+) — layers
// on in later slices (issues #66/#67); until then Class, Groups, Global, and
// Priority carry their zero values for every node.

// Middleware origins. These are the stable machine-readable values written to
// Middleware.Origin so consumers can tell "your code" from "Laravel's" without
// heuristics. Defined once here; never hardcode the literal elsewhere.
const (
	// OriginFramework marks a middleware that comes from Laravel itself — an
	// entry of the built-in alias backstop table (`auth`, `throttle`, …). A node
	// exists for it even on a project that never declares it, so the reverse
	// index is complete for framework middleware the app applies but never names
	// in its own Kernel.
	OriginFramework = "framework"
	// OriginApp marks a middleware the application itself declares — read from
	// the Kernel's alias map or a group definition. No node carries this origin
	// yet: Kernel reading is a later slice (issues #66/#67). Defined now so the
	// contract's origin vocabulary is complete and consumers can branch on it.
	OriginApp = "app"
	// OriginUnknown marks a middleware NAME applied on a Route that is neither a
	// framework built-in nor (once Kernel reading lands) an app-declared alias —
	// the app applies it but nothing declares it where we can read. Its class is
	// unresolved (empty). Precision over coverage (ADR 0002): the node exists so
	// the reverse index never dangles, but we do not guess what it resolves to.
	OriginUnknown = "unknown"
)

// BuiltinMiddlewareAliases is the canonical list of middleware aliases Laravel
// ships built in, in a fixed order. It is the backstop that guarantees a node
// exists for framework middleware an application applies but never declares in
// its own Kernel (for example a route that names `auth` on a project whose
// Kernel we have not yet read). The order is the emit order for the
// framework-origin tier (ADR 0012's determinism requirement): this slice is
// ranged in index order, never a Go map, so the serialized node set is
// byte-stable.
//
// The list mirrors the default $middlewareAliases of a fresh Laravel install
// (Illuminate\Foundation\Http\Kernel and the framework's bootstrap defaults).
// Defined once here; never hardcode these literals elsewhere.
var BuiltinMiddlewareAliases = []string{
	"auth",
	"auth.basic",
	"auth.session",
	"cache.headers",
	"can",
	"guest",
	"password.confirm",
	"precognitive",
	"signed",
	"subscribed",
	"throttle",
	"verified",
}

// Middleware is one request/response pipeline stage within the Project Model
// (ADR 0012): the node the alias side of the domain becomes. It carries what the
// tool knows about the middleware itself, distinct from any Route's application
// of it. Slice fields preserve source/discovery order; nothing here is sorted.
type Middleware struct {
	// Alias is the middleware's Kernel alias (for example "auth", "throttle") —
	// the short name a Route or group applies. Omitted from JSON when empty, the
	// state of a middleware applied by fully-qualified class name with no alias.
	Alias string `json:"alias,omitempty"`
	// Class is the fully qualified class name the alias resolves to (for example
	// "Illuminate\\Auth\\Middleware\\Authenticate"). Omitted from JSON when
	// empty — a built-in alias whose class we have not read yet (Kernel reading
	// is a later slice) or an Origin=="unknown" name we decline to guess (ADR
	// 0002).
	Class string `json:"class,omitempty"`
	// Groups are the middleware groups this middleware belongs to (for example
	// "web", "api"), in declaration order. Non-nil: an ungrouped middleware
	// serializes "groups": [] rather than null. Populated once the Kernel's group
	// definitions are read (a later slice); empty for every node in this slice.
	Groups []string `json:"groups"`
	// Origin is where the middleware comes from — one of OriginFramework,
	// OriginApp, or OriginUnknown — so a consumer can distinguish Laravel's own
	// middleware from the application's from an applied-but-undeclared name. Not
	// omitempty: every node carries an explicit origin.
	Origin string `json:"origin"`
	// Global reports whether the middleware runs on EVERY request (a member of
	// the Kernel's global $middleware stack) rather than being applied per-route
	// by alias. False for every node in this slice; populated once the global
	// stack is read (a later slice).
	Global bool `json:"global"`
	// Priority is the middleware's position in the Kernel's $middlewarePriority
	// ordering, which decides execution order when it matters. Zero for every
	// node in this slice; populated once the priority list is read (a later
	// slice).
	Priority int `json:"priority"`
}

// NewMiddleware returns a Middleware with the given alias and origin and a
// non-nil empty Groups slice, so a group-less middleware serializes
// "groups": [] rather than null, matching the project's non-nil-empty-slice
// convention. Class, Global, and Priority are left at their zero values for the
// caller to set when the Kernel provides them (a later slice).
func NewMiddleware(alias, origin string) Middleware {
	return Middleware{
		Alias:  alias,
		Origin: origin,
		Groups: []string{},
	}
}
