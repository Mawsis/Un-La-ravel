// Package web serves the Un(la)ravel interactive dashboard and its JSON API.
//
// It is JUST ANOTHER CONSUMER of the analysis engine (ADR 0001/0004: one
// Project Model, many renderers). The server never reimplements extraction — it
// calls engine.Analyze and hands the SAME *model.ProjectModel to the SAME
// renderers the CLI uses (er, openapi), so the JSON it returns is the identical
// unlaravel.json contract. Nothing here parses PHP or knows Laravel's layout;
// that all lives behind engine.Analyze.
//
// The server binds to localhost only (127.0.0.1): this is a LOCAL developer
// tool, not a public service, so the project path is a trusted local input and
// path traversal is not a security boundary. Errors are still surfaced cleanly
// as JSON.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// assetsFS embeds the dashboard's static files (index.html, css/, js/,
// vendor/, ...) recursively. Embedding the whole directory means the binary
// self-contains the dashboard with no runtime file dependency.
//
//go:embed assets
var assetsFS embed.FS

// assetsDir is the embed subtree the static file server is rooted at, so a
// request for "/" maps to assets/index.html rather than assets/assets/....
const assetsDir = "assets"

// DefaultPort is the port `unlaravel serve` listens on unless overridden. 4448
// spells "GITH"-ish on a phone keypad and stays clear of the common dev ports
// (3000/5173/8080), reducing the chance of a clash on a developer's machine.
const DefaultPort = 4448

// localhost is the loopback host the server binds to. Binding here rather than
// 0.0.0.0 keeps the dashboard reachable only from the developer's own machine —
// it is not meant to be exposed on a network.
const localhost = "127.0.0.1"

// readHeaderTimeout bounds how long a client may take to send its request
// headers, closing the Slowloris hole that an unbounded net/http server leaves
// open. It is generous because this server only ever talks to a local browser.
const readHeaderTimeout = 10 * time.Second

// Server is the dashboard HTTP server: a configured net/http server plus the
// address it will listen on. Construct it with NewServer.
type Server struct {
	httpServer *http.Server
	addr       string
}

// Option configures a Server at construction time (the standard Go
// functional-options pattern). See WithDefaultProject.
type Option func(*serverConfig)

// serverConfig collects the options NewServer applies before building the
// handler, so adding a future option never changes NewServer's signature.
type serverConfig struct {
	defaultProject string
}

// WithDefaultProject pre-fills the dashboard's bootstrap response
// (GET /api/bootstrap) with a project path, so `unlaravel serve [path]` can
// have the browser auto-analyze that path on load instead of the developer
// re-typing it. Passing "" (the zero value, and NewServer's default when this
// option is omitted) means no default — the dashboard starts at its normal
// empty entry screen.
func WithDefaultProject(path string) Option {
	return func(c *serverConfig) { c.defaultProject = path }
}

// NewServer builds a dashboard Server bound to 127.0.0.1 on the given port
// (pass DefaultPort for the standard 4448). It wires the routes but does not
// start listening — call Start for that. A non-positive port is rejected so a
// caller cannot accidentally bind to an OS-chosen port and print the wrong URL.
func NewServer(port int, opts ...Option) (*Server, error) {
	if port <= 0 {
		return nil, fmt.Errorf("invalid port %d: must be positive", port)
	}

	addr := net.JoinHostPort(localhost, strconv.Itoa(port))
	return &Server{
		httpServer: &http.Server{
			Addr: addr,
			// Delegates to HandlerWithOptions rather than re-applying opts
			// here, so option-construction logic (currently a trivial
			// apply-loop, but not guaranteed to stay that simple) has
			// exactly one implementation.
			Handler:           HandlerWithOptions(opts...),
			ReadHeaderTimeout: readHeaderTimeout,
		},
		addr: addr,
	}, nil
}

// Addr reports the "host:port" the server listens on, e.g. "127.0.0.1:4448".
func (s *Server) Addr() string { return s.addr }

// Start begins serving and blocks until the server stops. A clean shutdown
// (http.ErrServerClosed) is reported as nil so callers do not treat an
// intentional stop as a failure.
func (s *Server) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("dashboard server on %s failed: %w", s.addr, err)
	}
	return nil
}

// Handler builds the dashboard's HTTP handler with no default project
// (equivalent to `unlaravel serve` with no path argument). It is exported so
// tests (and any embedding caller) can exercise the routes with
// httptest.NewServer without binding a real socket. See handler for the
// full route table and HandlerWithOptions to configure a default project.
func Handler() http.Handler {
	return handler(&serverConfig{})
}

// HandlerWithOptions builds the dashboard's HTTP handler with the given
// Options applied — the same configuration NewServer accepts, exposed
// separately so tests can exercise GET /api/bootstrap against an
// httptest.NewServer without binding a real socket via NewServer/Start.
func HandlerWithOptions(opts ...Option) http.Handler {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return handler(cfg)
}

// handler builds the dashboard's HTTP handler: the embedded single-page UI at
// "/" plus the JSON API under "/api/".
//
// Routes:
//
//	GET /              -> the embedded dashboard (assets/index.html and friends)
//	GET /api/analyze   -> { model, er, openapi } for ?path=<local-path>
//	GET /api/er        -> { er } for ?path=<local-path>
//	GET /api/openapi   -> the OpenAPI 3 document for ?path=<local-path>
//	GET /api/bootstrap -> { default_path } — a versionless convenience
//	                      endpoint, NOT part of the unlaravel.json contract,
//	                      that lets the dashboard auto-analyze the project
//	                      `unlaravel serve [path]` was started with.
func handler(cfg *serverConfig) http.Handler {
	mux := http.NewServeMux()

	// JSON API. These are registered before the "/" catch-all; ServeMux's
	// longest-prefix match still routes "/api/..." here regardless of order, but
	// listing them first keeps the routing table readable.
	mux.HandleFunc("/api/analyze", handleAnalyze)
	mux.HandleFunc("/api/er", handleER)
	mux.HandleFunc("/api/openapi", handleOpenAPI)
	mux.HandleFunc("/api/bootstrap", handleBootstrap(cfg.defaultProject, sampleProjectPath()))

	// Static single-page dashboard, served from the embedded assets subtree so
	// "/" resolves to assets/index.html.
	mux.Handle("/", staticHandler())

	return logRequests(mux)
}

func init() {
	// Go's builtin mime table has no entry for .woff2, so http.FileServer would
	// fall back to whatever the host OS's mime.types says (or sniff to
	// application/octet-stream on slim containers). The vendored fonts (issue
	// #22) must serve as font/woff2 everywhere, so pin it explicitly.
	if err := mime.AddExtensionType(".woff2", "font/woff2"); err != nil {
		panic(fmt.Sprintf("web: registering .woff2 mime type: %v", err))
	}
}

// sampleProjectPath locates the bundled sample Laravel project
// (testdata/fixture-app) by walking up from the working directory, so it is
// found whether `unlaravel serve` runs from the repo root or a subdirectory.
// A directory only counts if it holds a composer.json — the marker that it is
// an analyzable project and not an unrelated dir that happens to share the
// name. Returns "" when no fixture is reachable (e.g. an installed binary run
// outside the repo), which the dashboard reads as "no sample available: hide
// the try-it button" (issue #29).
func sampleProjectPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, "testdata", "fixture-app")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(candidate, "composer.json")); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// staticHandler serves the embedded dashboard assets. It roots a sub-filesystem
// at the assets directory so URL "/" maps to index.html and "/app.js" to
// app.js, without leaking the "assets/" prefix into the URL space.
func staticHandler() http.Handler {
	sub, err := fs.Sub(assetsFS, assetsDir)
	if err != nil {
		// Unreachable in practice: assetsDir is a compile-time constant embedded
		// above, so fs.Sub cannot fail at runtime. Panicking here surfaces a
		// build-time mistake (a renamed/removed assets dir) loudly rather than
		// serving a broken UI.
		panic(fmt.Sprintf("web: embedded assets subtree %q missing: %v", assetsDir, err))
	}
	return http.FileServer(http.FS(sub))
}

// logRequests wraps a handler to log one concise line per request:
// method, path, and how long it took. It is deliberately simple — a local dev
// tool needs a request trace, not a structured access log.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
	})
}
