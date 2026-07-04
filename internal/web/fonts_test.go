// Package web_test — fonts_test.go covers issue #22: vendored typography.
// Instrument Sans (display) and JetBrains Mono are self-hosted as woff2 in the
// embedded assets — no external CDN, no build step — and wired into the type
// token variables from issue #20. Like restyle_test.go, these tests drive the
// REAL embedded assets through web.Handler(): the served bytes are the public
// interface of a static design system.
package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// vendoredFonts are the two self-hosted faces issue #22 vendors: the display
// face for identity moments and the mono face for code-shaped content.
var vendoredFonts = []struct {
	label string
	path  string
}{
	{"Instrument Sans (display)", "/vendor/fonts/instrument-sans-latin-wght-normal.woff2"},
	{"JetBrains Mono", "/vendor/fonts/jetbrains-mono-latin-wght-normal.woff2"},
}

// TestFonts_VendoredWoff2Served is the tracer bullet: both vendored faces must
// be served from the embedded assets as genuine WOFF2 — correct content type
// and the wOF2 magic number — so the dashboard's typography works fully
// offline, with no CDN dependency.
func TestFonts_VendoredWoff2Served(t *testing.T) {
	ts := newTestServer(t)
	for _, font := range vendoredFonts {
		resp := get(t, ts, font.path)
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Errorf("GET %s (%s): status = %d, want 200", font.path, font.label, resp.StatusCode)
			continue
		}
		if ct := resp.Header.Get("Content-Type"); ct != "font/woff2" {
			t.Errorf("GET %s: Content-Type = %q, want \"font/woff2\"", font.path, ct)
		}
		if len(body) < 4 || string(body[:4]) != "wOF2" {
			t.Errorf("GET %s: body does not start with the wOF2 magic number — not a real WOFF2 file", font.path)
		}
	}
}

// TestFonts_FontFaceLocalOnly asserts the @font-face layer declares both
// vendored families and sources them exclusively from the embedded assets:
// every src points at /vendor/fonts/, and nothing in the font layer or the
// page head reaches out to an external host. "No CDN" is an acceptance
// criterion, not a preference — the dashboard must work fully offline.
func TestFonts_FontFaceLocalOnly(t *testing.T) {
	css := fetchAsset(t, "/css/fonts.css")

	for _, family := range []string{`"Instrument Sans"`, `"JetBrains Mono"`} {
		if !strings.Contains(css, "font-family: "+family) {
			t.Errorf("fonts.css missing @font-face family %s", family)
		}
	}
	if got := strings.Count(css, "@font-face"); got != 2 {
		t.Errorf("fonts.css has %d @font-face blocks, want 2", got)
	}
	for _, src := range fontSrcURLs(css) {
		if !strings.HasPrefix(src, "/vendor/fonts/") {
			t.Errorf("fonts.css src url %q is not under /vendor/fonts/ — fonts must be self-hosted", src)
		}
	}
	if strings.Contains(css, "http://") || strings.Contains(css, "https://") {
		t.Error("fonts.css references an external URL — fonts must load with no CDN")
	}

	// The page must actually load the font layer, and its head must not smuggle
	// a font CDN back in (preconnect/stylesheet to Google Fonts or similar).
	ts := newTestServer(t)
	html := string(readBody(t, get(t, ts, "/")))
	if !strings.Contains(html, `href="/css/fonts.css"`) {
		t.Error("index.html does not link /css/fonts.css")
	}
	for _, cdn := range []string{"fonts.googleapis.com", "fonts.gstatic.com", "cdn.jsdelivr.net", "unpkg.com"} {
		if strings.Contains(html, cdn) {
			t.Errorf("index.html references external host %q — no CDN allowed", cdn)
		}
	}
}

// TestFonts_TypeTokensWireVendoredFaces asserts the token layer routes type
// through the vendored faces: a --display token leading with Instrument Sans,
// --mono leading with JetBrains Mono, and --sans still the fast system stack
// (body/UI text deliberately does NOT pay the webfont cost).
func TestFonts_TypeTokensWireVendoredFaces(t *testing.T) {
	css := fetchAsset(t, "/css/tokens.css")

	wantLead := map[string]string{
		"--display:": `"Instrument Sans"`,
		"--mono:":    `"JetBrains Mono"`,
		"--sans:":    "-apple-system",
	}
	for token, lead := range wantLead {
		decl := tokenDecl(css, token)
		if decl == "" {
			t.Errorf("tokens.css missing type token %q", token)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(decl), lead) {
			t.Errorf("tokens.css %s = %q, want it to lead with %s", token, strings.TrimSpace(decl), lead)
		}
	}
}

// headingDisplayRe matches a CSS rule whose selector covers h1 (the wordmark
// element) and whose body routes font-family through the --display token.
var headingDisplayRe = regexp.MustCompile(`(?s)h1[^{]*\{[^}]*font-family:\s*var\(--display\)`)

// TestFonts_HeadingsUseDisplayFace asserts the display face is actually
// applied where issue #22 says it belongs: the wordmark (the .brand <h1>) and
// headings route through var(--display), while the body keeps var(--sans).
// Tokens that exist but are never consumed would satisfy the token test and
// still ship a system-font wordmark.
func TestFonts_HeadingsUseDisplayFace(t *testing.T) {
	base := fetchAsset(t, "/css/base.css")

	if !headingDisplayRe.MatchString(base) {
		t.Error("base.css has no heading rule applying font-family: var(--display) — wordmark/headings not in the display face")
	}
	if !strings.Contains(base, "font-family: var(--sans)") {
		t.Error("base.css no longer sets body text to var(--sans) — body/UI must stay on the system stack")
	}
}

// tokenDecl returns the value of a custom-property declaration in css (the
// text between "--name:" and the terminating semicolon), or "" if absent.
func tokenDecl(css, token string) string {
	i := strings.Index(css, token)
	if i < 0 {
		return ""
	}
	rest := css[i+len(token):]
	j := strings.Index(rest, ";")
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// fontSrcRe captures the url(...) target of each @font-face src declaration.
var fontSrcRe = regexp.MustCompile(`url\(["']?([^"')]+)["']?\)`)

// fontSrcURLs returns every url(...) referenced in the stylesheet.
func fontSrcURLs(css string) []string {
	var urls []string
	for _, m := range fontSrcRe.FindAllStringSubmatch(css, -1) {
		urls = append(urls, m[1])
	}
	return urls
}
