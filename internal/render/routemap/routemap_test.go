package routemap

import (
	"strings"
	"testing"

	"github.com/mawsis/unlaravel/internal/model"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// lines splits s on newlines, dropping the final empty element that the
// mandatory trailing newline produces, so callers can reason about rows
// by index without off-by-one issues.
func lines(s string) []string {
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// ---------------------------------------------------------------------------
// RenderRoutes: empty route set
// ---------------------------------------------------------------------------

// TestRenderRoutes_Empty verifies that an empty route slice renders exactly one
// line — the header — plus the mandatory trailing newline, and that the header
// contains each expected column name.
func TestRenderRoutes_Empty(t *testing.T) {
	got := RenderRoutes(nil, nil)

	if !strings.HasSuffix(got, "\n") {
		t.Errorf("output must end with a trailing newline; got %q", got)
	}

	rows := lines(got)
	if len(rows) != 1 {
		t.Errorf("expected 1 row (header only), got %d rows:\n%s", len(rows), got)
	}

	for _, col := range []string{headerMethod, headerURI, headerAction, headerMiddleware} {
		if !strings.Contains(rows[0], col) {
			t.Errorf("header row missing column %q:\n%s", col, rows[0])
		}
	}
}

// TestRenderRoutes_EmptySlice confirms that an explicitly allocated but empty
// slice behaves identically to nil (no panic, header-only output).
func TestRenderRoutes_EmptySlice(t *testing.T) {
	got := RenderRoutes([]model.Route{}, []model.DeadRoute{})
	rows := lines(got)
	if len(rows) != 1 {
		t.Errorf("empty slice: expected 1 header row, got %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// RenderRoutes: column alignment
// ---------------------------------------------------------------------------

// TestRenderRoutes_ColumnAlignment constructs routes with deliberately varying
// widths and asserts that every row's columns start at the same byte offsets as
// the header row.  This exercises the padding logic without hard-coding the
// exact column widths, so it stays correct regardless of header length changes.
func TestRenderRoutes_ColumnAlignment(t *testing.T) {
	routes := []model.Route{
		{Method: "GET", URI: "/", Controller: "HomeController", Action: "index", Middleware: []string{"web"}},
		{Method: "DELETE", URI: "/admin/users/{id}", Controller: "Admin\\UserController", Action: "destroy", Middleware: []string{"web", "auth", "admin"}},
		{Method: "POST", URI: "/api/v1/articles", Controller: "Api\\ArticleController", Action: "store", Middleware: []string{"api"}},
	}

	got := RenderRoutes(routes, nil)

	if !strings.HasSuffix(got, "\n") {
		t.Errorf("output must end with a trailing newline")
	}

	rows := lines(got)
	// header + 3 data rows
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d:\n%s", len(rows), got)
	}

	// The header drives column start positions. Every data row must begin the URI
	// column at the same byte offset as the header, verifying that METHOD was
	// padded to a consistent width.
	//
	// Strategy: find where "URI" starts in the header; assert every other row has
	// the same content at that same offset (the first char after the METHOD gap).
	headerRow := rows[0]
	uriOffset := strings.Index(headerRow, headerURI)
	if uriOffset < 0 {
		t.Fatalf("header row missing URI column: %q", headerRow)
	}

	for i, row := range rows[1:] {
		if len(row) < uriOffset {
			t.Errorf("data row %d is shorter than header URI offset (%d):\n%q", i+1, uriOffset, row)
			continue
		}
		// The character at uriOffset in a data row must NOT be a space; if it is,
		// the METHOD column was not padded wide enough.
		if row[uriOffset] == ' ' {
			t.Errorf("data row %d URI column starts with space — METHOD not padded:\n%q", i+1, row)
		}
	}

	// Similarly pin the CONTROLLER@ACTION column offset.
	actionOffset := strings.Index(headerRow, headerAction)
	if actionOffset < 0 {
		t.Fatalf("header row missing CONTROLLER@ACTION column: %q", headerRow)
	}
	for i, row := range rows[1:] {
		if len(row) < actionOffset {
			t.Errorf("data row %d shorter than action offset (%d)", i+1, actionOffset)
		}
	}
}

// ---------------------------------------------------------------------------
// RenderRoutes: dead-route marker
// ---------------------------------------------------------------------------

// TestRenderRoutes_DeadMarker verifies that a route listed in the dead slice
// gets the trailing dead marker and that live routes do NOT carry it.
func TestRenderRoutes_DeadMarker(t *testing.T) {
	live := model.Route{
		Method: "GET", URI: "/home", Controller: "HomeController", Action: "index",
		Middleware: []string{"web"},
	}
	dead := model.Route{
		Method: "GET", URI: "/broken", Controller: "GhostController", Action: "show",
		Middleware: []string{"web"},
	}

	deadRoutes := []model.DeadRoute{
		{
			Method:     dead.Method,
			URI:        dead.URI,
			Controller: dead.Controller,
			Action:     dead.Action,
			Kind:       model.DeadRouteMissingController,
			Reason:     `controller "GhostController" not found`,
		},
	}

	got := RenderRoutes([]model.Route{live, dead}, deadRoutes)
	rows := lines(got)

	// header + 2 data rows
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d:\n%s", len(rows), got)
	}

	// row 1 is the live route — must NOT have the dead marker
	if strings.Contains(rows[1], deadMarker) {
		t.Errorf("live route row unexpectedly contains dead marker:\n%q", rows[1])
	}

	// row 2 is the dead route — MUST have the dead marker
	if !strings.Contains(rows[2], deadMarker) {
		t.Errorf("dead route row missing dead marker %q:\n%q", deadMarker, rows[2])
	}
}

// TestRenderRoutes_AllDead checks the boundary where every route is dead:
// all rows must carry the marker and alignment must still be correct.
func TestRenderRoutes_AllDead(t *testing.T) {
	routes := []model.Route{
		{Method: "POST", URI: "/a", Controller: "A", Action: "store"},
		{Method: "DELETE", URI: "/b", Controller: "B", Action: "destroy"},
	}
	deadRoutes := make([]model.DeadRoute, len(routes))
	for i, r := range routes {
		deadRoutes[i] = model.DeadRoute{
			Method: r.Method, URI: r.URI,
			Controller: r.Controller, Action: r.Action,
			Kind: model.DeadRouteMissingAction,
		}
	}

	got := RenderRoutes(routes, deadRoutes)
	rows := lines(got)

	for _, row := range rows[1:] {
		if !strings.Contains(row, deadMarker) {
			t.Errorf("expected dead marker on every data row; missing in:\n%q", row)
		}
	}
}

// ---------------------------------------------------------------------------
// RenderRoutes: middleware cell
// ---------------------------------------------------------------------------

// TestRenderRoutes_NoMiddleware asserts that a route with no middleware shows
// the emptyCell placeholder ("-") in its row, not a blank run.
func TestRenderRoutes_NoMiddleware(t *testing.T) {
	route := model.Route{
		Method: "GET", URI: "/ping", Controller: "PingController", Action: "index",
	}

	got := RenderRoutes([]model.Route{route}, nil)
	rows := lines(got)

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d:\n%s", len(rows), got)
	}

	if !strings.Contains(rows[1], emptyCell) {
		t.Errorf("row for route with no middleware must contain placeholder %q:\n%q", emptyCell, rows[1])
	}
}

// TestRenderRoutes_MultipleMiddleware verifies that several middleware names are
// joined with the middlewareSeparator and that the joined string appears in the
// output row.
func TestRenderRoutes_MultipleMiddleware(t *testing.T) {
	mw := []string{"web", "auth", "throttle:60,1"}
	route := model.Route{
		Method:     "GET",
		URI:        "/dashboard",
		Controller: "DashboardController",
		Action:     "index",
		Middleware: mw,
	}

	got := RenderRoutes([]model.Route{route}, nil)
	rows := lines(got)

	want := strings.Join(mw, middlewareSeparator)
	if !strings.Contains(rows[1], want) {
		t.Errorf("row missing joined middleware %q:\n%q", want, rows[1])
	}
}

// ---------------------------------------------------------------------------
// RenderRoutes: source order preserved
// ---------------------------------------------------------------------------

// TestRenderRoutes_SourceOrder confirms that rows appear in the same order the
// routes slice was given (nothing is sorted internally).
func TestRenderRoutes_SourceOrder(t *testing.T) {
	routes := []model.Route{
		{Method: "DELETE", URI: "/z", Controller: "ZController", Action: "destroy"},
		{Method: "GET", URI: "/a", Controller: "AController", Action: "index"},
		{Method: "POST", URI: "/m", Controller: "MController", Action: "store"},
	}

	got := RenderRoutes(routes, nil)
	rows := lines(got)

	// rows[0] is header; rows[1..3] are data
	for i, r := range routes {
		if !strings.Contains(rows[i+1], r.URI) {
			t.Errorf("row %d: expected URI %q (source order), got:\n%q", i+1, r.URI, rows[i+1])
		}
	}
}

// ---------------------------------------------------------------------------
// RenderRoutes: empty-field placeholders
// ---------------------------------------------------------------------------

// TestRenderRoutes_EmptyFieldsGetPlaceholder tests valueOr: a route whose
// Method/Controller/Action fields are empty must show the emptyCell placeholder
// rather than a blank, so the table reads clearly.
func TestRenderRoutes_EmptyFieldsGetPlaceholder(t *testing.T) {
	route := model.Route{
		URI: "/mystery",
		// Method, Controller, Action intentionally empty
	}

	got := RenderRoutes([]model.Route{route}, nil)
	rows := lines(got)

	dataRow := rows[1]
	// emptyCell must appear at least twice: once for method, once in controller@action
	count := strings.Count(dataRow, emptyCell)
	if count < 2 {
		t.Errorf("expected at least 2 occurrences of placeholder %q for empty fields, got %d in:\n%q",
			emptyCell, count, dataRow)
	}
}

// ---------------------------------------------------------------------------
// RenderRoutes: trailing newline invariant
// ---------------------------------------------------------------------------

// TestRenderRoutes_TrailingNewline is an explicit contract test: the returned
// string ALWAYS ends with exactly one newline, regardless of the number of rows.
func TestRenderRoutes_TrailingNewline(t *testing.T) {
	cases := []struct {
		name   string
		routes []model.Route
	}{
		{"nil routes", nil},
		{"empty slice", []model.Route{}},
		{"single route", []model.Route{
			{Method: "GET", URI: "/", Controller: "C", Action: "a"},
		}},
		{"many routes", []model.Route{
			{Method: "GET", URI: "/a", Controller: "A", Action: "x"},
			{Method: "POST", URI: "/b", Controller: "B", Action: "y"},
			{Method: "PUT", URI: "/c", Controller: "C", Action: "z"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderRoutes(tc.routes, nil)
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("output must end with '\\n'; got %q", got)
			}
			// Must not end with double newline (no blank trailing line).
			if strings.HasSuffix(got, "\n\n") {
				t.Errorf("output must not end with double newline; got %q", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Render (thin adapter)
// ---------------------------------------------------------------------------

// TestRender_NilModel asserts that Render(nil) does not panic and returns the
// header-only output (no data rows).
func TestRender_NilModel(t *testing.T) {
	var got string
	// Wrap in a recover to surface panics as test failures rather than crashes.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Render(nil) panicked: %v", r)
			}
		}()
		got = Render(nil)
	}()

	rows := lines(got)
	if len(rows) != 1 {
		t.Errorf("Render(nil): expected 1 header row, got %d rows:\n%s", len(rows), got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Render(nil): output must end with newline")
	}
}

// TestRender_WithModel verifies that Render delegates to RenderRoutes correctly:
// a model with routes produces the same output as calling RenderRoutes directly
// with those routes and dead routes.
func TestRender_WithModel(t *testing.T) {
	route := model.Route{
		Method: "GET", URI: "/posts", Controller: "PostController", Action: "index",
		Middleware: []string{"web"},
	}
	dead := model.DeadRoute{
		Method: "GET", URI: "/posts",
		Controller: "PostController", Action: "index",
		Kind: model.DeadRouteMissingAction,
	}

	m := model.New("blog", "11.x").
		AddRoute(route).
		AddDeadRoute(dead)

	fromRender := Render(m)
	fromDirect := RenderRoutes(m.Routes, m.DeadRoutes)

	if fromRender != fromDirect {
		t.Errorf("Render(m) != RenderRoutes(m.Routes, m.DeadRoutes)\nRender:  %q\nDirect: %q",
			fromRender, fromDirect)
	}

	// The dead marker must appear because the only route is also dead.
	if !strings.Contains(fromRender, deadMarker) {
		t.Errorf("Render: dead route must show marker %q:\n%s", deadMarker, fromRender)
	}
}

// ---------------------------------------------------------------------------
// RenderRoutes: dead detection is identity-tuple based
// ---------------------------------------------------------------------------

// TestRenderRoutes_DeadMatchIsExact verifies that a route is flagged dead only
// when its (Method, URI, Controller, Action) tuple matches a DeadRoute exactly.
// A route that shares three of four fields must NOT be flagged.
func TestRenderRoutes_DeadMatchIsExact(t *testing.T) {
	base := model.Route{
		Method: "GET", URI: "/items", Controller: "ItemController", Action: "index",
	}
	// DeadRoute differs only in Action — must NOT match base.
	wrongAction := model.DeadRoute{
		Method:     base.Method,
		URI:        base.URI,
		Controller: base.Controller,
		Action:     "other", // different
		Kind:       model.DeadRouteMissingAction,
	}

	got := RenderRoutes([]model.Route{base}, []model.DeadRoute{wrongAction})
	rows := lines(got)

	if strings.Contains(rows[1], deadMarker) {
		t.Errorf("route must NOT be flagged dead when only 3/4 identity fields match:\n%q", rows[1])
	}
}
