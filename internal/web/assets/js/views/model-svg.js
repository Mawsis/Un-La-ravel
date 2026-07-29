// model-svg.js emits the model-centric graph as owned SVG markup (issue #70)
// from an ELK-laid-out graph plus the source model graph. Like er-svg.js it
// does no layout math (ELK produced every coordinate) and touches no DOM — it
// returns a string the page mounts — so what gets drawn is unit-testable
// without a browser.
//
// It REUSES the ER diagram's visual chrome deliberately: the same er-node /
// er-box classes the CSS paints and the settle animation stages (ADR 0009), so
// a model graph resolves on screen exactly the way the ER diagram does. What it
// does NOT reuse is the ER graph's SHAPE or its semantics — the nodes here are
// Eloquent models, not schema tables, so the focus hook is data-model (not
// data-table) and the edges carry no schema/eloquent origin split, because
// every edge in a model graph is an Eloquent relationship by construction.

import { escapeHtml } from "../dom.js";

// Type-band height inside a model box: the class name sits above, the mapped
// table name below it in the smaller band.
const NAME_BAND = 30;

// renderModelSvg turns ELK's output into SVG. `layout` is the laid-out graph
// (root width/height, children with x/y/width/height, edges with sections);
// `graph` is the source model graph, read for each node's table/center/
// unresolved flags and each edge's label (ELK's output drops them). Nodes
// correlate to the source by id === model name; edges correlate by array index
// into the source edge list, the exact list buildModelElkGraph laid out.
export function renderModelSvg(layout, graph) {
  const width = Math.ceil((layout && layout.width) || 0);
  const height = Math.ceil((layout && layout.height) || 0);
  const nodesByName = new Map(((graph && graph.nodes) || []).map((n) => [n.name, n]));
  const sourceEdges = (graph && graph.edges) || [];

  const boxes = ((layout && layout.children) || [])
    .map((placed) => modelBox(placed, nodesByName.get(placed.id)))
    .join("");

  const edges = ((layout && layout.edges) || [])
    .map((placed, i) => modelEdge(placed, sourceEdges[i]))
    .join("");

  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}" ` +
    `class="model-svg">` +
    `<g class="model-edges">${edges}</g>` +
    `<g class="model-nodes">${boxes}</g>` +
    `</svg>`
  );
}

// modelBox draws one model: a positioned group carrying data-model (the focus
// hook this graph OWNS — deliberately not data-table, which would let the ER
// diagram's focus code match a model name against a table name), the shared ER
// box chrome, the class name, and the mapped table beneath it.
function modelBox(placed, node) {
  const n = node || {};
  const classes = ["er-node", "model-node"];
  if (n.center) classes.push("model-node-center");
  if (n.unresolved) classes.push("model-node-unresolved");

  // An unresolved node has no extracted table to name; saying "not extracted"
  // is honest where an empty band would read as "no table".
  const sub = n.unresolved ? "not extracted" : n.table || "";
  const subText = sub
    ? `<text class="model-node-table" x="12" y="${NAME_BAND + 12}" dy="0.35em">${escapeHtml(sub)}</text>`
    : "";

  return (
    `<g class="${classes.join(" ")}" data-model="${escapeHtml(placed.id)}" ` +
    `transform="translate(${placed.x}, ${placed.y})">` +
    `<rect class="er-box" width="${placed.width}" height="${placed.height}" rx="6"/>` +
    `<text class="model-node-name" x="12" y="${NAME_BAND / 2 + 4}" dy="0.35em">` +
    `${escapeHtml(placed.id)}</text>` +
    subText +
    `</g>`
  );
}

// modelEdge draws one relationship as a polyline through ELK's routed points,
// labeled with the relationship KIND at the segment's midpoint — the label is
// what the reader decodes the diagram with, so it travels with the line rather
// than living in a legend. A section without geometry draws nothing (the same
// never-crashes posture er-svg.js takes) rather than a line to nowhere.
function modelEdge(placed, source) {
  const section = ((placed && placed.sections) || [])[0];
  if (!section || !section.startPoint || !section.endPoint) return "";

  const points = [section.startPoint, ...(section.bendPoints || []), section.endPoint];
  const pts = points.map((p) => `${round(p.x)},${round(p.y)}`).join(" ");

  const label = (source && source.label) || "";
  const mid = points[Math.floor(points.length / 2)];
  const labelText = label
    ? `<text class="model-edge-label" x="${round(mid.x)}" y="${round(mid.y) - 6}" ` +
      `text-anchor="middle">${escapeHtml(label)}</text>`
    : "";

  // model-edge, not the ER diagram's er-edge-schema/er-edge-eloquent pair: that
  // split separates schema foreign keys from Eloquent associations, and every
  // edge here is an Eloquent relationship, so claiming either would be false.
  // Direction (inbound/outbound) is deliberately NOT a class: ELK already draws
  // it as the line's direction, and a modifier with no rule behind it is a
  // styling hook for a need that does not exist.
  return `<polyline class="model-edge" points="${pts}" fill="none"/>` + labelText;
}

// round keeps ELK's sub-pixel coordinates from bloating the markup while
// staying visually identical (same rule as er-svg.js).
function round(n) {
  return Math.round(n * 100) / 100;
}
