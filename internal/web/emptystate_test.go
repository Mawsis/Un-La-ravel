// Package web_test — emptystate_test.go covers issue #29: the logo-led
// empty state and the "try it on the sample project" button.
//
// The server half of that button lives here: GET /api/bootstrap (the
// versionless convenience endpoint, deliberately NOT part of the
// unlaravel.json contract) advertises where the bundled sample project
// (testdata/fixture-app) lives on disk, so the dashboard can offer a
// one-click "see it working" analysis — and can hide the button entirely
// when the fixture isn't present (an installed binary run outside the
// repo), rather than showing a button that errors.
package web_test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/web"
)

// sampleBootstrapBody is the slice of the /api/bootstrap response these tests
// care about (server_test.go declares its own struct for default_path).
type sampleBootstrapBody struct {
	SamplePath string `json:"sample_path"`
}

// TestBootstrap_SamplePath_PointsAtBundledFixture asserts the bootstrap
// response advertises the bundled fixture app: a non-empty path that ends in
// testdata/fixture-app and actually exists as a directory, so the dashboard's
// sample button analyzes something real.
func TestBootstrap_SamplePath_PointsAtBundledFixture(t *testing.T) {
	ts := newTestServer(t)
	resp := get(t, ts, "/api/bootstrap")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /api/bootstrap: status = %d, want 200", resp.StatusCode)
	}

	var bb sampleBootstrapBody
	if err := json.Unmarshal(readBody(t, resp), &bb); err != nil {
		t.Fatalf("decode bootstrap body: %v", err)
	}

	if bb.SamplePath == "" {
		t.Fatal("bootstrap sample_path is empty; want the bundled testdata/fixture-app")
	}
	want := filepath.Join("testdata", "fixture-app")
	if !filepath.IsAbs(bb.SamplePath) || filepath.Base(filepath.Dir(bb.SamplePath))+string(filepath.Separator)+filepath.Base(bb.SamplePath) != want {
		t.Errorf("sample_path = %q, want an absolute path ending in %q", bb.SamplePath, want)
	}
	info, err := os.Stat(bb.SamplePath)
	if err != nil || !info.IsDir() {
		t.Errorf("sample_path %q does not exist as a directory: %v", bb.SamplePath, err)
	}
}

// TestBootstrap_SamplePath_EmptyWhenFixtureUnreachable asserts the graceful
// degradation half of the contract: a server started somewhere with no
// testdata/fixture-app above it (an installed binary outside the repo)
// reports sample_path "" — the dashboard hides the button — rather than
// advertising a path that would 400 on analysis.
func TestBootstrap_SamplePath_EmptyWhenFixtureUnreachable(t *testing.T) {
	// The fixture is resolved relative to the working directory when the
	// handler is built, so chdir to an empty temp dir first. t.Chdir restores
	// the original working directory when the test ends.
	t.Chdir(t.TempDir())

	ts := httptest.NewServer(web.Handler())
	t.Cleanup(ts.Close)
	resp := get(t, ts, "/api/bootstrap")

	var bb sampleBootstrapBody
	if err := json.Unmarshal(readBody(t, resp), &bb); err != nil {
		t.Fatalf("decode bootstrap body: %v", err)
	}
	if bb.SamplePath != "" {
		t.Errorf("sample_path = %q, want \"\" when no fixture is reachable from the working directory", bb.SamplePath)
	}
}

// TestEmptyState_LogoLedHero asserts the first-run markup issue #29
// prescribes, as properties of the shipped index.html (the served bytes ARE
// the public interface here, as in restyle_test.go):
//
//   - a hero section leading with the un-raveling-Laravel mark as an inline
//     SVG (vector, token-colorable — not a raster background image);
//   - the wordmark and the one-line static-analysis value proposition;
//   - the path form as the primary action directly below the hero;
//   - the "try it on the sample project" button, shipped hidden (JS reveals
//     it only when /api/bootstrap advertises a sample_path);
//   - the recent-projects container below all of the above.
func TestEmptyState_LogoLedHero(t *testing.T) {
	ts := newTestServer(t)
	html := string(readBody(t, get(t, ts, "/")))

	markers := []struct{ label, marker string }{
		{"hero section", `id="hero"`},
		{"inline SVG logo", `class="hero-logo"`},
		{"wordmark", `class="hero-wordmark"`},
		{"value proposition", `class="hero-valueprop"`},
		{"value proposition copy", "No boot, no database, no"},
		{"path form (primary action)", `id="path-input"`},
		{"sample button", `id="sample-btn"`},
		{"recents container", `id="recents"`},
	}

	// Each marker must exist, and in this order: logo-led hero first, then
	// the path input as the primary action, then the sample button, then
	// recents ("Recent projects list appears below the hero").
	last := -1
	for _, m := range markers {
		i := strings.Index(html, m.marker)
		if i < 0 {
			t.Errorf("index.html missing %s (%q)", m.label, m.marker)
			continue
		}
		if i < last {
			t.Errorf("index.html: %s (%q) appears before the preceding hero element — hero order broken", m.label, m.marker)
		}
		last = i
	}

	// The logo must be an inline <svg> (crisp at any size, colorable via the
	// token layer), not an <img> pointing at one of the raster concepts.
	if !regexp.MustCompile(`<svg[^>]*class="hero-logo"`).MatchString(html) {
		t.Error(`hero logo is not an inline <svg class="hero-logo">`)
	}

	// The sample button ships hidden: it only becomes visible when the server
	// actually has a sample to offer.
	if !regexp.MustCompile(`<button[^>]*id="sample-btn"[^>]*\bhidden\b`).MatchString(html) {
		t.Error(`sample button must ship with the hidden attribute (JS reveals it when bootstrap advertises sample_path)`)
	}
}

// TestEmptyState_HeroStyledFromTokens asserts the hero's presentation
// criteria from issue #29: the identity moments (wordmark, value prop) are
// set in the display face from the token layer, the hero/sample-button/
// first-run styles exist at all, and — "no decorative background art beyond
// the logo" — no shell stylesheet paints any background image. (Raw-hex and
// old-token leaks are already covered globally by restyle_test.go.)
func TestEmptyState_HeroStyledFromTokens(t *testing.T) {
	css := fetchAsset(t, "/css/components.css")

	for _, sel := range []string{".hero", ".hero-logo", ".hero-wordmark", ".hero-valueprop", ".sample-btn"} {
		if !strings.Contains(css, sel) {
			t.Errorf("components.css missing %s rules — hero unstyled", sel)
		}
	}
	if !strings.Contains(css, "var(--display)") {
		t.Error("components.css: hero identity moments must use the display face via var(--display)")
	}

	// The path form must render as the hero's primary action on first run:
	// some stylesheet has to key off the .first-run state main.js maintains.
	if layout := fetchAsset(t, "/css/layout.css"); !strings.Contains(layout, ".first-run") {
		t.Error("layout.css missing .first-run rules — the path form never becomes the hero's primary action")
	}

	// No decorative background art: the logo (inline SVG in the HTML) is the
	// only imagery the empty state carries.
	for _, sheet := range shellStylesheets {
		if strings.Contains(fetchAsset(t, sheet), "background-image") {
			t.Errorf("%s declares a background-image — decorative background art is out (issue #29)", sheet)
		}
	}
}
