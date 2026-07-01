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
	"net"
	"net/http"
	"strconv"
	"time"
)

// assetsFS embeds the dashboard's static files (index.html, app.js, ...). The
// UI agent fills internal/web/assets; embedding the whole directory means the
// binary self-contains the dashboard with no runtime file dependency. The glob
// matches the directory's files; the package builds as long as assets/ is
// non-empty, which the placeholder index.html and app.js guarantee.
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

// NewServer builds a dashboard Server bound to 127.0.0.1 on the given port
// (pass DefaultPort for the standard 4448). It wires the routes but does not
// start listening — call Start for that. A non-positive port is rejected so a
// caller cannot accidentally bind to an OS-chosen port and print the wrong URL.
func NewServer(port int) (*Server, error) {
	if port <= 0 {
		return nil, fmt.Errorf("invalid port %d: must be positive", port)
	}

	addr := net.JoinHostPort(localhost, strconv.Itoa(port))
	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           Handler(),
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

// Handler builds the dashboard's HTTP handler: the embedded single-page UI at
// "/" plus the JSON API under "/api/". It is exported so tests (and any
// embedding caller) can exercise the routes with httptest.NewServer without
// binding a real socket.
//
// Routes:
//
//	GET /              -> the embedded dashboard (assets/index.html and friends)
//	GET /api/analyze   -> { model, mermaid, openapi } for ?path=<local-path>
//	GET /api/er        -> { mermaid } for ?path=<local-path>
//	GET /api/openapi   -> the OpenAPI 3 document for ?path=<local-path>
func Handler() http.Handler {
	mux := http.NewServeMux()

	// JSON API. These are registered before the "/" catch-all; ServeMux's
	// longest-prefix match still routes "/api/..." here regardless of order, but
	// listing them first keeps the routing table readable.
	mux.HandleFunc("/api/analyze", handleAnalyze)
	mux.HandleFunc("/api/er", handleER)
	mux.HandleFunc("/api/openapi", handleOpenAPI)

	// Static single-page dashboard, served from the embedded assets subtree so
	// "/" resolves to assets/index.html.
	mux.Handle("/", staticHandler())

	return logRequests(mux)
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
