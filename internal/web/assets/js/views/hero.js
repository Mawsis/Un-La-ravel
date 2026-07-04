// First-run hero helpers (issue #29). The hero itself is static markup in
// index.html; what needs logic is the "try it on the sample project" button,
// which must only appear when the server actually has the bundled fixture to
// offer (GET /api/bootstrap's sample_path).

// samplePathFrom answers "does this bootstrap response offer a sample
// project, and where?" from the /api/bootstrap body. Returns the path, or
// null when there is none to offer (empty, missing, malformed, or an older
// server without the field) — the caller keeps the button hidden on null.
export function samplePathFrom(bootstrapBody) {
  const path = bootstrapBody && bootstrapBody.sample_path;
  return typeof path === "string" && path !== "" ? path : null;
}
