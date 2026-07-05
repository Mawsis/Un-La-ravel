// Package web_test — erexport_test.go covers issue #30: the ER settle animation
// and SVG/PNG export on the owned SVG diagram. Like restyle_test.go and
// motion_test.go, these tests drive the REAL embedded assets through
// web.Handler() and assert the acceptance criteria as verifiable properties of
// the served bytes — the feature lives entirely in static assets, so the
// served-asset text IS its public interface.
//
// The pixel-level export behavior (canvas rasterization, file download) is
// browser-only and is covered by the jstest unit tests for the pure pieces
// (standalone markup, raster dimensions) plus manual verification against the
// fixture app; what's asserted HERE is that the shipped shell wires the feature
// up at all: the export controls exist, the modules that implement export and
// settle are served, and the settle honors reduced motion.
package web_test

import (
	"strings"
	"testing"
)

// TestER_ExportControlsServed is the tracer bullet: the served dashboard HTML
// must carry the SVG and PNG export controls, or there is no way to trigger an
// export (acceptance: "diagram can be exported as SVG / as PNG").
func TestER_ExportControlsServed(t *testing.T) {
	html := fetchAsset(t, "/")

	for _, id := range []string{`id="er-export-svg"`, `id="er-export-png"`} {
		if !strings.Contains(html, id) {
			t.Errorf("dashboard HTML missing export control %s — no way to trigger an export", id)
		}
	}
}

// TestER_ExportModuleServed asserts the export implementation is actually
// shipped and exposes both the pure serialization entry (self-contained markup)
// and the two DOM download entries the buttons call. A served-but-empty module
// would let the buttons exist yet do nothing.
func TestER_ExportModuleServed(t *testing.T) {
	js := fetchAsset(t, "/js/views/er-export.js")

	wantExports := []string{
		"export function standaloneSvg",    // pure: self-contained markup
		"export function scaledDimensions", // pure: raster size
		"export function downloadSvg",      // DOM: vector export
		"export function downloadPng",      // DOM: raster export
	}
	for _, sym := range wantExports {
		if !strings.Contains(js, sym) {
			t.Errorf("er-export.js does not export %q", sym)
		}
	}

	// Exports must reflect the current diagram styling — the module resolves the
	// token-driven look to concrete values via getComputedStyle and inlines it,
	// rather than emitting a var(--…) that means nothing outside the app.
	if !strings.Contains(js, "getComputedStyle") {
		t.Error("er-export.js never reads getComputedStyle — exports would not carry the token-resolved styling")
	}
}

// TestER_SettleModuleServed asserts the settle geometry module ships with the
// pure entry points the browser shell drives (where a node starts, where the
// diagram centers). The DOM choreography lives in er.js; the load-bearing
// geometry is here and unit-tested, but it must actually be served.
func TestER_SettleModuleServed(t *testing.T) {
	js := fetchAsset(t, "/js/views/er-settle.js")

	for _, sym := range []string{"export function settleOffset", "export function diagramCenter"} {
		if !strings.Contains(js, sym) {
			t.Errorf("er-settle.js does not export %q", sym)
		}
	}
}

// TestER_SettleRespectsReducedMotion asserts the two-layer guard issue #30
// requires: the shell skips the settle when the user asks for reduced motion
// (er.js reads prefers-reduced-motion before animating), AND the CSS itself
// carries the base.css reduced-motion kill switch as a backstop. Either layer
// alone is a partial guarantee; the acceptance criterion wants the animation to
// not play under reduced motion at all.
func TestER_SettleRespectsReducedMotion(t *testing.T) {
	// The media-query read moved to the shared motion.js when issue #39
	// promoted the settle app-wide — er.js consults it through that module.
	erjs := fetchAsset(t, "/js/views/er.js")
	if !strings.Contains(erjs, "prefersReducedMotion") {
		t.Error("er.js never checks prefersReducedMotion — the settle would play regardless of the user's motion preference")
	}
	motionjs := fetchAsset(t, "/js/motion.js")
	if !strings.Contains(motionjs, "prefers-reduced-motion") {
		t.Error("motion.js does not read the prefers-reduced-motion media query — every JS settle guard is broken")
	}

	// The settle is timed by the --motion-settle token (single source of truth),
	// and base.css collapses every animation/transition under reduced motion, so
	// the transition-driven settle is covered by the existing kill switch.
	base := fetchAsset(t, "/css/base.css")
	if !strings.Contains(base, "prefers-reduced-motion") {
		t.Error("base.css lost its reduced-motion guard — the settle transition would still run")
	}
}

// TestER_SettleTimingIsTokenized asserts the settle's duration lives in the
// token layer (--motion-settle), consumed by components.css via var(), not
// hardcoded — the same single-source-of-truth rule motion_test.go enforces for
// every other motion in the app. This is the mechanical half of the #25→#30
// policy reversal recorded in ADR 0009.
func TestER_SettleTimingIsTokenized(t *testing.T) {
	tokens := fetchAsset(t, "/css/tokens.css")
	if !strings.Contains(tokens, "--motion-settle:") {
		t.Error("tokens.css missing --motion-settle — the settle duration must live in the token layer")
	}

	components := fetchAsset(t, "/css/components.css")
	if !strings.Contains(components, "var(--motion-settle)") {
		t.Error("components.css does not consume var(--motion-settle) — the settle timing must reference the token, not a raw duration")
	}
}
