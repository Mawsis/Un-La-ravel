// ER Diagram view: Mermaid rendering + svg-pan-zoom navigation, plus
// focusTable for cross-navigation from other views (design.md
// "Cross-navigation"). focusTable does NOT depend on Mermaid's internal
// id/uuid scheme (which is vendored-library-internal and could change on an
// upgrade) — it matches the RENDERED entity label text instead, which is a
// stable, visible fact about the diagram regardless of how Mermaid names its
// internal DOM ids. Pinned to the vendored mermaid 10.9.6 bundle's DOM shape:
// each entity renders as <g id="entity-...-<uuid>"><text class="er
// entityLabel">NAME</text>...<rect class="er entityBox">...</g>.

import { $, escapeHtml } from "../dom.js";

let panZoom = null;
let currentSvg = null; // the currently-rendered <svg>, so focusTable can run after renderER without a caller-managed handoff

if (window.mermaid) {
  mermaid.initialize({
    startOnLoad: false,
    theme: "dark",
    securityLevel: "loose",
    // useMaxWidth:false stops Mermaid shrinking a large diagram to the
    // container width (which made big schemas unreadable); the SVG renders
    // at natural size and svg-pan-zoom provides navigation instead.
    er: { useMaxWidth: false },
  });
}

// renderGeneration guards against two overlapping renderER calls: if a
// second call starts before the first's async mermaid.render() resolves,
// the first call's result must be discarded rather than mounted (which
// would otherwise silently overwrite panZoom/currentSvg with a stale
// instance the second call already replaced, leaking the first call's
// svgPanZoom listeners for the page's lifetime). Each call captures the
// generation it started at and only mounts if still current when its
// await resolves.
let renderGeneration = 0;

// renderER renders the diagram, then (if focusEntityName is given) focuses
// that entity once the SVG and pan-zoom instance are ready — deep-linking
// into "#/er?table=users" needs the focus to happen on the SAME render that
// creates the pan-zoom instance, not a separate call, since svg-pan-zoom
// isn't ready until this function's async work completes.
export async function renderER(mermaidSrc, focusEntityName) {
  const myGeneration = ++renderGeneration;
  const container = $("#er-diagram");

  // Tear down the previous pan-zoom instance before replacing the SVG,
  // otherwise its listeners leak across analyses.
  if (panZoom) {
    panZoom.destroy();
    panZoom = null;
  }
  currentSvg = null;

  if (!mermaidSrc || !window.mermaid) {
    if (myGeneration === renderGeneration) container.innerHTML = '<p class="hint">No schema to diagram.</p>';
    return;
  }
  try {
    const { svg } = await mermaid.render("er-graph-" + renderNonce(), mermaidSrc);

    // A newer renderER call started while this one was awaiting Mermaid —
    // that call already destroyed our panZoom precursor and will mount its
    // own; mounting this stale result would silently leak the winner's
    // work (or, if this resolves last, overwrite it with older data).
    if (myGeneration !== renderGeneration) return;

    container.innerHTML = svg;
    const svgEl = container.querySelector("svg");
    if (svgEl && window.svgPanZoom) {
      // Fill the fixed-height container; pan-zoom drives the viewport.
      svgEl.style.width = "100%";
      svgEl.style.height = "100%";
      svgEl.style.maxWidth = "none";
      panZoom = svgPanZoom(svgEl, {
        zoomEnabled: true,
        panEnabled: true,
        mouseWheelZoomEnabled: true,
        dblClickZoomEnabled: true,
        controlIconsEnabled: true,
        fit: true,
        center: true,
        minZoom: 0.1,
        maxZoom: 50,
        zoomScaleSensitivity: 0.35,
      });
      currentSvg = svgEl;
      if (focusEntityName) focusTable(focusEntityName);
    }
  } catch (e) {
    if (myGeneration !== renderGeneration) return;
    // Fall back to showing the source if Mermaid can't parse it.
    container.innerHTML =
      '<p class="hint">Could not render the diagram; showing source:</p><pre>' +
      escapeHtml(mermaidSrc) + "</pre>";
  }
}

// focusTable centers the pan-zoom viewport on the entity whose rendered
// label text equals entityName (already uppercased/underscored — callers go
// through links.js's entityName so both sides of every focus link use the
// identical transform), then briefly highlights its box. A no-op (not an
// error) when the diagram isn't rendered yet or no matching entity exists —
// deep-linking to a table that doesn't exist in this project shouldn't break
// the page, just silently show the unfocused diagram.
export function focusTable(name) {
  if (!currentSvg || !panZoom) return;

  const group = findEntityGroup(name);
  if (!group) return;

  const box = group.querySelector("rect.entityBox") || group.querySelector("rect");
  const target = box || group;

  const containerRect = $("#er-diagram").getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const dx = containerRect.left + containerRect.width / 2 - (targetRect.left + targetRect.width / 2);
  const dy = containerRect.top + containerRect.height / 2 - (targetRect.top + targetRect.height / 2);
  panZoom.panBy({ x: dx, y: dy });

  if (box) {
    box.classList.remove("er-focus-highlight");
    // Force a reflow so re-adding the class restarts the animation even if
    // the same table is focused twice in a row.
    void box.getBoundingClientRect();
    box.classList.add("er-focus-highlight");
  }
}

// findEntityGroup locates the <g> for the entity whose rendered label text
// matches name, by scanning text.er.entityLabel elements — the rendered
// fact, not Mermaid's internal id/uuid naming. Falls back to an
// id^="entity-<name>-" prefix match (documented in this file's header
// comment as pinned to the vendored mermaid version) if no label match is
// found, in case a future mermaid upgrade changes how labels are marked up
// but keeps the id convention.
function findEntityGroup(name) {
  const labels = currentSvg.querySelectorAll("text.er.entityLabel, text.entityLabel");
  for (const label of labels) {
    if ((label.textContent || "").trim() === name) {
      return label.closest("g");
    }
  }
  return currentSvg.querySelector('g[id^="entity-' + cssEscape(name) + '-"]');
}

// cssEscape guards the id-prefix fallback selector against a table name
// that happens to contain CSS-meaningful characters (entity names are
// uppercased/underscored by entityName, so this is defensive, not expected
// to matter in practice).
function cssEscape(s) {
  return window.CSS && CSS.escape ? CSS.escape(s) : s.replace(/[^a-zA-Z0-9_-]/g, "");
}

// renderNonce gives each Mermaid render call a unique id without relying on
// Date.now(), which some harnesses restrict; a monotonic counter is enough
// since ids only need to be unique within one page session.
let nonce = 0;
function renderNonce() {
  nonce += 1;
  return nonce;
}
