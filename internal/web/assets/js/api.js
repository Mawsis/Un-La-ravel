// Fetch wrapper over the server's JSON API (internal/web/api.go). The
// dashboard only ever calls /api/analyze — /api/er and /api/openapi exist for
// callers that want a single artifact without the full model.

export async function analyze(path) {
  const res = await fetch("/api/analyze?path=" + encodeURIComponent(path));
  const body = await res.json();
  if (!res.ok) {
    throw new Error(body && body.error ? body.error : "Analysis failed (HTTP " + res.status + ")");
  }
  return body; // { model, er, openapi }
}
