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
let current = { view: DEFAULT_VIEW, detail: null, params: new URLSearchParams() };
// applying guards against a hashchange firing re-entrantly from inside
// navigate() and double-notifying subscribers. history.pushState/
// replaceState (what navigate() actually uses below) do NOT fire
// hashchange per spec, so this guard is inert today — but it's cheap
// insurance against a future edit to navigate() that assigns
// location.hash directly (which DOES fire hashchange), so it stays.
let applying = false;

// parse reads a location hash into { view, detail, params }. It defaults to the
// live location.hash so callers in the browser pass nothing; it accepts an
// explicit hash string so the grammar stays unit-testable without a real
// Location. An unknown or missing view falls back to DEFAULT_VIEW rather than
// erroring, so a stale or hand-edited hash never leaves the page unnavigable.
//
// The path part may be one or two segments: "<view>" or "<view>/<detail>". The
// detail segment addresses one entity within a view — today only the model
// detail page (#/models/{name}, issue #51). A detail on an unknown view is
// dropped along with the view (both fall back), so a bogus deep link degrades
// cleanly rather than half-applying.
export function parse(rawHash) {
  const hash = (rawHash != null ? rawHash : location.hash).replace(/^#\/?/, ""); // strip "#" and a leading "/"
  const qIdx = hash.indexOf("?");
  const pathPart = qIdx === -1 ? hash : hash.slice(0, qIdx);
  const queryPart = qIdx === -1 ? "" : hash.slice(qIdx + 1);
  const slash = pathPart.indexOf("/");
  const viewPart = slash === -1 ? pathPart : pathPart.slice(0, slash);
  const detailPart = slash === -1 ? "" : pathPart.slice(slash + 1);
  const known = VALID_VIEWS.has(viewPart);
  const view = known ? viewPart : DEFAULT_VIEW;
  const detail = known && detailPart ? safeDecode(detailPart) : null;
  return { view, detail, params: new URLSearchParams(queryPart) };
}

// safeDecode percent-decodes a path segment, tolerating a malformed sequence by
// returning it verbatim rather than throwing (decodeURIComponent throws on a
// lone '%'), so one bad character in a hand-edited URL never breaks navigation.
function safeDecode(segment) {
  try {
    return decodeURIComponent(segment);
  } catch (e) {
    return segment;
  }
}

// hashFor builds the canonical hash string for a { view, detail, params } tuple:
// "#/<view>", "#/<view>/<detail>", each optionally suffixed with "?<query>".
// The detail segment is percent-encoded so a name with reserved characters
// round-trips through parse() intact. Pure (no history/location) so the URL
// grammar is unit-testable and navigate() has one place that knows it.
export function hashFor(view, detail, params) {
  const qs = new URLSearchParams(params).toString();
  const path = detail ? view + "/" + encodeURIComponent(detail) : view;
  return "#/" + path + (qs ? "?" + qs : "");
}

// navigate sets the hash to reflect { view, params } (and an optional detail
// segment for entity-detail sub-routes like #/models/{name}, issue #51). By
// default it pushes a new history entry (so Back steps through views/focuses
// meaningfully); pass { replace: true } for high-frequency updates like filter
// keystrokes, which should not spam history.
export function navigate(view, params, { replace = false, detail = null } = {}) {
  const search = new URLSearchParams(params);
  const hash = hashFor(view, detail, search);
  applying = true;
  if (replace) {
    history.replaceState(null, "", hash);
  } else {
    history.pushState(null, "", hash);
  }
  applying = false;
  current = { view, detail, params: search };
  notify();
}

// current returns the last-parsed { view, detail, params } without re-reading
// the hash, so callers that just called navigate() see their own update
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

// The hashchange listener is a browser-only side effect; guard its registration
// so importing this module under Node (to unit-test the pure parse() grammar)
// doesn't throw on a missing `window`. In the browser this runs exactly as
// before.
if (typeof window !== "undefined") {
  window.addEventListener("hashchange", () => {
    if (applying) return; // navigate() already updated `current` and notified
    current = parse();
    notify();
  });
}

// init reads the hash present at page load (a deep link or a refresh) and
// notifies subscribers once, without pushing a duplicate history entry.
export function init() {
  current = parse();
  notify();
}
