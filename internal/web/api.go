package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/render/er"
	"github.com/Mawsis/Un-La-ravel/internal/render/openapi"
)

// pathParam is the query-string key naming the local Laravel project to analyze,
// e.g. /api/analyze?path=/abs/path/to/app.
const pathParam = "path"

// contentTypeJSON is the media type every API response is served with.
const contentTypeJSON = "application/json; charset=utf-8"

// errMissingPath is returned when the request omits the ?path= query parameter.
// It is a sentinel so handlers can map it to a 400 uniformly.
var errMissingPath = errors.New("missing required query parameter 'path'")

// analyzeResponse is the body of GET /api/analyze — the one endpoint the
// dashboard needs. It bundles the three artifacts a caller would otherwise
// assemble from three round-trips:
//
//   - Model is the raw Project Model: the exact unlaravel.json contract the CLI
//     emits (schema_version, project_name, schemas, models, routes, ...). It is
//     embedded as json.RawMessage so it is serialized once, by the model's own
//     ToJSON, and passed through verbatim — the web response can never drift
//     from the CLI's JSON.
//   - ER is er.RenderGraph(model): the structured ER graph (nodes + edges) the
//     browser SVG renderer draws — the tables and relationships as data, laid
//     out client-side by ELK (issue #26). This replaced the former Mermaid
//     erDiagram string, which the browser no longer consumes.
//   - OpenAPI is openapi.Render(model): the OpenAPI 3 document, embedded raw so
//     Swagger UI can consume it directly.
type analyzeResponse struct {
	Model   json.RawMessage `json:"model"`
	ER      er.ERGraph      `json:"er"`
	OpenAPI json.RawMessage `json:"openapi"`
}

// erResponse is the body of GET /api/er: the structured ER graph the browser
// SVG renderer draws, so a UI that only wants the diagram need not receive the
// whole model.
type erResponse struct {
	ER er.ERGraph `json:"er"`
}

// errorResponse is the uniform error envelope every failing API request
// returns: { "error": "<message>" } with a 4xx/5xx status.
type errorResponse struct {
	Error string `json:"error"`
}

// handleAnalyze serves GET /api/analyze?path=<local-path>. It runs the analysis
// engine once and returns the model, its structured ER graph, and its OpenAPI 3
// document in a single { model, er, openapi } response — the primary endpoint
// the dashboard consumes. A missing path or a non-Laravel target is a 400 with
// a JSON error; a renderer failure on an otherwise-valid model is a 500 (it is
// a server-side defect, not bad input).
func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	pm, err := analyzeFromRequest(r)
	if err != nil {
		writeAnalyzeError(w, err)
		return
	}

	modelJSON, err := pm.ToJSON()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to serialize project model: %w", err))
		return
	}

	openAPIJSON, err := openapi.Render(pm)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to render OpenAPI document: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, analyzeResponse{
		Model:   modelJSON,
		ER:      er.RenderGraph(pm),
		OpenAPI: openAPIJSON,
	})
}

// handleER serves GET /api/er?path=<local-path>, returning only the structured
// ER graph in an { er } envelope — for a UI that renders the diagram without
// needing the full model.
func handleER(w http.ResponseWriter, r *http.Request) {
	pm, err := analyzeFromRequest(r)
	if err != nil {
		writeAnalyzeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, erResponse{ER: er.RenderGraph(pm)})
}

// bootstrapResponse is the body of GET /api/bootstrap: a versionless
// convenience payload, deliberately NOT part of the unlaravel.json contract,
// telling the dashboard which project (if any) `unlaravel serve [path]` was
// started with so it can auto-analyze on load instead of the developer
// re-typing a path they already gave on the command line.
type bootstrapResponse struct {
	DefaultPath string `json:"default_path"`
}

// handleBootstrap returns a GET /api/bootstrap handler closed over the
// server's configured default project path (empty when `serve` was started
// with no path argument, in which case the response's default_path is "").
func handleBootstrap(defaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, bootstrapResponse{DefaultPath: defaultPath})
	}
}

// handleOpenAPI serves GET /api/openapi?path=<local-path>, returning the
// OpenAPI 3 document itself (not wrapped) so Swagger UI can be pointed straight
// at this URL.
func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	pm, err := analyzeFromRequest(r)
	if err != nil {
		writeAnalyzeError(w, err)
		return
	}

	openAPIJSON, err := openapi.Render(pm)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to render OpenAPI document: %w", err))
		return
	}
	writeRawJSON(w, http.StatusOK, openAPIJSON)
}

// analyzeFromRequest reads and validates the ?path= parameter, then runs
// engine.Analyze against it. It is the shared front half of every API handler:
// on success the caller has the Project Model; on failure it returns the error
// for writeAnalyzeError to classify (missing path vs. failed analysis) into the
// right HTTP status.
func analyzeFromRequest(r *http.Request) (*model.ProjectModel, error) {
	path := r.URL.Query().Get(pathParam)
	if path == "" {
		return nil, errMissingPath
	}

	pm, err := engine.Analyze(path)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze %q: %w", path, err)
	}
	return pm, nil
}

// writeAnalyzeError maps a front-half failure to an HTTP status: a missing path
// or a failed analysis (not a Laravel project, unreadable path) is a 400,
// because both are bad client input rather than a server fault. The full error
// message is returned so the developer sees exactly why analysis failed.
func writeAnalyzeError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadRequest, err)
}

// writeJSON serializes v as the response body with the given status and the JSON
// content type. A serialization failure is logged and downgraded to a 500 with a
// generic body, since the header may already be committed.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to encode response: %w", err))
		return
	}
	writeRawJSON(w, status, body)
}

// writeRawJSON writes an already-serialized JSON body with the given status and
// the JSON content type. It is used for pass-through payloads (the model's own
// JSON, the OpenAPI document) that must not be re-encoded.
func writeRawJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeError writes the uniform { "error": "..." } envelope with the given
// status. It marshals a tiny fixed struct, so encoding cannot itself fail.
func writeError(w http.ResponseWriter, status int, err error) {
	body, _ := json.Marshal(errorResponse{Error: err.Error()})
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
