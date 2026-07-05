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

// TestTokens_ThreadRolesAndWarmedNeutrals is the tracer bullet for the design
// system migration (issue #37 / DESIGN.md §1): the served tokens.css must
// declare the thread-role tokens (--resolved / --unresolved / --brand), author
// every neutral in warmed OKLCH (~25° hue), and carry no trace of the retired
// --accent name or the Tailwind-default neutral ramp.
func TestTokens_ThreadRolesAndWarmedNeutrals(t *testing.T) {
	css := fetchAsset(t, "/css/tokens.css")

	// Semantic token names. The thread metaphor is carried by *state roles*
	// (resolved/unresolved), not a decorative accent; a future light theme is
	// still a values swap, so these names must exist regardless of value.
	wantNames := []string{
		"--surface:",
		"--surface-elevated:",
		"--border:",
		"--text:",
		"--resolved:",   // cyan — "the tool resolved this"
		"--unresolved:", // brand-adjacent red — "still tangled"
		"--brand:",      // Laravel red — identity
		"--danger:",
		"--warn:",
	}
	for _, name := range wantNames {
		if !strings.Contains(css, name) {
			t.Errorf("tokens.css missing semantic token %q", name)
		}
	}

	// The retired accent name must be gone from the token layer — a consumer
	// referencing var(--accent) would silently resolve to nothing.
	for _, retired := range []string{"--accent:", "--accent-hover:"} {
		if strings.Contains(css, retired) {
			t.Errorf("tokens.css still declares retired token %q — renamed to the resolved role", retired)
		}
	}

	// Neutrals are authored in OKLCH, warmed toward the brand hue (DESIGN.md
	// §1). Assert the authored values for the poles of the ramp.
	wantValues := map[string]string{
		"surface":          "oklch(0.145 0.008 25)",
		"surface-elevated": "oklch(0.205 0.009 25)",
		"text":             "oklch(0.97 0.004 25)",
		"dim text":         "oklch(0.72 0.008 25)",
		"brand red":        "#F53003",
	}
	for label, val := range wantValues {
		if !containsHexFold(css, val) {
			t.Errorf("tokens.css missing %s value %q", label, val)
		}
	}

	// The Tailwind-default pure-neutral ramp is anti-reference #1: its
	// signature grays must be gone from the token layer.
	for _, hex := range []string{"#0d1117", "#0a0a0a", "#171717", "#262626", "#404040", "#525252", "#fafafa", "#a3a3a3"} {
		if containsHexFold(css, hex) {
			t.Errorf("tokens.css still contains Tailwind-default neutral %q — neutrals are authored in warmed OKLCH", hex)
		}
	}
}

// containsHexFold reports whether css contains hex, case-insensitively (CSS hex
// is case-insensitive, so #FAFAFA and #fafafa are the same color).
func containsHexFold(css, hex string) bool {
	return strings.Contains(strings.ToLower(css), strings.ToLower(hex))
}

// TestTokens_ModularTypeScale asserts the type scale is the real ~1.25 modular
// scale from DESIGN.md §2: body bumps to 15px for legibility, the top steps
// out-scale body (19/24/30/38), and the display sizes exist as NAMED tokens
// (--text-2xl / --text-3xl / --text-hero) so headings scale through the system
// instead of ad-hoc pixel literals.
func TestTokens_ModularTypeScale(t *testing.T) {
	css := fetchAsset(t, "/css/tokens.css")

	wantScale := map[string]string{
		"--text-2xs:":  "11px",
		"--text-xs:":   "12px",
		"--text-sm:":   "13px",
		"--text-base:": "15px",
		"--text-lg:":   "19px",
		"--text-xl:":   "24px",
		"--text-2xl:":  "30px",
		"--text-3xl:":  "38px",
	}
	for name, px := range wantScale {
		if !strings.Contains(css, name+" "+px) {
			t.Errorf("tokens.css missing type-scale step %q = %s", name, px)
		}
	}

	// The hero step is fluid — assert the token exists and is a clamp(), not a
	// fixed literal, so the wow moment scales with the viewport.
	if !regexp.MustCompile(`--text-hero:\s*clamp\(`).MatchString(css) {
		t.Error("tokens.css missing fluid --text-hero: clamp(...) display token")
	}
}

// fontSizeLiteralRe matches a font-size declaration whose value is a raw px
// literal (including inside clamp()), i.e. one that bypasses the type-scale
// tokens.
var fontSizeLiteralRe = regexp.MustCompile(`font-size:\s*[^;]*\d+px`)

// TestConsumers_NoAdHocFontSizeLiterals enforces DESIGN.md §2: display steps
// are tokens, not literals. Every font-size in a consumer stylesheet must
// route through a --text-* token; a raw px value means a heading or stat has
// opted out of the modular scale and would silently miss a future scale change.
func TestConsumers_NoAdHocFontSizeLiterals(t *testing.T) {
	for _, sheet := range shellStylesheets {
		css := fetchAsset(t, sheet)
		for _, match := range fontSizeLiteralRe.FindAllString(css, -1) {
			t.Errorf("%s has ad-hoc font-size literal %q — type sizes belong to the --text-* scale in tokens.css", sheet, match)
		}
	}
}

// coloredTextRe matches a color declaration that sets one of the status/role
// hues (as opposed to the neutral --text/--text-dim pair).
var coloredTextRe = regexp.MustCompile(`color:\s*var\(--(warn|danger|ok|resolved|unresolved|brand)\b`)

// TestConsumers_NoColoredTextAt2xs enforces the small-text legibility rule
// from DESIGN.md §1–2: --text-2xs (11px) is dense metadata only — never
// colored. The prior amber-at-11px failure came from exactly this pairing, so
// any rule block that sets both var(--text-2xs) and a status/role text color
// is a regression. Colored labels live at --text-xs (12px) or above.
func TestConsumers_NoColoredTextAt2xs(t *testing.T) {
	for _, sheet := range shellStylesheets {
		css := fetchAsset(t, sheet)
		for _, block := range strings.Split(css, "}") {
			if strings.Contains(block, "var(--text-2xs)") && coloredTextRe.MatchString(block) {
				t.Errorf("%s pairs --text-2xs with a colored text role in rule %q — colored small text must be >=12px", sheet, strings.TrimSpace(block))
			}
		}
	}
}

// TestTokens_WarnLegibleOnDark pins the other half of the amber fix: --warn is
// the lighter amber shade (DESIGN.md §1, "small colored text ... uses a
// lighter shade on dark"), not the darker default that failed AA in context.
func TestTokens_WarnLegibleOnDark(t *testing.T) {
	css := fetchAsset(t, "/css/tokens.css")
	if !containsHexFold(css, "#fbbf24") {
		t.Error("tokens.css --warn is not the lighter on-dark amber #fbbf24")
	}
	if regexp.MustCompile(`--warn:\s*#f59e0b`).MatchString(strings.ToLower(css)) {
		t.Error("tokens.css --warn still the darker #f59e0b amber")
	}
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
		// Retired by the design-system migration (issue #37): the cyan role is
		// --resolved now. (--on-accent deliberately survives — see tokens.css.)
		"var(--accent)",
		"var(--accent-hover)",
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
