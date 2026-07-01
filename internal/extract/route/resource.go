package route

// This file expands Laravel's resource-route macros into the concrete routes
// they register (ROUTE_FACTS.md). Route::apiResource and Route::resource each
// stand in for a fixed set of verb+path+action combinations; the extractor emits
// one model.Route per entry, applying the enclosing group context like any other
// route.

// resourceRoute is one concrete route a resource macro expands to: the HTTP
// method, the path suffix appended to the resource base name, and the controller
// action it dispatches to. suffix is joined under the base ("comments") and any
// group prefix, so an empty suffix targets the collection ("/comments") and
// "/{id}" targets a member ("/comments/{id}").
type resourceRoute struct {
	method string
	suffix string
	action string
}

// resourceParam is the member-route placeholder segment. Laravel names it after
// the resource's singular (for example /comments/{comment}); this slice uses the
// generic {id} for now (ROUTE_FACTS.md notes the real convention is out of scope
// for MVP), kept as one constant so the placeholder is never a scattered magic
// string.
const resourceParam = "/{id}"

// apiResourceRoutes is the set of routes Route::apiResource(name, C::class)
// registers, in Laravel's canonical order: the five API-facing actions, without
// the HTML-form create/edit pages an API has no use for (ROUTE_FACTS.md).
var apiResourceRoutes = []resourceRoute{
	{method: "GET", suffix: "", action: "index"},
	{method: "POST", suffix: "", action: "store"},
	{method: "GET", suffix: resourceParam, action: "show"},
	{method: "PUT", suffix: resourceParam, action: "update"},
	{method: "DELETE", suffix: resourceParam, action: "destroy"},
}

// resourceRoutes is the set of routes Route::resource(name, C::class) registers:
// the same five API actions plus the two HTML-form pages create and edit that a
// web resource serves (ROUTE_FACTS.md). Order follows Laravel's registration:
// index, create, store, show, edit, update, destroy.
var resourceRoutes = []resourceRoute{
	{method: "GET", suffix: "", action: "index"},
	{method: "GET", suffix: "/create", action: "create"},
	{method: "POST", suffix: "", action: "store"},
	{method: "GET", suffix: resourceParam, action: "show"},
	{method: "GET", suffix: resourceParam + "/edit", action: "edit"},
	{method: "PUT", suffix: resourceParam, action: "update"},
	{method: "DELETE", suffix: resourceParam, action: "destroy"},
}

// resourceMacros maps the Route:: macro name to the route set it expands to. A
// verb name absent from this map is not a resource macro. Keeping the mapping in
// one place lets the caller test membership and fetch the expansion in a single
// lookup.
var resourceMacros = map[string][]resourceRoute{
	"apiResource": apiResourceRoutes,
	"resource":    resourceRoutes,
}
