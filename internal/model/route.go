package model

// This file defines the Route-related Node types of the Project Model
// (ADR 0001): the Route, the Controller it dispatches to, and the DeadRoute
// finding that records a Route whose Controller or Action does not resolve.
//
// A Route is one dispatchable HTTP entry point extracted from the route files
// (routes/*.php), with its group prefixes already applied to the URI and its
// inherited middleware already flattened in (see the group-flattening algorithm
// in ROUTE_FACTS.md). A Controller is one class from app/Http/Controllers,
// carrying the public method names that can serve as Actions. A DeadRoute is
// the new domain term this slice introduces (see vault CONTEXT.md): a Route
// whose Controller/Action edge dangles — the two-phase symbol-table resolution
// (ADR 0006) could not match it to a declared Controller class or public
// method.
//
// These are pure data types: no parsing, no I/O, no resolution logic. The
// two-phase resolution that produces DeadRoutes lives in an extractor/analyzer;
// this package only carries the resulting values.

// Controller is one class extracted from app/Http/Controllers within the
// Project Model. It carries the fully qualified name used to resolve Route
// edges during phase two (ADR 0006) and the public method names any Route may
// target as an Action. Actions preserve their source-declaration order.
type Controller struct {
	// Name is the controller class name as written (for example
	// "PostController").
	Name string `json:"name"`
	// FQN is the fully qualified class name (namespace + class, for example
	// "App\\Http\\Controllers\\PostController"). It is the key the symbol table
	// resolves a Route's controller reference against in phase two.
	FQN string `json:"fqn"`
	// Actions are the controller's public method names — the methods a Route may
	// dispatch to — in source-declaration order, with the constructor excluded.
	Actions []string `json:"actions"`
}

// Route is one dispatchable HTTP entry point within the Project Model,
// extracted from the route files with group prefixes applied and inherited
// middleware flattened in (ROUTE_FACTS.md). It records the controller and
// action as written in source; the resolved controller FQN is filled in during
// phase-two symbol-table resolution (ADR 0006).
type Route struct {
	// Method is the HTTP verb the route responds to, uppercased (for example
	// "GET", "POST", "PUT", "PATCH", "DELETE").
	Method string `json:"method"`
	// URI is the fully resolved request path, with all enclosing group prefixes
	// already applied (for example "/admin/comments").
	URI string `json:"uri"`
	// Controller is the controller short name exactly as written at the route
	// site (for example "PostController"), before FQN resolution.
	Controller string `json:"controller"`
	// Action is the controller method the route dispatches to (for example
	// "index").
	Action string `json:"action"`
	// Middleware are the middleware names applied to the route, including those
	// inherited from enclosing groups. Names only; alias→class resolution is out
	// of scope for this slice.
	Middleware []string `json:"middleware"`
	// Name is the route's name when one was assigned via ->name(...). Omitted
	// from JSON when empty.
	Name string `json:"name,omitempty"`
	// FQN is the fully qualified controller class name resolved in phase two via
	// the route file's use-imports (ADR 0006). Omitted from JSON when the
	// controller could not be resolved — a state also recorded as a DeadRoute.
	FQN string `json:"fqn,omitempty"`
	// FormRequest is the fully qualified name (or short name) of the FormRequest
	// class linked to this route, resolved from the dispatched action's typed
	// parameter (ADR 0006). It makes the Route→FormRequest link visible in
	// unlaravel.json. Set by the analyzer; omitted from JSON when the action
	// takes no FormRequest parameter.
	FormRequest string `json:"form_request,omitempty"`
}

// Dead-route kinds. These are the stable machine-readable values written to
// DeadRoute.Kind so consumers can branch without parsing the human-readable
// Reason. Defined once here; never hardcode the literal elsewhere.
const (
	// DeadRouteMissingController marks a Route whose controller reference could
	// not be resolved to any declared Controller class in phase two (ADR 0006).
	DeadRouteMissingController = "missing_controller"
	// DeadRouteMissingAction marks a Route whose controller resolved but whose
	// action is not among that Controller's public method names.
	DeadRouteMissingAction = "missing_action"
)

// DeadRoute is a finding that a Route's Controller/Action edge dangles: the
// two-phase resolution (ADR 0006) could not match the route to a declared
// Controller class, or matched the class but not the action method. It is the
// new domain term this slice introduces (see vault CONTEXT.md).
type DeadRoute struct {
	// Method is the HTTP verb of the offending Route (the Route.Method value).
	Method string `json:"method"`
	// URI is the fully resolved path of the offending Route (the Route.URI
	// value).
	URI string `json:"uri"`
	// Controller is the controller short name as written at the route site (the
	// Route.Controller value).
	Controller string `json:"controller"`
	// Action is the action the Route tried to dispatch to (the Route.Action
	// value).
	Action string `json:"action"`
	// Reason is a human-readable explanation, for example
	// `controller "App\Http\Controllers\FooController" not found` or
	// `action "bar" not found on controller "App\Http\Controllers\FooController"`.
	Reason string `json:"reason"`
	// Kind is the machine-readable category, one of DeadRouteMissingController or
	// DeadRouteMissingAction, so consumers can branch without parsing Reason.
	Kind string `json:"kind"`
}

// NewController returns a Controller with the given name and FQN and a non-nil
// Actions slice, so JSON serialization yields "actions": [] rather than null
// for a controller that has not yet had actions appended.
func NewController(name, fqn string) Controller {
	return Controller{
		Name:    name,
		FQN:     fqn,
		Actions: []string{},
	}
}
