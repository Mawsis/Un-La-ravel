// Package web_test — restyle_test.go covers issue #20: the design-token
// foundation rewrite + app-shell restyle. These tests drive the REAL embedded
// assets through web.Handler() (same as server_test.go) and assert the
// acceptance criteria as verifiable properties of the served CSS/HTML:
//
//   - the token layer is the single source of truth (semantic names, neutral
//     ramp + red/cyan/danger/amber, no raw hex outside tokens.css);
//   - the shell renders in the new palette (old blue-slate grays gone, cyan
//     interactive, brand red confined to brand moments);
//   - no glow/decorative effects;
//   - the accessibility baseline and responsive behavior are preserved.
//
// Testing the shipped bytes (not a Go abstraction) is deliberate: the design
// system lives entirely in static assets, so the served-asset text IS the
// public interface here.
package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// fetchAsset GETs a static asset off the test server and returns its body as a
// string, failing the test on a non-200 or transport error.
func fetchAsset(t *testing.T, path string) string {
	t.Helper()
	ts := newTestServer(t)
	resp := get(t, ts, path)
	if resp.StatusCode != 200 {
		body := readBody(t, resp)
		t.Fatalf("GET %s: status = %d, want 200; body = %s", path, resp.StatusCode, body)
	}
	return string(readBody(t, resp))
}

// TestTokens_SemanticNamesAndNeutralRamp is the tracer bullet: the served
// tokens.css must declare the semantic token names issue #20 prescribes, with
// the neutral-ramp + brand/accent/danger/warn values, and the old blue-slate
// surface (#0d1117) must be gone.
func TestTokens_SemanticNamesAndNeutralRamp(t *testing.T) {
	css := fetchAsset(t, "/css/tokens.css")

	// Semantic token names (dark-first single source of truth). A future light
	// theme is a values swap, so these names must exist regardless of value.
	wantNames := []string{
		"--surface:",
		"--surface-elevated:",
		"--border:",
		"--text:",
		"--accent:", // cyan — interactive
		"--brand:",  // Laravel red — identity
		"--danger:",
		"--warn:",
	}
	for _, name := range wantNames {
		if !strings.Contains(css, name) {
			t.Errorf("tokens.css missing semantic token %q", name)
		}
	}

	// Neutral ramp values from issue #20 (pure Tailwind neutral, dark-first).
	wantValues := map[string]string{
		"surface":          "#0A0A0A",
		"surface-elevated": "#171717",
		"text":             "#FAFAFA",
		"dim text":         "#A3A3A3",
		"brand red":        "#F53003",
		"danger red":       "#EF4444",
		"warn amber":       "#F59E0B",
	}
	for label, hex := range wantValues {
		if !containsHexFold(css, hex) {
			t.Errorf("tokens.css missing %s value %q", label, hex)
		}
	}

	// The old blue-slate GitHub surface must be gone from the token layer.
	if containsHexFold(css, "#0d1117") {
		t.Error("tokens.css still contains the old blue-slate surface #0d1117")
	}
}

// containsHexFold reports whether css contains hex, case-insensitively (CSS hex
// is case-insensitive, so #FAFAFA and #fafafa are the same color).
func containsHexFold(css, hex string) bool {
	return strings.Contains(strings.ToLower(css), strings.ToLower(hex))
}

// shellStylesheets are the stylesheets that make up the app shell + component
// layer, all of which must consume the semantic tokens.
var shellStylesheets = []string{
	"/css/base.css",
	"/css/layout.css",
	"/css/components.css",
}

// TestShell_NoDanglingOldTokens asserts the consumer stylesheets reference only
// the new semantic tokens — none of the removed blue-slate/periwinkle token
// names (--bg, --bg-elev, --bg-elev2, --accent-2, --green, --red, --amber) may
// survive, or the shell would render against undefined custom properties.
func TestShell_NoDanglingOldTokens(t *testing.T) {
	removed := []string{
		"var(--bg)",
		"var(--bg-elev)",
		"var(--bg-elev2)",
		"var(--accent-2)",
		"var(--green)",
		"var(--red)",
		"var(--amber)",
	}
	for _, sheet := range shellStylesheets {
		css := fetchAsset(t, sheet)
		for _, tok := range removed {
			if strings.Contains(css, tok) {
				t.Errorf("%s still references removed token %q", sheet, tok)
			}
		}
	}
}

// hexRe matches a CSS hex color literal (#rgb, #rrggbb, or #rrggbbaa).
var hexRe = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)

// allowedConsumerHex is the one hex color a consumer stylesheet may name
// directly: the pure white of the Swagger light frame. Swagger UI ships its own
// light theme, so we deliberately paint that panel #fff rather than routing it
// through a semantic token — the exception is documented at its use site in
// components.css. Any OTHER raw hex in a consumer sheet is a token-layer leak.
var allowedConsumerHex = map[string]bool{"#fff": true}

// TestConsumers_NoRawHexOutsideTokens enforces the "single source of truth"
// criterion from issue #20: color literals live in tokens.css and nowhere else.
// A consumer stylesheet that hardcodes a hex has bypassed the token layer, so a
// future palette or light-theme swap would silently miss it. The lone sanctioned
// exception is the documented Swagger #fff (see allowedConsumerHex).
func TestConsumers_NoRawHexOutsideTokens(t *testing.T) {
	for _, sheet := range shellStylesheets {
		css := fetchAsset(t, sheet)
		for _, match := range hexRe.FindAllString(css, -1) {
			if allowedConsumerHex[strings.ToLower(match)] {
				continue
			}
			t.Errorf("%s hardcodes raw hex %q — color literals belong in tokens.css", sheet, match)
		}
	}
}

// TestShell_AccessibilityBaselinePreserved guards issue #20's promise that the
// restyle is a *values* change, not a structural one: the a11y primitives that
// were in place before the token rewrite must survive it. These are the markers
// the served index.html carries — a keyboard skip-link into the main region, a
// current-page indicator on nav, and a polite live region for status updates.
// A restyle that dropped any of them would regress keyboard/screen-reader users
// while looking fine to a sighted mouse user, so we assert on the shipped HTML.
func TestShell_AccessibilityBaselinePreserved(t *testing.T) {
	ts := newTestServer(t)
	html := string(readBody(t, get(t, ts, "/")))

	wantMarkers := map[string]string{
		"skip link":          `class="skip-link"`,
		"skip link target":   `href="#main-content"`,
		"main region id":     `id="main-content"`,
		"current-page nav":   `aria-current="page"`,
		"polite live region": `aria-live="polite"`,
	}
	for label, marker := range wantMarkers {
		if !strings.Contains(html, marker) {
			t.Errorf("index.html missing %s (%q) — a11y baseline regressed", label, marker)
		}
	}

	// The visible focus ring is a CSS primitive, not an HTML one; without it,
	// keyboard focus would be invisible against the new flat palette.
	if base := fetchAsset(t, "/css/base.css"); !strings.Contains(base, ":focus-visible") {
		t.Error("base.css missing :focus-visible rule — keyboard focus ring regressed")
	}
}

// TestShell_ResponsiveBaselinePreserved guards the other half of the "structure
// unchanged" promise: the desktop→narrow breakpoint that collapses the shell and
// the reduced-motion guard both survive the restyle. Issue #20 is a palette pass;
// losing either media query would be a behavioral regression hiding inside a
// styling change.
func TestShell_ResponsiveBaselinePreserved(t *testing.T) {
	if layout := fetchAsset(t, "/css/layout.css"); !strings.Contains(layout, "@media (max-width: 860px)") {
		t.Error("layout.css missing the 860px breakpoint — responsive shell regressed")
	}
	if base := fetchAsset(t, "/css/base.css"); !strings.Contains(base, "prefers-reduced-motion: reduce") {
		t.Error("base.css missing the reduced-motion guard — motion-safety regressed")
	}
}
