// Package web_test — settlepromo_test.go covers issue #39's settle promotion:
// the ER panel's one sanctioned resolve gesture (ADR 0009) becomes the app's
// signature motion (ADR 0010, DESIGN.md §6). Two surfaces, both asserted as
// properties of the real embedded assets served by web.Handler():
//
//   - the entering view resolves in from a slightly-tangled state — the
//     panel-in keyframe animates transform as well as opacity (reversing the
//     opacity-only restriction issue #25 imposed before the promotion);
//   - a small persistent thread-mark in the sidebar re-settles on each new
//     analysis, styled thread-red→cyan and timed by the settle token.
//
// Per the PRD's testing decisions, these assert observable end-state and
// token wiring, not animation frames; reduced-motion degradation rides the
// existing base.css kill switch that motion_test.go already pins.
package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// panelInBlockRe captures the body of the @keyframes panel-in declaration.
var panelInBlockRe = regexp.MustCompile(`@keyframes\s+panel-in\s*\{([\s\S]*?)\n\}`)

// TestSettlePromotion_PanelInResolvesWithTransform: the entering panel is no
// longer a bare fade — it settles in (a small transform resolving to none),
// the same gesture family as the ER settle, still animating only the
// compositor-friendly pair (transform/opacity — DESIGN.md §6's hard rule).
func TestSettlePromotion_PanelInResolvesWithTransform(t *testing.T) {
	layout := fetchAsset(t, "/css/layout.css")

	m := panelInBlockRe.FindStringSubmatch(layout)
	if m == nil {
		t.Fatal("layout.css has no @keyframes panel-in — entering views lost their resolve-in animation")
	}
	body := m[1]

	if !strings.Contains(body, "transform") {
		t.Error("panel-in animates opacity only — the settle promotion (ADR 0010) has the view resolve in from a slightly-tangled state, so the keyframe must animate transform too")
	}
	if !strings.Contains(body, "opacity") {
		t.Error("panel-in lost its opacity fade — the resolve-in is transform AND opacity")
	}

	// Only transform/opacity may animate: any other property in the keyframe
	// body (margin, top, filter…) breaks the compositor-only rule.
	for _, decl := range regexp.MustCompile(`([a-z-]+)\s*:`).FindAllStringSubmatch(body, -1) {
		if prop := decl[1]; prop != "transform" && prop != "opacity" {
			t.Errorf("panel-in animates %q — the settle gesture animates transform/opacity only (DESIGN.md §6)", prop)
		}
	}
}

// TestSettlePromotion_SidebarThreadMark: the persistent sidebar thread-mark
// exists (decorative to AT — the health chip next to it carries the words),
// its resolve is timed by the settle token and styled with the thread roles
// (unresolved red easing to resolved cyan — PRODUCT.md ban #6 territory: an
// analysis completing IS a resolution), and the re-settle choreography module
// is served and wired into the once-per-analysis render path.
func TestSettlePromotion_SidebarThreadMark(t *testing.T) {
	html := fetchAsset(t, "/index.html")

	markRe := regexp.MustCompile(`<svg[^>]*class="thread-mark"[^>]*>`)
	tag := markRe.FindString(html)
	if tag == "" {
		t.Fatal(`index.html has no <svg class="thread-mark"> — the sidebar signature gesture is missing`)
	}
	if !strings.Contains(tag, `aria-hidden="true"`) {
		t.Error("thread-mark is not aria-hidden — it is identity motion, not information; the health chip carries the words")
	}

	components := fetchAsset(t, "/css/components.css")
	for _, want := range []string{"--motion-settle", "--unresolved", "--resolved"} {
		if !strings.Contains(components, "var("+want+")") {
			t.Errorf("components.css does not reference var(%s) — the thread-mark's settle must be token-timed and use the thread roles", want)
		}
	}
	if !strings.Contains(components, ".thread-mark") {
		t.Error("components.css has no .thread-mark rules")
	}

	// The re-settle module is a real served asset, wired from main.js's
	// once-per-analysis block, and it honors reduced motion in JS (the same
	// double-guard the ER settle uses — ADR 0009).
	threadJS := fetchAsset(t, "/js/thread-mark.js")
	if !strings.Contains(threadJS, "prefersReducedMotion") {
		t.Error("thread-mark.js does not consult prefersReducedMotion — every settle degrades to the static resolved end-state")
	}
	mainJS := fetchAsset(t, "/js/main.js")
	if !strings.Contains(mainJS, "thread-mark.js") {
		t.Error("main.js does not import thread-mark.js — the re-settle never fires on a new analysis")
	}
}
