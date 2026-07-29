// model-graph-view.js is the thin browser shell for the Model page's graph
// (issue #70) — the model-graph counterpart to er.js. It calls ELK, feeds the
// pure builders (model-graph → ELK input, model-svg → SVG markup), mounts the
// result, and plays the settle. All the layout and markup logic it depends on
// is pure and unit-tested (jstest/model-graph, model-svg); this file owns only
// the ELK call and the DOM wiring, which is why it has no tests of its own —
// exactly the split er.js takes.
//
// The settle is the shared one (ADR 0009 / issue #30): the same er-settling
// class, the same outward-displacement geometry from er-settle.js, the same
// reduced-motion guard. Only the graph being settled is different.

import { $ } from "../dom.js";
import { buildModelElkGraph } from "./model-graph.js";
import { renderModelSvg } from "./model-svg.js";
import { runSettle } from "./er-settle.js";
import { prefersReducedMotion, SETTLE_CLEANUP_MS } from "../motion.js";

// A model graph is small (one model and its direct neighbours), so it is laid
// out left-to-right with tighter spacing than the ER diagram — the relationship
// direction reads along the flow, and the whole graph fits without panning.
const LAYOUT_OPTIONS = {
  "elk.algorithm": "layered",
  "elk.direction": "RIGHT",
  "elk.spacing.nodeNode": "32",
  "elk.layered.spacing.nodeNodeBetweenLayers": "96",
  "elk.spacing.edgeNode": "20",
};

// A single ELK instance, reused across renders (elk.bundled.js lays out on the
// main thread, so no worker file is needed). Null when ELK is absent — the
// caller then gets the graceful note below rather than a crash.
//
// The `typeof window` guard is not defensive noise: model-detail.js imports
// this module for renderModelGraph, and the jstests import model-detail.js
// under Node to exercise its PURE compose/template functions. A bare
// `window.ELK` here would make that import throw before a single test ran, the
// same browser-global guard router.js carries for its hashchange listener.
const elk = typeof window !== "undefined" && window.ELK ? new window.ELK() : null;

// renderGeneration guards two overlapping renders: ELK layout is async, so if a
// second render starts before the first resolves, the first must be discarded
// rather than mounted over the newer one. Each call captures the generation it
// began at and only mounts while still current. The Model page re-renders on
// every navigation between models, so this race is routine here, not theoretical.
let renderGeneration = 0;

// renderModelGraph lays out the model graph with ELK and mounts the owned SVG
// into #model-graph. A no-op when the mount point is absent (the section is only
// emitted for a model that HAS neighbours) so calling it unconditionally after
// every detail render is safe.
export async function renderModelGraph(graph) {
  const myGeneration = ++renderGeneration;
  const container = $("#model-graph");
  if (!container) return;

  const nodes = (graph && graph.nodes) || [];
  if (nodes.length === 0 || !elk) {
    container.innerHTML = '<p class="hint">No model graph to draw.</p>';
    return;
  }

  try {
    const input = buildModelElkGraph(graph);
    input.layoutOptions = LAYOUT_OPTIONS;
    const laidOut = await elk.layout(input);

    // A newer render started while we awaited ELK — drop this stale result
    // rather than overwriting the newer graph with an older model's.
    if (myGeneration !== renderGeneration) return;

    container.innerHTML = renderModelSvg(laidOut, graph);
    playSettle(container, laidOut);
  } catch (e) {
    if (myGeneration !== renderGeneration) return;
    // Surface the cause for diagnosis; the user sees a calm note. A graph that
    // won't lay out must not take the rest of the detail page down with it.
    console.error("Model graph render failed:", e);
    container.innerHTML = '<p class="hint">Could not render the model graph.</p>';
  }
}

// playSettle runs the shared settle on the freshly-drawn model nodes: each box
// eases from a rough, outward-displaced start into its laid-out position (issue
// #30, ADR 0009). Unlike the ER diagram's once-per-page-session gate, this one
// plays on EVERY model graph mount — the ER settle is the signature moment of
// an analysis resolving (one per session by design), whereas here it is the
// per-model transition that makes navigating between models legible. Skipped
// entirely under reduced motion.
//
// The staged nodes are captured by the rAF/cleanup closures, not re-read off
// the container, so if an overlapping render replaces the SVG mid-settle the
// pending cleanup runs harmlessly against the detached old nodes.
function playSettle(container, laidOut) {
  if (prefersReducedMotion()) return;
  runSettle(container, "g.model-node", "data-model", laidOut, SETTLE_CLEANUP_MS);
}
