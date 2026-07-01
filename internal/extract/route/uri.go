package route

import "strings"

// uriSeparator is the single slash that joins URI path segments and prefixes a
// route's final path.
const uriSeparator = "/"

// joinURI joins two URI path fragments into a single clean, slash-prefixed path,
// collapsing the boundary slashes so no doubled or trailing slashes appear. It
// is the URI half of the group-flattening algorithm: joinURI(ctx.prefix, uri)
// applies an enclosing group's prefix to a route's own path.
//
// Each fragment is trimmed of its surrounding slashes before joining, then the
// result is re-prefixed with a single leading slash, so all of "posts",
// "/posts", "posts/" and "admin" + "users" normalise predictably:
//
//	joinURI("", "/posts")        == "/posts"
//	joinURI("admin", "/users")   == "/admin/users"
//	joinURI("admin/", "/users/") == "/admin/users"
//	joinURI("admin", "")         == "/admin"
//	joinURI("", "")              == "/"
//
// Placeholder segments such as {id} or {comment} contain no slashes and are
// carried through untouched, so parameterised paths like "/comments/{id}"
// survive joining intact. joinURI is a pure function of its arguments.
func joinURI(base, path string) string {
	base = strings.Trim(base, uriSeparator)
	path = strings.Trim(path, uriSeparator)

	switch {
	case base == "" && path == "":
		return uriSeparator
	case base == "":
		return uriSeparator + path
	case path == "":
		return uriSeparator + base
	default:
		return uriSeparator + base + uriSeparator + path
	}
}
