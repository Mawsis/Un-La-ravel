// ER Diagram view: Mermaid rendering + svg-pan-zoom navigation. Ported from
// the original renderER in app.js. focusTable (cross-navigation from other
// views, design.md "Cross-navigation") lands in PR 5 — this module exposes
// the pan-zoom instance so that later addition doesn't need to touch
// rendering logic again.

import { $, escapeHtml } from "../dom.js";

let panZoom = null;

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

export async function renderER(mermaidSrc) {
  const container = $("#er-diagram");

  // Tear down the previous pan-zoom instance before replacing the SVG,
  // otherwise its listeners leak across analyses.
  if (panZoom) {
    panZoom.destroy();
    panZoom = null;
  }
  if (!mermaidSrc || !window.mermaid) {
    container.innerHTML = '<p class="hint">No schema to diagram.</p>';
    return;
  }
  try {
    const { svg } = await mermaid.render("er-graph-" + renderNonce(), mermaidSrc);
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
    }
  } catch (e) {
    // Fall back to showing the source if Mermaid can't parse it.
    container.innerHTML =
      '<p class="hint">Could not render the diagram; showing source:</p><pre>' +
      escapeHtml(mermaidSrc) + "</pre>";
  }
}

// renderNonce gives each Mermaid render call a unique id without relying on
// Date.now(), which some harnesses restrict; a monotonic counter is enough
// since ids only need to be unique within one page session.
let nonce = 0;
function renderNonce() {
  nonce += 1;
  return nonce;
}
