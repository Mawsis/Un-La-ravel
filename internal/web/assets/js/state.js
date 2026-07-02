// Holds the current analysis result in memory so every view reads from one
// place rather than each view re-fetching (design.md: "the dashboard fetches
// /api/analyze once per project and renders every view from that one
// in-memory object").

let current = null; // { model, mermaid, openapi } | null
const listeners = new Set();

export function setResult(result) {
  current = result;
  listeners.forEach((fn) => fn(current));
}

export function getResult() {
  return current;
}

export function onResultChange(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
