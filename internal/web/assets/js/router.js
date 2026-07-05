// Hash router (design.md "URL & state"): #/<view>?path=<abs-path>&filter=
// <text>&sort=<field>&table=<name>. The URL carries everything needed to
// reproduce a view — which project, which view, which filter/sort/focus —
// so refresh, Back/Forward, and copy-paste-share all just work. localStorage
// (store.js) is convenience only and never load-bearing for correctness.
//
// Views are the seven sidebar destinations: overview, er, models, routes, api,
// findings, auth (the auth-coverage view, issue #50).

const VALID_VIEWS = new Set(["overview", "er", "models", "routes", "api", "findings", "auth"]);
const DEFAULT_VIEW = "overview";

const listeners = new Set();
let current = { view: DEFAULT_VIEW, params: new URLSearchParams() };
// applying guards against a hashchange firing re-entrantly from inside
// navigate() and double-notifying subscribers. history.pushState/
// replaceState (what navigate() actually uses below) do NOT fire
// hashchange per spec, so this guard is inert today — but it's cheap
// insurance against a future edit to navigate() that assigns
// location.hash directly (which DOES fire hashchange), so it stays.
let applying = false;

// parse reads the current location.hash into { view, params }. An unknown or
// missing view falls back to DEFAULT_VIEW rather than erroring, so a stale or
// hand-edited hash never leaves the page unnavigable.
export function parse() {
  const hash = location.hash.replace(/^#\/?/, ""); // strip "#" and a leading "/"
  const qIdx = hash.indexOf("?");
  const viewPart = qIdx === -1 ? hash : hash.slice(0, qIdx);
  const queryPart = qIdx === -1 ? "" : hash.slice(qIdx + 1);
  const view = VALID_VIEWS.has(viewPart) ? viewPart : DEFAULT_VIEW;
  return { view, params: new URLSearchParams(queryPart) };
}

// navigate sets the hash to reflect { view, params }. By default it pushes a
// new history entry (so Back steps through views/focuses meaningfully);
// pass { replace: true } for high-frequency updates like filter keystrokes,
// which should not spam history.
export function navigate(view, params, { replace = false } = {}) {
  const search = new URLSearchParams(params);
  const qs = search.toString();
  const hash = "#/" + view + (qs ? "?" + qs : "");
  applying = true;
  if (replace) {
    history.replaceState(null, "", hash);
  } else {
    history.pushState(null, "", hash);
  }
  applying = false;
  current = { view, params: search };
  notify();
}

// current returns the last-parsed { view, params } without re-reading the
// hash, so callers that just called navigate() see their own update
// immediately rather than racing the hashchange event.
export function getCurrent() {
  return current;
}

export function subscribe(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

function notify() {
  listeners.forEach((fn) => fn(current));
}

window.addEventListener("hashchange", () => {
  if (applying) return; // navigate() already updated `current` and notified
  current = parse();
  notify();
});

// init reads the hash present at page load (a deep link or a refresh) and
// notifies subscribers once, without pushing a duplicate history entry.
export function init() {
  current = parse();
  notify();
}
