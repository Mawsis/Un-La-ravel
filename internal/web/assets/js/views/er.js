// ER Diagram view (issue #26): a hand-rolled SVG renderer laid out by ELK,
// replacing the former Mermaid diagram and its private-DOM focus code. This
// module is the thin browser shell — it calls ELK, feeds the pure builders
// (er-graph → ELK input, er-svg → SVG markup, er-markers → crow's-foot ends),
// mounts the result, and drives pan/zoom and focus. All the layout/geometry
// logic it depends on is pure and unit-tested (jstest/er-graph, er-svg,
// er-markers); this file owns only the ELK call and the DOM wiring.
//
// Focus is now a trivial id lookup: the renderer OWNS the SVG, so every entity
// group carries data-table="<name>", and focusTable finds its target with one
// querySelector — no reverse-engineering a vendored library's internal ids.

import { $ } from "../dom.js";
import { buildElkGraph } from "./er-graph.js";
import { renderSvg } from "./er-svg.js";

// ELK layout options: a layered (Sugiyama) left-to-right graph, which reads as
// "parents on the left, dependents to the right" — the natural direction for
// foreign-key/association arrows. Node and edge spacing keep boxes legible.
const LAYOUT_OPTIONS = {
  "elk.algorithm": "layered",
  "elk.direction": "RIGHT",
  "elk.spacing.nodeNode": "48",
  "elk.layered.spacing.nodeNodeBetweenLayers": "72",
  "elk.spacing.edgeNode": "24",
};

// A single ELK instance; elk.bundled.js runs layout in the main thread (no
// worker file needed), so one instance is reused across analyses.
const elk = window.ELK ? new window.ELK() : null;

let panZoom = null;
let currentSvg = null; // the mounted <svg>, so focusTable can run after renderER

// renderGeneration guards two overlapping renderER calls: ELK layout is async,
// so if a second render starts before the first resolves, the first must be
// discarded rather than mounted (which would leak the winner's pan-zoom
// listeners and overwrite currentSvg with a stale instance). Each call captures
// the generation it began at and only mounts while still current.
let renderGeneration = 0;

// renderER lays out the ER graph with ELK, mounts the owned SVG, wires
// svg-pan-zoom, and (if focusName is given) focuses that table on the same
// render that created the pan-zoom instance. `graph` is the server's ER graph
// contract ({nodes, edges}); an empty or schema-less graph degrades to a
// graceful empty state rather than a broken diagram.
export async function renderER(graph, focusName) {
  const myGeneration = ++renderGeneration;
  const container = $("#er-diagram");
  if (!container) return;

  teardown();

  const nodes = (graph && graph.nodes) || [];
  if (nodes.length === 0 || !elk) {
    if (myGeneration === renderGeneration) {
      container.innerHTML = '<p class="hint">No schema to diagram.</p>';
    }
    return;
  }

  try {
    const input = buildElkGraph(graph);
    input.layoutOptions = LAYOUT_OPTIONS;
    const laidOut = await elk.layout(input);

    // A newer renderER started while we awaited ELK — it already tore down our
    // pan-zoom precursor and will mount its own; drop this stale result.
    if (myGeneration !== renderGeneration) return;

    container.innerHTML = renderSvg(laidOut, graph);
    mount(container, focusName);
  } catch (e) {
    if (myGeneration !== renderGeneration) return;
    // Surface the failure for diagnosis (a malformed graph or an ELK
    // regression) — the user sees a generic hint, but the cause shouldn't
    // vanish silently.
    console.error("ER diagram render failed:", e);
    container.innerHTML = '<p class="hint">Could not render the ER diagram.</p>';
  }
}

// mount wires svg-pan-zoom onto the freshly-drawn SVG and applies the initial
// focus, if any. Kept separate so renderER reads as layout → mount.
function mount(container, focusName) {
  const svgEl = container.querySelector("svg");
  if (!svgEl) return;

  // Fill the fixed-height container; pan-zoom drives the viewport.
  svgEl.style.width = "100%";
  svgEl.style.height = "100%";
  svgEl.style.maxWidth = "none";
  currentSvg = svgEl;

  if (window.svgPanZoom) {
    panZoom = window.svgPanZoom(svgEl, {
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

  if (focusName) focusTable(focusName);
}

// teardown destroys the previous pan-zoom instance before its SVG is replaced,
// so its listeners don't leak across analyses.
function teardown() {
  if (panZoom) {
    panZoom.destroy();
    panZoom = null;
  }
  currentSvg = null;
}

// focusTable centers the viewport on the entity whose data-table attribute
// equals name, then briefly highlights its box. A no-op (not an error) when the
// diagram isn't mounted yet or no such table exists — deep-linking to a table
// absent from this project shouldn't break the page, just show it unfocused.
// name is the raw table name (links.js passes it through verbatim now that the
// renderer owns the SVG and tags each group with the raw name).
export function focusTable(name) {
  if (!currentSvg || !panZoom) return;

  const group = findEntityGroup(name);
  if (!group) return;

  const box = group.querySelector("rect.er-box") || group.querySelector("rect");
  const target = box || group;

  const containerRect = $("#er-diagram").getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const dx = containerRect.left + containerRect.width / 2 - (targetRect.left + targetRect.width / 2);
  const dy = containerRect.top + containerRect.height / 2 - (targetRect.top + targetRect.height / 2);
  panZoom.panBy({ x: dx, y: dy });

  if (box) {
    box.classList.remove("er-focus-highlight");
    // Force a reflow so re-adding the class restarts the animation even when
    // the same table is focused twice in a row.
    void box.getBoundingClientRect();
    box.classList.add("er-focus-highlight");
  }
}

// findEntityGroup locates the <g> the renderer drew for `name`, by the
// data-table attribute it OWNS — a stable fact of our own markup, not a
// vendored library's internal id scheme.
function findEntityGroup(name) {
  return currentSvg.querySelector('g.er-node[data-table="' + cssEscape(name) + '"]');
}

// cssEscape guards the attribute selector against a table name containing
// CSS-meaningful characters. Table names are arbitrary strings from parsed PHP
// source, so a name with a "]" or quote would otherwise make querySelector
// throw a SyntaxError instead of the documented no-op. Native CSS.escape is the
// real path in every supported browser; the fallback escapes every character
// outside the CSS identifier-safe set, matching CSS.escape's guarantee.
function cssEscape(s) {
  if (window.CSS && CSS.escape) return CSS.escape(String(s));
  return String(s).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
}
