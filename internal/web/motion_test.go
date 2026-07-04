// Package web_test — motion_test.go covers issue #25's functional-motion pass.
// Like restyle_test.go, these tests drive the REAL embedded assets through
// web.Handler() and assert the acceptance criteria as verifiable properties of
// the served bytes:
//
//   - motion durations/easing live in the token layer (single source of truth,
//     ≈150–200ms, native-feeling — no slow decorative timing);
//   - consumer stylesheets animate via the motion tokens, never a raw duration;
//   - no decorative animation (keyframes pinned to a small functional allowlist);
//   - motion respects prefers-reduced-motion.
//
// Issue #25's interim ER theming (Mermaid "base" theme + erThemeVariables) was
// superseded by the hand-rolled SVG ER renderer in issue #26, which removed
// Mermaid entirely; its test lived here and was dropped with that engine swap.
package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// TestMotionTokens_DeclaredInTokenLayer is the tracer bullet: tokens.css must
// declare the motion tokens (fast hover feedback, view/panel duration, easing)
// so every transition in the app has exactly one place to change — the same
// single-source-of-truth rule issue #20 established for color.
func TestMotionTokens_DeclaredInTokenLayer(t *testing.T) {
	css := fetchAsset(t, "/css/tokens.css")

	wantNames := []string{
		"--motion-fast:", // hover/press feedback
		"--motion-view:", // view/panel/filter changes
		"--motion-ease:", // shared easing curve
	}
	for _, name := range wantNames {
		if !strings.Contains(css, name) {
			t.Errorf("tokens.css missing motion token %q", name)
		}
	}

	// The issue prescribes fast, native-feeling motion (≈150–200ms).
	wantValues := map[string]string{
		"fast duration": "150ms",
		"view duration": "200ms",
	}
	for label, val := range wantValues {
		if !strings.Contains(css, val) {
			t.Errorf("tokens.css missing %s %q", label, val)
		}
	}
}

// TestMotion_HoverAndViewSurfacesUseTokens asserts the functional-motion pass
// actually reaches the surfaces issue #25 names: hover feedback (nav, buttons,
// chips) transitions at --motion-fast, and view/panel changes animate at
// --motion-view. Consumers must reference the tokens — a consumer that types
// its own duration has bypassed the single source of truth.
func TestMotion_HoverAndViewSurfacesUseTokens(t *testing.T) {
	layout := fetchAsset(t, "/css/layout.css")
	components := fetchAsset(t, "/css/components.css")

	// Hover feedback: both the shell (nav links, header buttons) and the
	// component layer (chips, cards, table rows) transition at --motion-fast.
	for sheet, css := range map[string]string{"layout.css": layout, "components.css": components} {
		if !strings.Contains(css, "var(--motion-fast)") {
			t.Errorf("%s has no var(--motion-fast) transition — hover feedback not wired to the motion tokens", sheet)
		}
	}

	// View/panel changes: the entering panel animates at --motion-view.
	if !strings.Contains(layout, "var(--motion-view)") {
		t.Error("layout.css has no var(--motion-view) — view/panel changes not wired to the motion tokens")
	}
}

// keyframesRe matches a @keyframes declaration and captures the animation name.
var keyframesRe = regexp.MustCompile(`@keyframes\s+([a-zA-Z0-9_-]+)`)

// allowedKeyframes are the only animations the app may define — each one
// functional (state the user caused), none decorative. A new keyframe added
// anywhere else is exactly the count-up/stagger/settle decoration issue #25
// rules out, so it must be argued into this list, not slipped past it.
var allowedKeyframes = map[string]bool{
	"spin":           true, // loading spinner — in-progress feedback
	"er-focus-pulse": true, // entity-focus highlight — cross-navigation landing marker
	"panel-in":       true, // entering view fade — display can't transition
}

// durationRe matches a raw CSS time literal (150ms, 0.7s, 1.6s…).
var durationRe = regexp.MustCompile(`\b\d+(?:\.\d+)?m?s\b`)

// allowedConsumerDurations are the raw time literals a consumer stylesheet may
// contain. Everything else must come from the motion tokens:
//   - 0.001ms is the prefers-reduced-motion kill switch in base.css — its whole
//     job is to be a constant, not a themed value;
//   - 0.7s is the loading spinner's rotation period — a continuous indicator,
//     not a transition, so the 150–200ms motion scale doesn't apply to it.
var allowedConsumerDurations = map[string]bool{"0.001ms": true, "0.7s": true}

// TestMotion_NoDecorativeAnimationOrRawDurations enforces issue #25's "no
// decorative animation" criterion as two checkable properties of every
// consumer stylesheet: (1) only the allowlisted functional keyframes exist;
// (2) no raw duration literals — timing lives in tokens.css (same
// single-source rule TestConsumers_NoRawHexOutsideTokens applies to color).
func TestMotion_NoDecorativeAnimationOrRawDurations(t *testing.T) {
	for _, sheet := range shellStylesheets {
		css := fetchAsset(t, sheet)

		for _, m := range keyframesRe.FindAllStringSubmatch(css, -1) {
			if !allowedKeyframes[m[1]] {
				t.Errorf("%s defines non-allowlisted @keyframes %q — decorative animation is out (issue #25)", sheet, m[1])
			}
		}

		for _, d := range durationRe.FindAllString(css, -1) {
			if !allowedConsumerDurations[d] {
				t.Errorf("%s hardcodes duration %q — timing belongs in tokens.css motion tokens", sheet, d)
			}
		}
	}
}
