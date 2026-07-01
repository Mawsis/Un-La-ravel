// Package routemap is the route-map Renderer (ADR 0001, ADR 0004): it turns
// the Route and DeadRoute nodes of a Project Model into a readable, aligned
// text table for the CLI.
//
// It is a pure model→string transform. It reads ONLY the in-memory model
// (model.ProjectModel) and never touches source files, the parser, or any
// other I/O. Output order follows model order exactly (Routes in discovery
// order), so identical models always produce byte-identical strings — which
// golden-file tests and deterministic CLI output rely on.
//
// The listing has one row per Route, with columns aligned to the widest cell
// in each column so the output reads as a table:
//
//	METHOD  URI                     CONTROLLER@ACTION            MIDDLEWARE        [⚠ DEAD]
//
// A Route whose Controller/Action edge dangles (recorded as a DeadRoute, ADR
// 0006) is flagged with a trailing dead marker so a reader can spot broken
// entry points at a glance.
package routemap

import (
	"strings"

	"github.com/mawsis/unlaravel/internal/model"
)

const (
	// Column headers for the aligned route table. Defined once so the header
	// row and any width computation reference the same literals.
	headerMethod     = "METHOD"
	headerURI        = "URI"
	headerAction     = "CONTROLLER@ACTION"
	headerMiddleware = "MIDDLEWARE"

	// columnGap separates adjacent columns in the aligned listing. Two spaces
	// keeps the table compact while leaving a clear gutter between columns.
	columnGap = "  "

	// deadMarker flags a row whose Route is dead (its Controller/Action edge
	// does not resolve, ADR 0006). Trailing on the row so the aligned columns
	// stay intact whether or not the marker is present.
	deadMarker = "⚠ DEAD"

	// actionSeparator joins a Route's controller and action into the single
	// "Controller@action" cell shown in the CONTROLLER@ACTION column, matching
	// Laravel's own legacy "Controller@method" spelling.
	actionSeparator = "@"

	// middlewareSeparator joins a Route's middleware names into the single
	// MIDDLEWARE cell. A comma-space keeps multiple names readable on one line.
	middlewareSeparator = ", "

	// emptyCell is the placeholder shown for a column that has no value for a
	// given Route (for example a route with no middleware), so a blank cell is
	// visible rather than an ambiguous run of spaces.
	emptyCell = "-"
)

// Render turns a whole Project Model into the aligned route-map text.
//
// It is a thin adapter over RenderRoutes that reads the model's Routes and
// DeadRoutes. A nil model renders just the header row (no data rows) rather
// than panicking. The renderer reads ONLY the in-memory model (ADR 0004); it
// never loads source files or the parser.
func Render(m *model.ProjectModel) string {
	if m == nil {
		return RenderRoutes(nil, nil)
	}
	return RenderRoutes(m.Routes, m.DeadRoutes)
}

// RenderRoutes renders the routes as an aligned, table-like text listing.
//
// Layout:
//   - a header row: METHOD  URI  CONTROLLER@ACTION  MIDDLEWARE.
//   - one row per Route, in the given (source/discovery) order, with each cell
//     padded to the widest value in its column so the columns line up.
//   - a Route that appears in dead is flagged with a trailing dead marker.
//
// The dead slice identifies which routes are dead by their (Method, URI,
// Controller, Action) tuple — the same values a DeadRoute copies from its
// Route — so the renderer needs no resolution logic of its own. Output is
// deterministic: rows follow routes' order and nothing is sorted. The returned
// string always ends in a trailing newline.
func RenderRoutes(routes []model.Route, dead []model.DeadRoute) string {
	deadSet := deadRouteKeys(dead)

	rows := make([]rowCells, 0, len(routes))
	for _, r := range routes {
		rows = append(rows, rowCells{
			method:     valueOr(r.Method),
			uri:        valueOr(r.URI),
			action:     actionCell(r),
			middleware: middlewareCell(r.Middleware),
			isDead:     isDead(deadSet, r),
		})
	}

	widths := columnWidths(rows)

	var b strings.Builder
	b.WriteString(renderRow(headerCells(), widths, false))
	for _, row := range rows {
		b.WriteString(renderRow(row, widths, row.isDead))
	}
	return b.String()
}

// rowCells is one already-formatted row of the listing: the pre-joined string
// value of each column plus whether the row's Route is dead. Keeping formatting
// (joining controller@action, joining middleware) separate from padding lets
// columnWidths and renderRow work on plain strings.
type rowCells struct {
	method     string
	uri        string
	action     string
	middleware string
	isDead     bool
}

// headerCells is the header row expressed as rowCells so it flows through the
// same width computation and padding as data rows, keeping the header aligned
// with the columns beneath it.
func headerCells() rowCells {
	return rowCells{
		method:     headerMethod,
		uri:        headerURI,
		action:     headerAction,
		middleware: headerMiddleware,
	}
}

// columnWidths returns the display width each column must be padded to: the
// maximum rune count across the header and every row's cell for that column.
// Widths are measured in runes, not bytes, so multi-byte URI or middleware
// characters do not skew the alignment.
func columnWidths(rows []rowCells) columnWidth {
	w := columnWidth{
		method:     runeLen(headerMethod),
		uri:        runeLen(headerURI),
		action:     runeLen(headerAction),
		middleware: runeLen(headerMiddleware),
	}
	for _, r := range rows {
		w.method = max(w.method, runeLen(r.method))
		w.uri = max(w.uri, runeLen(r.uri))
		w.action = max(w.action, runeLen(r.action))
		w.middleware = max(w.middleware, runeLen(r.middleware))
	}
	return w
}

// columnWidth carries the padded display width of each column.
type columnWidth struct {
	method     int
	uri        int
	action     int
	middleware int
}

// renderRow formats one row into a single line: each cell left-padded to its
// column width, joined by the column gap, with the dead marker appended when
// dead is true. The line always ends in a newline. The trailing MIDDLEWARE cell
// is not padded when the row is not dead, so lines carry no trailing whitespace.
func renderRow(r rowCells, w columnWidth, dead bool) string {
	var b strings.Builder
	b.WriteString(padRight(r.method, w.method))
	b.WriteString(columnGap)
	b.WriteString(padRight(r.uri, w.uri))
	b.WriteString(columnGap)
	b.WriteString(padRight(r.action, w.action))
	b.WriteString(columnGap)

	if dead {
		b.WriteString(padRight(r.middleware, w.middleware))
		b.WriteString(columnGap)
		b.WriteString(deadMarker)
	} else {
		b.WriteString(r.middleware)
	}

	b.WriteString("\n")
	return b.String()
}

// actionCell builds the CONTROLLER@ACTION cell for a Route as
// "Controller@action". When either half is empty the placeholder is used in its
// place so the separator never dangles against a blank side.
func actionCell(r model.Route) string {
	return valueOr(r.Controller) + actionSeparator + valueOr(r.Action)
}

// middlewareCell joins a Route's middleware names into a single cell, or the
// placeholder when the route has no middleware. Names are emitted in their
// model order (inherited-then-local), never sorted.
func middlewareCell(middleware []string) string {
	if len(middleware) == 0 {
		return emptyCell
	}
	return strings.Join(middleware, middlewareSeparator)
}

// deadRouteKeys builds the set of dead-route identity keys from the DeadRoutes
// slice, so a Route can be tested for deadness in constant time. The key is the
// (Method, URI, Controller, Action) tuple a DeadRoute copies verbatim from its
// Route.
func deadRouteKeys(dead []model.DeadRoute) map[string]struct{} {
	set := make(map[string]struct{}, len(dead))
	for _, d := range dead {
		set[routeKey(d.Method, d.URI, d.Controller, d.Action)] = struct{}{}
	}
	return set
}

// isDead reports whether a Route's identity tuple appears in the dead set.
func isDead(deadSet map[string]struct{}, r model.Route) bool {
	_, ok := deadSet[routeKey(r.Method, r.URI, r.Controller, r.Action)]
	return ok
}

// routeKey joins a Route's identity fields into a single map key. A newline
// separator cannot appear inside any of the fields, so distinct routes never
// collide on the same key.
func routeKey(method, uri, controller, action string) string {
	return method + "\n" + uri + "\n" + controller + "\n" + action
}

// valueOr returns s, or the placeholder when s is empty, so an empty field
// renders as a visible marker rather than a blank run of padding.
func valueOr(s string) string {
	if s == "" {
		return emptyCell
	}
	return s
}

// padRight left-aligns s in a field of the given rune width by appending
// spaces. A string already at least width runes wide is returned unchanged.
func padRight(s string, width int) string {
	pad := width - runeLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// runeLen returns the display width of s in runes, so multi-byte characters
// count as one column each when padding.
func runeLen(s string) int {
	return len([]rune(s))
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
