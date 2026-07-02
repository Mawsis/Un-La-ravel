// Un(la)ravel dashboard bootstrap.
//
// Drives the UI in index.html against the local server's JSON API:
//   GET /api/analyze?path=<local-path>  ->  { model, mermaid, openapi }
// where `model` is the unlaravel.json contract, `mermaid` is the ER diagram
// source, and `openapi` is the OpenAPI 3 document. Errors come back as
// { error } with a 4xx.
//
// No framework, no build step (design.md architecture constraint) — this
// file and internal/web/assets/js/* are plain ES modules loaded via
// <script type="module">, embedded in the Go binary via //go:embed.
//
// View switching here is a simple class toggle keyed by the sidebar's
// data-view attribute, NOT a hash router — that lands in a later PR
// (design.md "URL & state"). Sidebar links are still real <a> elements so
// they remain keyboard/AT-navigable; only the destination (a class toggle
// rather than a route) is provisional.

import { $, $$, escapeHtml } from "./dom.js";
import { analyze } from "./api.js";
import { setResult, onResultChange } from "./state.js";
import { renderOverview } from "./views/overview.js";
import { renderER } from "./views/er.js";
import { renderModels } from "./views/models.js";
import { renderRoutes } from "./views/routes.js";
import { renderFindings } from "./views/findings.js";
import { renderSwagger } from "./views/swagger.js";

const el = {
  form: $("#search-form"),
  input: $("#path-input"),
  btn: $("#analyze-btn"),
  intro: $("#intro"),
  status: $("#status"),
  results: $("#results"),
};

async function runAnalysis(path) {
  setLoading(true);
  announce("Analyzing " + path + "…");
  try {
    const result = await analyze(path);
    setResult(result);
    el.intro.classList.add("hidden");
    el.results.classList.remove("hidden");
    const routeCount = ((result.model || {}).routes || []).length;
    announce("Analyzed " + path + " — " + routeCount + " route(s) found.");
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

// announce writes to the aria-live status region (design.md a11y baseline:
// "One aria-live=\"polite\" status region announces... replacing today's
// silent spinner swap").
function announce(message) {
  $("#live-status").textContent = message;
}

onResultChange((result) => {
  if (!result) return;
  const model = result.model || {};
  renderOverview(model);
  renderER(result.mermaid);
  renderModels(model.models || [], model.schemas || []);
  renderRoutes(model.routes || [], model.dead_routes || []);
  renderFindings(model.disagreements || [], model.dead_routes || []);
  renderSwagger(result.openapi);
});

// ---- sidebar navigation (provisional — see file header) -------------------

function activateView(name) {
  $$(".nav-views a").forEach((a) => {
    const active = a.dataset.view === name;
    a.toggleAttribute("aria-current", active);
    if (active) a.setAttribute("aria-current", "page");
  });
  $$(".panel").forEach((p) => p.classList.toggle("active", p.dataset.panel === name));

  const heading = document.querySelector('.panel.active h2');
  if (heading) {
    heading.setAttribute("tabindex", "-1");
    heading.focus();
  }
  const label = document.querySelector('.nav-views a[data-view="' + name + '"]');
  document.title = (label ? label.textContent.trim() : "Un(la)ravel") + " — Un(la)ravel";
}

// Only intercept a plain left-click. Middle-click, Ctrl/Cmd-click, and
// Shift-click are the browser's built-in "open in new tab/window" gestures —
// these are real <a href="#/view"> elements specifically so that keeps
// working; preventDefault() unconditionally would silently swallow it.
$$(".nav-views a").forEach((a) =>
  a.addEventListener("click", (e) => {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    activateView(a.dataset.view);
  })
);

// ---- wire up ----------------------------------------------------------------

el.form.addEventListener("submit", (e) => {
  e.preventDefault();
  const path = el.input.value.trim();
  if (path) runAnalysis(path);
});
