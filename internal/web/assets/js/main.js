// Un(la)ravel dashboard bootstrap.
//
// Drives the UI in index.html against the local server's JSON API:
//   GET /api/analyze?path=<local-path>  ->  { model, er, openapi }
//   GET /api/bootstrap                  ->  { default_path }
// where `model` is the unlaravel.json contract, `er` is the structured ER
// graph (nodes + edges) the SVG renderer draws, and `openapi` is the OpenAPI 3
// document. Errors come back as { error } with a 4xx.
//
// No framework, no build step (design.md architecture constraint) — this
// file and internal/web/assets/js/* are plain ES modules loaded via
// <script type="module">, embedded in the Go binary via //go:embed.
//
// State model (design.md "URL & state"): the hash (router.js) is the source
// of truth for which view is active and its filter/sort/focus params — it's
// what makes refresh, Back/Forward, and copy-paste-share all just work.
// localStorage (store.js) is a convenience-only recent-projects list; nothing
// here depends on it for correctness.
//
// Boot order: an explicit ?path= in the hash (a deep link — explicit intent,
// auto-analyzed) > the project `unlaravel serve [path]` was started with
// (GET /api/bootstrap) > an empty entry screen with the recent-projects list
// offered, not auto-run.

import { $, $$, escapeHtml } from "./dom.js";
import { analyze } from "./api.js";
import { setResult, getResult } from "./state.js";
import { addRecent } from "./store.js";
import * as router from "./router.js";
import { renderOverview } from "./views/overview.js";
import { renderER, focusTable } from "./views/er.js";
import { renderModels } from "./views/models.js";
import { renderRoutes } from "./views/routes.js";
import { renderFindings } from "./views/findings.js";
import { renderAuth } from "./views/auth.js";
import { renderSidebarHealth } from "./sidebar.js";
import { resettleThreadMark } from "./thread-mark.js";
import { prepareOverviewWow, maybePlayOverviewWow } from "./views/overview-wow.js";
import { renderSwagger } from "./views/swagger.js";
import { renderRecents } from "./views/recents.js";
import { samplePathFrom } from "./views/hero.js";
import { initSearch } from "./search.js";

const el = {
  form: $("#search-form"),
  input: $("#path-input"),
  btn: $("#analyze-btn"),
  hero: $("#hero"),
  sampleBtn: $("#sample-btn"),
  intro: $("#intro"),
  status: $("#status"),
  results: $("#results"),
  content: $("#main-content"),
};

let currentPath = null; // the project path the last successful analysis ran against

async function runAnalysis(path, { fromRouter = false } = {}) {
  setLoading(true);
  announce("Analyzing " + path + "…");
  try {
    const result = await analyze(path);
    currentPath = path;
    setResult(result);
    addRecent(path, (result.model || {}).project_name);
    // Leave first-run: the hero and its extras go away, and the path form
    // shrinks from hero primary action to the compact header bar (a CSS
    // concern keyed off .first-run). An analysis *failure* deliberately
    // keeps the hero: the user is still on the entry screen.
    el.hero.classList.add("hidden");
    el.intro.classList.add("hidden");
    el.content.classList.remove("first-run");
    el.results.classList.remove("hidden");
    const routeCount = ((result.model || {}).routes || []).length;
    announce("Analyzed " + path + " — " + routeCount + " route(s) found.");
    if (!fromRouter) {
      // A fresh analysis from the form is new navigation intent — reflect it
      // in the URL so refresh/Back/share work from here on. A hash-driven
      // analysis (deep link, already reflects the URL) skips this to avoid
      // pushing a redundant duplicate history entry.
      const { view, params } = router.getCurrent();
      const next = new URLSearchParams(params);
      next.set("path", path);
      router.navigate(view, next);
    }
  } catch (err) {
    const message = err && err.message ? err.message : String(err);
    showError(message);
    announce("Analysis failed: " + message);
  } finally {
    setLoading(false);
  }
}

function setLoading(on) {
  el.btn.disabled = on;
  el.status.innerHTML = on ? '<div class="loading"><span class="spinner"></span>Analyzing…</div>' : "";
}

function showError(msg) {
  el.results.classList.add("hidden");
  el.status.innerHTML = '<div class="error">' + escapeHtml(msg) + "</div>";
}

// announce writes to the aria-live status region (design.md a11y baseline).
function announce(message) {
  $("#live-status").textContent = message;
}

// ---- rendering from router state -------------------------------------------

// renderedForResult tracks which analysis result the once-per-analysis
// views (everything except the active view's filter/sort-driven content)
// were last rendered for, so navigating between views doesn't re-render or
// re-mount them on every hash change. Comparing by reference is enough since
// state.js always replaces (never mutates) the result object.
let renderedForResult = null;

// lastFocusedTable tracks the ?table= value ER was last focused with, so a
// navigation that changes the focus target (e.g. clicking a different
// table's link while already on the ER view) calls focusTable again even
// though the analysis result itself hasn't changed and ER's one-render-per-
// analysis panel below is skipped.
let lastFocusedTable = null;

function renderCurrentView() {
  const result = getResult();
  if (!result) return;
  const { view, params } = router.getCurrent();
  const model = result.model || {};
  const table = params.get("table");

  // Render once per analysis, not once per navigation — these panels don't
  // depend on router params (other than ER's initial focus, handled below),
  // and re-rendering ER on every view switch was destroying/recreating its
  // svg-pan-zoom instance while hidden (display:none), which svg-pan-zoom
  // can't handle (degenerate transform matrix on a zero-size container).
  if (renderedForResult !== result) {
    renderedForResult = result;
    lastFocusedTable = null;
    renderOverview(model);
    renderSidebarHealth(model);
    resettleThreadMark(); // signature gesture: once per analysis, never per navigation
    // The wow moment is also once per analysis; if Overview isn't the active
    // view right now it holds until the first visit (see overview-wow.js).
    prepareOverviewWow(model);
    renderER(result.er, table);
    lastFocusedTable = table;
    renderFindings(model.disagreements || [], model.dead_routes || [], params);
    renderAuth(model.routes || [], params);
    renderSwagger(result.openapi);
  } else if (view === "er" && table && table !== lastFocusedTable) {
    // Same analysis, same diagram already mounted — just refocus.
    focusTable(table);
    lastFocusedTable = table;
  }

  // Filter/sort-driven content is cheap to redraw and its correctness
  // depends on the current URL params, so it re-renders on every navigation
  // regardless of which view is active — the badge counts it also updates
  // (e.g. "Routes 11") must stay correct even when Models is the active view.
  renderModels(model.models || [], model.schemas || [], params.get("filter") || "", (filter) => {
    const next = new URLSearchParams(params);
    filter ? next.set("filter", filter) : next.delete("filter");
    router.navigate(view, next, { replace: true });
  }, params);
  renderRoutes(model.routes || [], model.dead_routes || [], routesStateFromParams(params), (newState) => {
    router.navigate(view, paramsFromRoutesState(params, newState), { replace: true });
  }, params);

  activateView(view);
}

function routesStateFromParams(params) {
  return {
    filter: params.get("filter") || "",
    sortKey: params.get("sort") || null,
    sortDir: params.get("dir") === "desc" ? -1 : 1,
  };
}

function paramsFromRoutesState(params, state) {
  const next = new URLSearchParams(params);
  state.filter ? next.set("filter", state.filter) : next.delete("filter");
  state.sortKey ? next.set("sort", state.sortKey) : next.delete("sort");
  if (state.sortKey && state.sortDir === -1) next.set("dir", "desc");
  else next.delete("dir");
  return next;
}

// ---- sidebar navigation (router-backed) ------------------------------------

function activateView(name) {
  $$(".nav-views a").forEach((a) => {
    const active = a.dataset.view === name;
    a.toggleAttribute("aria-current", active);
    if (active) a.setAttribute("aria-current", "page");
  });
  $$(".panel").forEach((p) => p.classList.toggle("active", p.dataset.panel === name));

  if (name === "overview") maybePlayOverviewWow(); // release a held take

  const heading = document.querySelector(".panel.active h2");
  if (heading) {
    heading.setAttribute("tabindex", "-1");
    heading.focus();
  }
  const label = document.querySelector('.nav-views a[data-view="' + name + '"]');
  document.title = (label ? label.textContent.trim() : "Un(la)ravel") + " — Un(la)ravel";
}

// Any static link that switches views WITHOUT going through hrefFor()
// (currently: the sidebar, plus the ER hint's "see Models" link) has a
// static href with no path baked in — data-view marks it as one of these,
// so its path gets injected from router state at click time. Cross-
// navigation links built via hrefFor() (links.js) are NOT marked
// data-view: they already bake in the current path when rendered, so a
// plain native hash click works for them without interception.
//
// Only intercept a plain left-click. Middle-click, Ctrl/Cmd-click, and
// Shift-click are the browser's built-in "open in new tab/window" gestures —
// these are real <a href="#/view"> elements specifically so that keeps
// working; preventDefault() unconditionally would silently swallow it.
function isPlainLeftClick(e) {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}

$$("a[data-view]").forEach((a) =>
  a.addEventListener("click", (e) => {
    if (!isPlainLeftClick(e)) return;
    e.preventDefault();
    const { params } = router.getCurrent();
    // Carry the current path forward across a view switch, drop any
    // view-specific filter/sort/focus params — those belong to the view
    // being left, not the one being entered.
    const next = new URLSearchParams();
    if (params.get("path")) next.set("path", params.get("path"));
    router.navigate(a.dataset.view, next);
  })
);

// ---- router subscription ----------------------------------------------------

router.subscribe(({ params }) => {
  const path = params.get("path");
  if (!path) {
    renderCurrentView(); // no path in the URL — nothing to (re)analyze
    return;
  }
  if (path === currentPath && getResult()) {
    renderCurrentView(); // same project already analyzed — just re-render the view
    return;
  }
  el.input.value = path;
  runAnalysis(path, { fromRouter: true }).then(renderCurrentView);
});

// ---- entity-chip cross-navigation (issue #24) -----------------------------
// Chips (rendered by entityChip in chip.js) are created dynamically inside
// panels on every analysis, so they can't be wired up individually the way the
// static sidebar links are. One delegated listener on the document handles all
// of them: a plain left-click on any .entity-chip[data-view] switches to that
// entity's view, while the same modifier gestures as the sidebar keep native
// open-in-new-tab working.
//
// Unlike the sidebar links, a chip navigates via the router so the switch is a
// real, shareable URL change consistent with the rest of the app (#16/#17): it
// carries the current ?path= forward and drops the view-specific params of the
// view being left. Focusing the specific entity within the view (er.js's
// focusTable, models/routes filter) lands in a later PR — this delivers
// reference → chip → click → the entity's proper view.
document.addEventListener("click", (e) => {
  const chip = e.target.closest && e.target.closest("a.entity-chip[data-view]");
  if (!chip || !isPlainLeftClick(e)) return;
  e.preventDefault();
  const { params } = router.getCurrent();
  const next = new URLSearchParams();
  if (params.get("path")) next.set("path", params.get("path"));
  router.navigate(chip.dataset.view, next);
});

// ---- wire up ----------------------------------------------------------------

// The form and recents-click handlers below deliberately do NOT check
// path === currentPath before calling runAnalysis, unlike the router
// subscription above. That asymmetry is intentional: a hash change carrying
// the same path is usually just a view switch that happens to repeat the
// param (skip re-fetching is correct), whereas the user submitting the form
// or re-clicking a recent project is an explicit "analyze this again" — the
// engine is static with no caching (CLAUDE.md ADR 0007), so a deliberate
// re-run should re-read the project's current files on disk, not reuse a
// stale in-memory result.
el.form.addEventListener("submit", (e) => {
  e.preventDefault();
  const path = el.input.value.trim();
  if (path) runAnalysis(path).then(renderCurrentView);
});

renderRecents((path) => {
  el.input.value = path;
  runAnalysis(path).then(renderCurrentView);
});

initSearch(
  () => (getResult() || {}).model || null,
  () => router.getCurrent().params
);

// ---- boot sequence ------------------------------------------------------

async function boot() {
  router.init();
  const { params } = router.getCurrent();

  // 1. An explicit ?path= in the hash is a deep link — explicit intent,
  //    auto-analyze it. router.subscribe (above) already fires from
  //    router.init(), so if a path is present this is already in flight.
  if (params.get("path")) return;

  // 2. Otherwise, ask the server what `unlaravel serve [path]` was started
  //    with, if anything.
  try {
    const res = await fetch("/api/bootstrap");
    const body = await res.json();
    if (res.ok && body && body.default_path) {
      el.input.value = body.default_path;
      const next = new URLSearchParams(router.getCurrent().params);
      next.set("path", body.default_path);
      router.navigate(router.getCurrent().view, next, { replace: true });
      return;
    }
    // No serve-arg default: if the server has the bundled sample project
    // (issue #29), reveal the hero's "try it on the sample project" button.
    // A click is an explicit "analyze this" — same contract as the form.
    const samplePath = samplePathFrom(res.ok ? body : null);
    if (samplePath) {
      el.sampleBtn.hidden = false;
      el.sampleBtn.addEventListener("click", () => {
        el.input.value = samplePath;
        runAnalysis(samplePath).then(renderCurrentView);
      });
    }
  } catch (e) {
    // /api/bootstrap unreachable — fall through to the empty entry screen.
  }

  // 3. No hash path, no serve-arg default: empty entry screen. The recent
  //    projects list (already rendered above) is offered, not auto-run.
}

boot();
