// Package web_test — tablepolish_test.go covers issue #28: Routes/Models
// table polish with inline danger flags. Like restyle_test.go, these tests
// drive the REAL embedded assets through web.Handler() and assert the
// acceptance criteria as properties of the served CSS:
//
//   - HTTP methods are color-coded via the .m-<METHOD> classes, with DELETE
//     on the danger token;
//   - long tables scroll under a sticky header;
//   - code-shaped columns (method, URI, action) are monospace;
//   - the inline danger flag (.entity-chip.danger-flag) is painted with the
//     danger token — never the cyan accent or brand red — in rest AND hover,
//     mirroring the verdict-link precedent from issue #27.
//
// The flags' markup/behavior half (cross-linking to the finding category) is
// covered by the Node unit tests in jstest/ (route-rows, model-cards, chip),
// which TestJSUnitTests runs under `go test`.
package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// TestRoutesTable_MethodColorCoding locks the HTTP-method color classes: one
// per verb, and DELETE must read as danger.
func TestRoutesTable_MethodColorCoding(t *testing.T) {
	css := fetchAsset(t, "/css/components.css")

	for _, cls := range []string{".m-GET", ".m-POST", ".m-PUT", ".m-PATCH", ".m-DELETE"} {
		if !strings.Contains(css, cls) {
			t.Errorf("components.css missing HTTP method color class %q", cls)
		}
	}
	if !ruleBodyContains(css, ".m-DELETE", "var(--danger)") {
		t.Error("components.css: .m-DELETE must use the danger token")
	}
}

// TestRoutesTable_StickyHeader asserts a long routes table scrolls inside
// .table-scroll while its header row stays pinned: the scroll container must
// cap its height and scroll vertically, and the header cells must be sticky
// with an opaque background (or rows would show through the pinned header).
func TestRoutesTable_StickyHeader(t *testing.T) {
	css := fetchAsset(t, "/css/components.css")

	if !ruleBodyContains(css, ".table-scroll", "max-height") {
		t.Error("components.css: .table-scroll needs a max-height so the table scrolls inside it (sticky can't pin against the page scroll from inside an overflow container)")
	}
	if !ruleBodyContains(css, "table.routes th", "position: sticky") &&
		!ruleBodyContains(css, "table.routes thead th", "position: sticky") {
		t.Error("components.css: routes table header cells are not position: sticky")
	}
	if !ruleBodyContains(css, "table.routes th", "background") &&
		!ruleBodyContains(css, "table.routes thead th", "background") {
		t.Error("components.css: sticky header cells need an opaque background or scrolled rows show through them")
	}
}

// TestRoutesTable_MonospaceCodeColumns locks the code-shaped columns (method,
// URI, controller@action) to the mono stack.
func TestRoutesTable_MonospaceCodeColumns(t *testing.T) {
	css := fetchAsset(t, "/css/components.css")

	if !ruleBodyContains(css, "table.routes td.method", "var(--mono)") {
		t.Error("components.css: method column is not monospace")
	}
	// uri and action share one rule in the current sheet; accept either shape.
	monoURI := ruleBodyContains(css, "table.routes td.uri", "var(--mono)") ||
		ruleBodyContains(css, "table.routes td.uri,\ntable.routes td.action", "var(--mono)")
	if !monoURI {
		t.Error("components.css: uri/action columns are not monospace")
	}
}

// TestDangerFlag_UsesDangerTokenRestAndHover mirrors issue #27's verdict-link
// test intent: a danger flag borrows .entity-chip for chip navigation but must
// be painted with the danger token in both rest and hover — a problem marker
// that turned cyan on hover would read as ordinary interactive chrome.
func TestDangerFlag_UsesDangerTokenRestAndHover(t *testing.T) {
	css := fetchAsset(t, "/css/components.css")

	if !ruleBodyContains(css, ".entity-chip.danger-flag", "var(--danger)") {
		t.Error("components.css: .entity-chip.danger-flag missing (rest state must use the danger token)")
	}
	if !ruleBodyContains(css, ".entity-chip.danger-flag:hover", "var(--danger)") {
		t.Error("components.css: .entity-chip.danger-flag:hover must keep the danger token (not fall back to the cyan accent hover)")
	}
}

// TestDeadTag_ClassRetired asserts the old inert .dead-tag span is fully gone:
// routes.js no longer emits it (the DEAD marker is a danger-flag chip now), so
// a surviving CSS rule would be dead code masking the migration.
func TestDeadTag_ClassRetired(t *testing.T) {
	if css := fetchAsset(t, "/css/components.css"); strings.Contains(css, ".dead-tag") {
		t.Error("components.css still styles .dead-tag — retired in favor of .entity-chip.danger-flag (issue #28)")
	}
	if js := fetchAsset(t, "/js/views/routes.js"); strings.Contains(js, "dead-tag") {
		t.Error("routes.js still emits .dead-tag — the DEAD marker must be the danger-flag chip")
	}
}

// ruleBodyContains reports whether a CSS rule that APPLIES TO selector has want
// inside its declaration block. Whitespace inside the declaration block is
// normalized so formatting doesn't matter.
//
// The selector may appear anywhere in a comma-separated selector list, not only
// immediately before the "{". These tests assert what the sheet GUARANTEES for
// an element ("the method column is monospace"), and grouping a selector with
// others changes nothing about that guarantee — CSS applies the rule to every
// selector in the list. An earlier version anchored the match to the "{", so
// grouping `table.routes td.method` with `.method` (to style the same badge
// outside the routes table, issues #68/#69) made this report "not monospace"
// about a sheet that was still monospacing it. That is a false negative about
// the property under test, so the matcher — not the assertion — was the thing
// to fix.
func ruleBodyContains(css, selector, want string) bool {
	// Strip comments from the WHOLE sheet first. The selector-list capture below
	// reaches back to the previous "}", so it swallows any comment sitting above
	// the rule — and this sheet comments nearly every rule. Removing them here
	// rather than per-part is what keeps the first selector in a list from
	// arriving glued to the tail of a comment.
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, " ")

	// Match the whole selector list preceding a declaration block, then check
	// whether any comma-separated part equals the selector we're asking about.
	re := regexp.MustCompile(`([^{}]+)\{([^}]*)\}`)
	for _, m := range re.FindAllStringSubmatch(css, -1) {
		if !selectorListHas(m[1], selector) {
			continue
		}
		body := regexp.MustCompile(`\s+`).ReplaceAllString(m[2], " ")
		if strings.Contains(body, want) {
			return true
		}
	}
	return false
}

// selectorListHas reports whether a comma-separated CSS selector list contains
// selector as one of its parts. Both sides are whitespace-normalized so a list
// broken across lines matches a single-line selector argument. A caller may
// still pass a multi-part selector ("a,\nb") — it is normalized the same way and
// compared against the whole list, preserving the older exact-list behavior.
func selectorListHas(list, selector string) bool {
	norm := func(s string) string {
		return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(s, " "))
	}
	want := norm(selector)
	if norm(list) == want {
		return true
	}
	for _, part := range strings.Split(list, ",") {
		if norm(part) == want {
			return true
		}
	}
	return false
}
