// Package web_test — navgroups_test.go covers issue #23: the sidebar nav
// regrouped under STRUCTURE and HEALTH headings plus the persistent health
// chip near the brand. Like restyle_test.go, these assert the acceptance
// criteria as properties of the REAL served HTML/CSS through web.Handler():
// the shipped bytes are the public interface of the static shell.
package web_test

import (
	"strings"
	"testing"
)

// TestSidebar_NavGroupedStructureThenHealth asserts the understand-then-judge
// grouping: two real heading elements (not styled divs — a11y criterion) with
// the structure views under STRUCTURE and Findings under HEALTH, in that order.
func TestSidebar_NavGroupedStructureThenHealth(t *testing.T) {
	ts := newTestServer(t)
	html := string(readBody(t, get(t, ts, "/")))

	// The group headings are real headings so screen readers expose the
	// grouping as document structure, not just visual styling.
	for _, heading := range []string{">Structure<", ">Health<"} {
		if !strings.Contains(html, "<h2") || !strings.Contains(html, heading) {
			t.Errorf("index.html missing real nav group heading %s", heading)
		}
	}

	// Ordering: Structure heading, then the five structure views, then the
	// Health heading, then Findings. Index positions on the served bytes give
	// us the under-the-correct-group property without parsing the DOM.
	markers := []string{
		">Structure<",
		`data-view="overview"`,
		`data-view="er"`,
		`data-view="models"`,
		`data-view="routes"`,
		`data-view="api"`,
		">Health<",
		`data-view="findings"`,
	}
	last := -1
	for _, m := range markers {
		idx := strings.Index(html, m)
		if idx == -1 {
			t.Fatalf("index.html missing nav marker %q", m)
		}
		if idx < last {
			t.Errorf("nav marker %q appears out of understand-then-judge order", m)
		}
		last = idx
	}

	// The Findings count badge stays always visible under the HEALTH group.
	if !strings.Contains(html, `id="badge-findings"`) {
		t.Error("index.html missing the always-visible Findings badge")
	}
}

// TestSidebar_HealthChipNearBrand asserts the persistent chip slot renders in
// the brand block, before the nav groups, so project health is always in view.
func TestSidebar_HealthChipNearBrand(t *testing.T) {
	ts := newTestServer(t)
	html := string(readBody(t, get(t, ts, "/")))

	chip := strings.Index(html, `id="health-chip"`)
	if chip == -1 {
		t.Fatal(`index.html missing the health chip slot (id="health-chip")`)
	}
	brand := strings.Index(html, `class="brand"`)
	nav := strings.Index(html, "nav-views")
	if brand == -1 || nav == -1 {
		t.Fatal("index.html missing brand block or nav-views")
	}
	if !(brand < chip && chip < nav) {
		t.Error("health chip is not between the brand block and the nav groups")
	}
}

// TestSidebar_ChipSeverityUsesStatusTokensNotBrand asserts the chip's issue
// state reads in the danger/warn status hues, never brand red — a real problem
// must not be mistakable for brand chrome (issue #23 criterion, echoing the
// token-layer rule from issue #20).
func TestSidebar_ChipSeverityUsesStatusTokensNotBrand(t *testing.T) {
	css := fetchAsset(t, "/css/layout.css")

	start := strings.Index(css, ".health-chip")
	if start == -1 {
		t.Fatal("layout.css has no .health-chip rules")
	}
	chipCSS := css[start:]

	if !strings.Contains(chipCSS, "var(--danger") && !strings.Contains(chipCSS, "var(--warn") {
		t.Error(".health-chip issue state does not use the danger/warn tokens")
	}
	if strings.Contains(chipCSS, "var(--brand") {
		t.Error(".health-chip styles reference brand red — severity must not read as brand chrome")
	}
}
