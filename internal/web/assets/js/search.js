// Command palette (Cmd/Ctrl+K), design.md "Search": indexes the current
// Project Model client-side (routes, models, tables, findings), substring
// match, deterministic ordering (model order, then kind). Openable via the
// sidebar's visible Search button OR the keyboard shortcut — never
// shortcut-only (design.md a11y baseline).
//
// Accessibility: role="dialog" aria-modal="true" with a focus trap; Esc
// restores focus to whatever opened the dialog; the input behaves as a
// combobox (aria-expanded/aria-controls/aria-autocomplete) over a
// role="listbox" results list with aria-selected marking the active item.

import { $, escapeHtml } from "./dom.js";
import { hrefFor } from "./links.js";

let index = []; // [{ kind, label, sublabel, href }]
let results = [];
let activeIndex = -1;
let invoker = null; // element to refocus when the dialog closes

const el = {
  openBtn: $("#search-open-btn"),
  overlay: $("#search-overlay"),
  dialog: $("#search-dialog"),
  input: $("#search-input"),
  results: $("#search-results"),
};

// initSearch wires up the palette. buildIndexFn(model) -> index entries;
// called lazily each time the dialog opens, so the index always reflects
// the latest analysis without the caller needing to push updates.
export function initSearch(getModel, currentParamsFn) {
  el.openBtn.addEventListener("click", () => open());
  document.addEventListener("keydown", (e) => {
    const isCmdK = (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k";
    if (isCmdK) {
      e.preventDefault();
      isOpen() ? close() : open();
    } else if (e.key === "Escape" && isOpen()) {
      close();
    }
  });
  el.overlay.addEventListener("click", (e) => {
    if (e.target === el.overlay) close();
  });
  el.input.addEventListener("input", () => {
    runQuery(getModel(), currentParamsFn());
  });
  el.input.addEventListener("keydown", (e) => onInputKeydown(e));
  el.dialog.addEventListener("keydown", (e) => trapTab(e));

  function open() {
    const model = getModel();
    if (!model) return; // nothing to search before an analysis has run
    invoker = document.activeElement;
    el.overlay.classList.remove("hidden");
    el.input.value = "";
    el.input.focus();
    runQuery(model, currentParamsFn());
  }

  function close() {
    el.overlay.classList.add("hidden");
    if (invoker && typeof invoker.focus === "function") invoker.focus();
    invoker = null;
  }

  function isOpen() {
    return !el.overlay.classList.contains("hidden");
  }

  return { open, close };
}

function onInputKeydown(e) {
  if (e.key === "ArrowDown") {
    e.preventDefault();
    setActive(Math.min(activeIndex + 1, results.length - 1));
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    setActive(Math.max(activeIndex - 1, 0));
  } else if (e.key === "Enter") {
    e.preventDefault();
    if (results[activeIndex]) navigateTo(results[activeIndex]);
  }
}

// trapTab keeps focus inside the dialog while it's open — the only two
// focusable elements are the input and (indirectly, via arrow keys, not
// Tab) the results, so trapping just means Tab never leaves the input.
function trapTab(e) {
  if (e.key !== "Tab") return;
  e.preventDefault();
  el.input.focus();
}

function buildIndex(model, currentParams) {
  const entries = [];

  (model.routes || []).forEach((r) => {
    entries.push({
      kind: "route",
      label: (r.method || "") + " " + (r.uri || ""),
      sublabel: (r.controller || "") + "@" + (r.action || ""),
      href: hrefFor("route", r.uri || "", currentParams),
    });
  });
  (model.models || []).forEach((m) => {
    entries.push({
      kind: "model",
      label: m.name || "",
      sublabel: m.table || "",
      href: hrefFor("model", m.name || "", currentParams),
    });
  });
  (model.schemas || []).forEach((t) => {
    entries.push({
      kind: "table",
      label: t.name || "",
      sublabel: (t.columns || []).length + " column(s)",
      href: hrefFor("table", t.name || "", currentParams),
    });
  });
  (model.dead_routes || []).forEach((d) => {
    entries.push({
      kind: "finding",
      label: "Dead route: " + (d.method || "") + " " + (d.uri || ""),
      sublabel: d.reason || "",
      href: hrefFor("findings", "", currentParams),
    });
  });
  (model.disagreements || []).forEach((d) => {
    entries.push({
      kind: "finding",
      label: "Disagreement: " + (d.model || "") + "::" + (d.relationship || ""),
      sublabel: d.reason || "",
      href: hrefFor("findings", "", currentParams),
    });
  });

  return entries;
}

function runQuery(model, currentParams) {
  index = buildIndex(model, currentParams);
  const q = el.input.value.trim().toLowerCase();
  results = q
    ? index.filter(
        (e) => e.label.toLowerCase().includes(q) || (e.sublabel || "").toLowerCase().includes(q)
      )
    : index;
  renderResults();
  setActive(results.length > 0 ? 0 : -1);
}

function renderResults() {
  if (results.length === 0) {
    el.results.innerHTML = '<li class="search-empty">No matches.</li>';
    return;
  }
  el.results.innerHTML = results
    .map(
      (r, i) =>
        '<li class="search-result" role="option" id="search-result-' + i + '" data-index="' + i + '">' +
        '<span class="kind">' + escapeHtml(r.kind) + '</span>' +
        '<span class="label">' + escapeHtml(r.label) + '</span>' +
        "</li>"
    )
    .join("");
  el.results.querySelectorAll(".search-result").forEach((li) => {
    li.addEventListener("click", () => {
      const i = Number(li.dataset.index);
      if (results[i]) navigateTo(results[i]);
    });
  });
}

function setActive(i) {
  activeIndex = i;
  el.results.querySelectorAll(".search-result").forEach((li, idx) => {
    li.setAttribute("aria-selected", String(idx === i));
  });
  el.input.setAttribute("aria-activedescendant", i >= 0 ? "search-result-" + i : "");
  const activeLi = el.results.querySelector('[data-index="' + i + '"]');
  if (activeLi) activeLi.scrollIntoView({ block: "nearest" });
}

// navigateTo pushes entry.href (already a full "#/view?..." string from
// hrefFor) via history.pushState + a manual hashchange dispatch, matching
// how router.js's own navigate() updates history — NOT a bare
// `location.hash = entry.href` assignment, which would double the leading
// "#" (entry.href already starts with one).
function navigateTo(entry) {
  history.pushState(null, "", entry.href);
  window.dispatchEvent(new HashChangeEvent("hashchange"));
  el.overlay.classList.add("hidden");
  if (invoker && typeof invoker.focus === "function") invoker.focus();
  invoker = null;
}
