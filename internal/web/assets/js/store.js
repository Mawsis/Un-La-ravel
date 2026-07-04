// localStorage recent-projects list (design.md "URL & state": "localStorage
// carries cross-session convenience only... correctness never depends on
// localStorage being present"). Every function here degrades to a no-op if
// localStorage throws (private browsing, quota, disabled) — a dashboard that
// can't remember recent projects should still work, it just won't remember.

const STORAGE_KEY = "unlaravel.recentProjects";
const MAX_RECENTS = 8;

// addRecent records path as most-recently-used, deduping by exact path match
// and capping the list at MAX_RECENTS. projectName is best-effort display
// text (falls back to the path itself when the model has no project_name).
export function addRecent(path, projectName) {
  if (!path) return;
  try {
    const list = getRecents().filter((r) => r.path !== path);
    list.unshift({ path, projectName: projectName || path, lastUsed: nowIso() });
    localStorage.setItem(STORAGE_KEY, JSON.stringify(list.slice(0, MAX_RECENTS)));
  } catch (e) {
    // localStorage unavailable — recents are a convenience, not a requirement.
  }
}

export function getRecents() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch (e) {
    return [];
  }
}

export function removeRecent(path) {
  try {
    const list = getRecents().filter((r) => r.path !== path);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
  } catch (e) {
    // no-op — see addRecent.
  }
}

// nowIso avoids Date.now()/new Date() directly at call sites so every
// timestamp in this module goes through one place; this is a plain browser
// module with no test-determinism constraint on Date, unlike the Go side.
function nowIso() {
  return new Date().toISOString();
}
