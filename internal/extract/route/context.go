package route

// groupCtx is the context a route inherits from its enclosing Route::group(...)
// blocks: the URI prefix accumulated from every ->prefix(...) on the way in, and
// the middleware names accumulated from every ->middleware(...). It is the
// carrier for the group-flattening algorithm (ROUTE_FACTS.md / ADR 0006): a
// route's final URI is its own path joined under the context prefix, and its
// final middleware is the context middleware plus any it declares itself.
//
// groupCtx is treated as an immutable value — the withers return a fresh context
// rather than mutating in place — so that descending into a group closure with a
// combined context never disturbs the sibling routes that share the outer one.
type groupCtx struct {
	// prefix is the accumulated URI prefix of all enclosing groups, already
	// joined (for example "admin" or "admin/v1"). Empty at the top level.
	prefix string
	// middleware are the middleware names inherited from all enclosing groups, in
	// outermost-to-innermost order. Empty at the top level.
	middleware []string
}

// withPrefix returns a copy of the context with prefix appended under the
// existing one via joinPrefix, so nested ->prefix() calls compose into a single
// clean path segment. The receiver is not modified.
func (c groupCtx) withPrefix(prefix string) groupCtx {
	if prefix == "" {
		return c
	}
	return groupCtx{
		prefix:     joinURI(c.prefix, prefix),
		middleware: c.middleware,
	}
}

// withMiddleware returns a copy of the context with the given middleware names
// appended after the inherited ones, preserving outermost-first order. The
// append is onto a fresh backing array so the receiver's slice is never aliased
// or mutated. An empty names list yields an unchanged copy.
func (c groupCtx) withMiddleware(names []string) groupCtx {
	if len(names) == 0 {
		return c
	}
	combined := make([]string, 0, len(c.middleware)+len(names))
	combined = append(combined, c.middleware...)
	combined = append(combined, names...)
	return groupCtx{
		prefix:     c.prefix,
		middleware: combined,
	}
}

// middlewareCopy returns a fresh copy of the context's middleware names, or an
// empty (non-nil) slice when there are none. Routes take this copy for their own
// Middleware field so no two routes share a backing array with the context or
// each other.
func (c groupCtx) middlewareCopy() []string {
	out := make([]string, 0, len(c.middleware))
	return append(out, c.middleware...)
}
