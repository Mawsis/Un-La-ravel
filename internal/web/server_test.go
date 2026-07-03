// Package web_test exercises the HTTP handler exported by internal/web.
//
// It uses httptest.NewServer(web.Handler()) so every test drives the REAL
// router, middleware (logRequests), and JSON serialization paths, without
// opening a real network socket or calling cobra / CLI code.
//
// Absolute paths to testdata are constructed from the source-file location via
// runtime.Caller(0) so the tests work regardless of which directory `go test`
// is invoked from.
package web_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/web"
)

// repoRoot walks up from this file's directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod above %s", dir)
		}
		dir = parent
	}
}

// newTestServer starts an httptest.Server backed by web.Handler() and returns
// it. The caller must call ts.Close() (registered via t.Cleanup for
// convenience).
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(web.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// get is a tiny helper that issues a GET against the test server and returns
// the response, failing the test on transport errors.
func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// readBody reads and closes the response body, failing the test on error.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

// ---------------------------------------------------------------------------
// GET / — static dashboard
// ---------------------------------------------------------------------------

// TestHandler_Root_ServesHTML asserts that GET / returns 200, a Content-Type
// containing "text/html", and a body that contains the dashboard's title
// marker so we know the real embedded index.html is being served.
func TestHandler_Root_ServesHTML(t *testing.T) {
	ts := newTestServer(t)
	resp := get(t, ts, "/")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status = %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want it to contain text/html", ct)
	}

	body := readBody(t, resp)
	// "Un(la)ravel" appears in the <title> and in the brand element of index.html.
	if !strings.Contains(string(body), "Un(la)ravel") {
		t.Errorf("response body does not contain 'Un(la)ravel'; body = %q", body[:min(200, len(body))])
	}
	// "ER Diagram" is the label of the sidebar's ER Diagram nav link.
	if !strings.Contains(string(body), "ER Diagram") {
		t.Errorf("response body does not contain 'ER Diagram'")
	}
}

// TestHandler_Assets_Served asserts that every asset index.html references
// (the entry module and the extracted stylesheets) resolves to 200, so the
// browser never hits a 404 for a <script>/<link> the shell depends on. This
// also guards the css/js directory split itself: //go:embed embeds
// subdirectories automatically, but a typo'd path in index.html would only
// surface as a silent broken page without this test.
func TestHandler_Assets_Served(t *testing.T) {
	ts := newTestServer(t)

	assets := []string{
		"/js/main.js",
		"/js/dom.js",
		"/js/api.js",
		"/js/state.js",
		"/js/views/overview.js",
		"/js/views/er.js",
		"/js/views/models.js",
		"/js/views/routes.js",
		"/js/views/findings.js",
		"/js/views/swagger.js",
		"/css/tokens.css",
		"/css/base.css",
		"/css/layout.css",
		"/css/components.css",
		"/vendor/mermaid.min.js",
		"/vendor/svg-pan-zoom.min.js",
		"/vendor/swagger-ui-bundle.js",
		"/vendor/swagger-ui.css",
	}

	for _, path := range assets {
		t.Run(path, func(t *testing.T) {
			resp := get(t, ts, path)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body := readBody(t, resp)
				t.Fatalf("GET %s: status = %d, want 200; body = %s", path, resp.StatusCode, body)
			}
		})
	}
}

// TestHandler_OldAppJS_Gone asserts that /app.js — the pre-restructure
// single-file bundle, replaced by the js/ ES module tree — is no longer
// served. A regression here would mean the old file crept back into
// internal/web/assets and is shipping as dead weight in the embedded binary.
func TestHandler_OldAppJS_Gone(t *testing.T) {
	ts := newTestServer(t)
	resp := get(t, ts, "/app.js")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /app.js: status = %d, want 404 (the old single-file bundle should no longer be embedded)", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GET /api/analyze — happy path
// ---------------------------------------------------------------------------

// analyzeResponse mirrors the server's internal analyzeResponse type so we can
// unmarshal the JSON without importing the (unexported) struct.
type analyzeResponse struct {
	Model   json.RawMessage `json:"model"`
	Mermaid string          `json:"mermaid"`
	OpenAPI json.RawMessage `json:"openapi"`
}

// projectModelShape is the minimal projection of the embedded model field we
// need to assert the contract without importing the full model package.
type projectModelShape struct {
	SchemaVersion string `json:"schema_version"`
	ProjectName   string `json:"project_name"`
	Schemas       []struct {
		Name string `json:"name"`
	} `json:"schemas"`
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
	Routes []struct {
		Method string `json:"method"`
		URI    string `json:"uri"`
	} `json:"routes"`
	FormRequests []struct {
		Name string `json:"name"`
	} `json:"form_requests"`
	DeadRoutes []struct {
		Kind string `json:"kind"`
	} `json:"dead_routes"`
	Disagreements []struct {
		Kind string `json:"kind"`
	} `json:"disagreements"`
}

// TestHandler_Analyze_FixtureApp asserts the primary happy-path contract for
// GET /api/analyze?path=<fixture-app>:
//   - 200 status
//   - Content-Type is JSON
//   - Body is a valid JSON object with "model", "mermaid", "openapi" keys
//   - "model" embeds a schema_version and the expected node arrays
//   - "mermaid" is non-empty
//   - "openapi" is valid JSON
func TestHandler_Analyze_FixtureApp(t *testing.T) {
	ts := newTestServer(t)

	fixtureApp := filepath.Join(repoRoot(t), "testdata", "fixture-app")
	target := ts.URL + "/api/analyze?path=" + url.QueryEscape(fixtureApp)

	resp, err := http.Get(target)
	if err != nil {
		t.Fatalf("GET /api/analyze: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	body := readBody(t, resp)
	var ar analyzeResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody=%s", err, body)
	}

	// "model" must be a non-null JSON object.
	if len(ar.Model) == 0 || string(ar.Model) == "null" {
		t.Fatal("model field is null or empty")
	}

	// Unmarshal the embedded model to assert schema_version and node counts.
	var shape projectModelShape
	if err := json.Unmarshal(ar.Model, &shape); err != nil {
		t.Fatalf("model field is not valid JSON: %v", err)
	}
	if shape.SchemaVersion == "" {
		t.Error("model.schema_version is empty")
	}
	if got, want := len(shape.Schemas), 3; got != want {
		t.Errorf("model.schemas count = %d, want %d", got, want)
	}
	if got, want := len(shape.Models), 3; got != want {
		t.Errorf("model.models count = %d, want %d", got, want)
	}
	if got, want := len(shape.Routes), 11; got != want {
		t.Errorf("model.routes count = %d, want %d", got, want)
	}
	if got, want := len(shape.FormRequests), 1; got != want {
		t.Errorf("model.form_requests count = %d, want %d", got, want)
	}
	if got, want := len(shape.DeadRoutes), 1; got != want {
		t.Errorf("model.dead_routes count = %d, want %d", got, want)
	}
	if got, want := len(shape.Disagreements), 1; got != want {
		t.Errorf("model.disagreements count = %d, want %d", got, want)
	}

	// "mermaid" must be non-empty (the ER diagram source).
	if ar.Mermaid == "" {
		t.Error("mermaid field is empty")
	}

	// "openapi" must be a valid JSON object.
	if len(ar.OpenAPI) == 0 || string(ar.OpenAPI) == "null" {
		t.Fatal("openapi field is null or empty")
	}
	var opDoc map[string]json.RawMessage
	if err := json.Unmarshal(ar.OpenAPI, &opDoc); err != nil {
		t.Fatalf("openapi field is not valid JSON: %v", err)
	}
	if _, ok := opDoc["openapi"]; !ok {
		t.Error("openapi document is missing the 'openapi' version key")
	}
}

// ---------------------------------------------------------------------------
// GET /api/analyze — error paths
// ---------------------------------------------------------------------------

// errorBody is the JSON envelope the API returns for all 4xx/5xx responses.
type errorBody struct {
	Error string `json:"error"`
}

// TestHandler_Analyze_MissingPath asserts that GET /api/analyze without a
// ?path= parameter returns 400 with a JSON { "error": "..." } body.
func TestHandler_Analyze_MissingPath(t *testing.T) {
	ts := newTestServer(t)
	resp := get(t, ts, "/api/analyze")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, body)
	}

	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil {
		t.Fatalf("error body is not valid JSON: %v\nbody=%s", err, body)
	}
	if eb.Error == "" {
		t.Error("error body has empty 'error' field")
	}
}

// TestHandler_Analyze_NonLaravelPath asserts that GET /api/analyze?path=<dir>
// where the directory exists but is not a Laravel project returns 400 with a
// JSON { "error": "..." } body.
func TestHandler_Analyze_NonLaravelPath(t *testing.T) {
	ts := newTestServer(t)

	notLaravel := t.TempDir() // real dir, definitely not a Laravel project
	target := ts.URL + "/api/analyze?path=" + url.QueryEscape(notLaravel)

	resp, err := http.Get(target)
	if err != nil {
		t.Fatalf("GET /api/analyze: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, body)
	}

	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil {
		t.Fatalf("error body is not valid JSON: %v\nbody=%s", err, body)
	}
	if eb.Error == "" {
		t.Error("error body has empty 'error' field")
	}
}

// TestHandler_Analyze_NonExistentPath asserts that GET /api/analyze?path=...
// with a path that does not exist on disk returns 400 (bad input, not server
// fault).
func TestHandler_Analyze_NonExistentPath(t *testing.T) {
	ts := newTestServer(t)

	ghost := filepath.Join(t.TempDir(), "does-not-exist")
	target := ts.URL + "/api/analyze?path=" + url.QueryEscape(ghost)

	resp, err := http.Get(target)
	if err != nil {
		t.Fatalf("GET /api/analyze: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, body)
	}

	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil {
		t.Fatalf("error body is not valid JSON: %v\nbody=%s", err, body)
	}
	if eb.Error == "" {
		t.Error("error body has empty 'error' field")
	}
}

// ---------------------------------------------------------------------------
// GET /api/er
// ---------------------------------------------------------------------------

// TestHandler_ER_FixtureApp asserts that GET /api/er?path=<fixture-app>
// returns 200 and a JSON body with a non-empty "mermaid" field.
func TestHandler_ER_FixtureApp(t *testing.T) {
	ts := newTestServer(t)

	fixtureApp := filepath.Join(repoRoot(t), "testdata", "fixture-app")
	target := ts.URL + "/api/er?path=" + url.QueryEscape(fixtureApp)

	resp, err := http.Get(target)
	if err != nil {
		t.Fatalf("GET /api/er: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}

	var er struct {
		Mermaid string `json:"mermaid"`
	}
	if err := json.Unmarshal(body, &er); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody=%s", err, body)
	}
	if er.Mermaid == "" {
		t.Error("mermaid field is empty")
	}
}

// TestHandler_ER_MissingPath asserts that GET /api/er without ?path= returns 400.
func TestHandler_ER_MissingPath(t *testing.T) {
	ts := newTestServer(t)
	resp := get(t, ts, "/api/er")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, body)
	}
	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil {
		t.Fatalf("error body is not valid JSON: %v\nbody=%s", err, body)
	}
	if eb.Error == "" {
		t.Error("error body has empty 'error' field")
	}
}

// ---------------------------------------------------------------------------
// GET /api/openapi
// ---------------------------------------------------------------------------

// TestHandler_OpenAPI_FixtureApp asserts that GET /api/openapi?path=<fixture-app>
// returns 200 and a valid OpenAPI 3 JSON document (with an "openapi" version
// key at the root).
func TestHandler_OpenAPI_FixtureApp(t *testing.T) {
	ts := newTestServer(t)

	fixtureApp := filepath.Join(repoRoot(t), "testdata", "fixture-app")
	target := ts.URL + "/api/openapi?path=" + url.QueryEscape(fixtureApp)

	resp, err := http.Get(target)
	if err != nil {
		t.Fatalf("GET /api/openapi: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody=%s", err, body)
	}
	if _, ok := doc["openapi"]; !ok {
		t.Error("OpenAPI document is missing the 'openapi' version key")
	}
}

// TestHandler_OpenAPI_MissingPath asserts that GET /api/openapi without ?path=
// returns 400.
func TestHandler_OpenAPI_MissingPath(t *testing.T) {
	ts := newTestServer(t)
	resp := get(t, ts, "/api/openapi")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, body)
	}
	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil {
		t.Fatalf("error body is not valid JSON: %v\nbody=%s", err, body)
	}
	if eb.Error == "" {
		t.Error("error body has empty 'error' field")
	}
}

// ---------------------------------------------------------------------------
// GET /api/bootstrap
// ---------------------------------------------------------------------------

type bootstrapBody struct {
	DefaultPath string `json:"default_path"`
}

// TestHandler_Bootstrap_NoDefaultProject asserts that GET /api/bootstrap
// returns an empty default_path when the server was built with no
// WithDefaultProject option — the `unlaravel serve` (no path argument) case.
func TestHandler_Bootstrap_NoDefaultProject(t *testing.T) {
	ts := newTestServer(t)
	resp := get(t, ts, "/api/bootstrap")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	var bb bootstrapBody
	if err := json.Unmarshal(body, &bb); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody=%s", err, body)
	}
	if bb.DefaultPath != "" {
		t.Errorf("default_path = %q, want empty (no WithDefaultProject was set)", bb.DefaultPath)
	}
}

// TestHandler_Bootstrap_WithDefaultProject asserts that GET /api/bootstrap
// echoes back the path passed to WithDefaultProject — the
// `unlaravel serve [path]` case, letting the dashboard auto-analyze without
// the developer re-typing the path they already gave on the command line.
func TestHandler_Bootstrap_WithDefaultProject(t *testing.T) {
	fixtureApp := filepath.Join(repoRoot(t), "testdata", "fixture-app")
	ts := httptest.NewServer(web.HandlerWithOptions(web.WithDefaultProject(fixtureApp)))
	t.Cleanup(ts.Close)

	resp := get(t, ts, "/api/bootstrap")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	var bb bootstrapBody
	if err := json.Unmarshal(body, &bb); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody=%s", err, body)
	}
	if bb.DefaultPath != fixtureApp {
		t.Errorf("default_path = %q, want %q", bb.DefaultPath, fixtureApp)
	}
}

// TestHandler_Bootstrap_ConcurrentRequests verifies no data race or
// cross-request leakage when many goroutines hit /api/bootstrap concurrently
// against a server configured with WithDefaultProject — the closure captures
// an immutable string once at handler-construction time, so this is mostly a
// belt-and-suspenders check against a future refactor introducing shared
// mutable state, run under `go test -race`.
func TestHandler_Bootstrap_ConcurrentRequests(t *testing.T) {
	ts := httptest.NewServer(web.HandlerWithOptions(web.WithDefaultProject("/fixed/path")))
	t.Cleanup(ts.Close)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := get(t, ts, "/api/bootstrap")
			body := readBody(t, resp)

			var bb bootstrapBody
			if err := json.Unmarshal(body, &bb); err != nil {
				t.Errorf("response is not valid JSON: %v\nbody=%s", err, body)
				return
			}
			if bb.DefaultPath != "/fixed/path" {
				t.Errorf("default_path = %q, want /fixed/path", bb.DefaultPath)
			}
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// NewServer
// ---------------------------------------------------------------------------

// TestNewServer_ValidPort verifies that NewServer returns a non-nil Server and
// that Addr() returns the host:port string, without actually listening.
func TestNewServer_ValidPort(t *testing.T) {
	s, err := web.NewServer(web.DefaultPort)
	if err != nil {
		t.Fatalf("NewServer(%d): %v", web.DefaultPort, err)
	}
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	addr := s.Addr()
	if addr == "" {
		t.Error("Addr() returned empty string")
	}
	if !strings.Contains(addr, "127.0.0.1") {
		t.Errorf("Addr() = %q, expected loopback (127.0.0.1)", addr)
	}
}

// TestNewServer_InvalidPort verifies that NewServer rejects a non-positive port
// with an error, protecting callers from accidentally binding to port 0.
func TestNewServer_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s, err := web.NewServer(tc.port)
			if err == nil {
				t.Errorf("NewServer(%d): expected error, got nil (server=%+v)", tc.port, s)
			}
			if s != nil {
				t.Errorf("NewServer(%d): expected nil server on error, got %+v", tc.port, s)
			}
		})
	}
}

// min is a local helper for Go < 1.21 which lacks the built-in min.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
