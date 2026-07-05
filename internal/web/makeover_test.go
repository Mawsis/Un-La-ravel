// Package web_test — makeover_test.go covers issue #44's layout makeover and
// the committed Overview wow moment (issue #39's variant A, judged as landing).
// Same discipline as restyle/reskin: every criterion is a verifiable property
// of the real embedded assets served by web.Handler().
//
//   - the sidebar search affordance is actually styled (it was a bare native
//     button rendering light-on-dark);
//   - the active-nav pseudo-element marker is gone: it rendered as a cyan
//     sliver on the item's left edge, which is perceptually the banned
//     side-stripe (PRODUCT.md anti-ref #2). Active state is a tinted fill;
//   - Overview owns its content: meta, tally cards, and verdict live INSIDE
//     the overview panel so working views stay data-only (dual register);
//   - the wow moment is wired: a canvas in the overview panel, a choreography
//     module that consults reduced motion, invoked from main.js;
//   - nav count badges stay out of the first-run frame (zero-count noise).
package web_test

import (
	"regexp"
	"strings"
	"testing"
)

func TestMakeover_SearchButtonStyled(t *testing.T) {
	css := fetchAsset(t, "/css/components.css")
	i := strings.Index(css, ".search-button")
	if i < 0 {
		t.Fatal("components.css has no .search-button rules — the sidebar search renders as an unstyled native (light) button")
	}
	end := i + 600
	if end > len(css) {
		end = len(css)
	}
	block := css[i:end]
	for _, want := range []string{"var(--surface-raised)", "var(--border)", "var(--text-dim)"} {
		if !strings.Contains(block, want) {
			t.Errorf(".search-button styles missing %s — the affordance must sit on the shell's own tokens", want)
		}
	}
}

func TestMakeover_NoActiveNavMarkerPseudoElement(t *testing.T) {
	layout := fetchAsset(t, "/css/layout.css")
	if regexp.MustCompile(`\.nav-views a[^{]*::before`).MatchString(layout) {
		t.Error("layout.css still declares the nav ::before marker — it rendered as a left-edge sliver (the banned side-stripe, anti-ref #2); active nav is a tinted fill, no marker element")
	}
	if !regexp.MustCompile(`aria-current="page"\]\s*\{[^}]*background`).MatchString(layout) {
		t.Error("layout.css active nav has no background fill — the active state must read as a tinted fill")
	}
}

// TestMakeover_OverviewOwnsItsContent: proj-meta, cards, and verdict must sit
// inside the overview panel, not above all panels — the stat strip haunting
// Routes/Models was a dual-register violation (PRODUCT.md: the loud and the
// quiet must not bleed into each other).
func TestMakeover_OverviewOwnsItsContent(t *testing.T) {
	html := fetchAsset(t, "/index.html")

	start := strings.Index(html, `data-panel="overview"`)
	if start < 0 {
		t.Fatal("index.html has no overview panel")
	}
	end := strings.Index(html[start:], `data-panel="er"`)
	if end < 0 {
		t.Fatal("index.html has no er panel after overview")
	}
	overviewBlock := html[start : start+end]

	for _, id := range []string{`id="proj-meta"`, `id="cards"`, `id="verdict"`, `id="overview-canvas"`} {
		if !strings.Contains(overviewBlock, id) {
			t.Errorf("overview panel does not contain %s — Overview must own its meta/tally/verdict/wow content", id)
		}
	}
}

// TestMakeover_WowMomentWired: the un-ravel constellation (issue #39 variant
// A, judged as landing) is a real served module, consulted by main.js, that
// honors reduced motion (static resolved end-state) and reads its colors from
// the token layer rather than hardcoding.
func TestMakeover_WowMomentWired(t *testing.T) {
	wow := fetchAsset(t, "/js/views/overview-wow.js")
	for _, want := range []string{"prefersReducedMotion", "--unresolved", "--resolved", "getComputedStyle"} {
		if !strings.Contains(wow, want) {
			t.Errorf("overview-wow.js missing %q — the wow must degrade under reduced motion and read thread colors from the tokens", want)
		}
	}
	mainJS := fetchAsset(t, "/js/main.js")
	if !strings.Contains(mainJS, "overview-wow.js") {
		t.Error("main.js does not import overview-wow.js — the wow moment never plays")
	}
}

func TestMakeover_BadgesHiddenBeforeFirstAnalysis(t *testing.T) {
	layout := fetchAsset(t, "/css/layout.css")
	if !regexp.MustCompile(`:has\(\.first-run\)[^{]*\{[^}]*display:\s*none`).MatchString(layout) {
		t.Error("layout.css does not hide nav badges during first-run — a column of zeros is noise before any analysis exists")
	}
}
