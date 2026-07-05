// er-svg.js emits the ER diagram as owned SVG markup (issue #26) from an
// ELK-laid-out graph plus the source {nodes, edges} contract. It does no layout
// math — ELK produced every coordinate and edge section — and touches no DOM;
// it returns a string the shell (er.js) mounts. Keeping it a pure string
// transform is what lets what-gets-drawn be unit-tested without a browser.

import { escapeHtml } from "../dom.js";
import { HEADER_HEIGHT, ROW_HEIGHT, drawableEdges } from "./er-graph.js";
import { edgeMarkers, ONE, MANY } from "./er-markers.js";

// renderSvg turns ELK's output into SVG. `layout` is the laid-out graph
// (root width/height, children with x/y/width/height, edges with sections);
// `graph` is the source contract, read for each table's columns and key markers
// (ELK's output drops them). Nodes correlate to the contract by id === table;
// edges correlate by array index into drawableEdges(graph) — the exact list
// buildElkGraph laid out, so a dropped orphan edge (issue #36) cannot shift
// the pairing.
export function renderSvg(layout, graph) {
  const width = Math.ceil(layout.width || 0);
  const height = Math.ceil(layout.height || 0);
  const nodesByTable = new Map(((graph && graph.nodes) || []).map((n) => [n.table, n]));
  const contractEdges = drawableEdges(graph);

  const boxes = (layout.children || [])
    .map((placed) => entityBox(placed, nodesByTable.get(placed.id)))
    .join("");

  const edges = (layout.edges || [])
    .map((placed, i) => edgeLine(placed, contractEdges[i]))
    .join("");

  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}" ` +
    `class="er-svg">` +
    markerDefs() +
    `<g class="er-edges">${edges}</g>` +
    `<g class="er-nodes">${boxes}</g>` +
    `</svg>`
  );
}

// markerDefs defines the crow's-foot markers once, referenced by every edge.
// "one" is a single perpendicular tick; "many" is the three-line crow's foot.
// orient="auto-start-reverse" makes a marker at the line START point the right
// way round (SVG would otherwise draw start markers pointing backwards).
function markerDefs() {
  return (
    `<defs>` +
    `<marker id="er-marker-one" class="er-marker" markerWidth="14" markerHeight="14" ` +
    `refX="10" refY="7" orient="auto-start-reverse" markerUnits="userSpaceOnUse">` +
    `<path d="M6,2 L6,12"/></marker>` +
    `<marker id="er-marker-many" class="er-marker" markerWidth="16" markerHeight="14" ` +
    `refX="12" refY="7" orient="auto-start-reverse" markerUnits="userSpaceOnUse">` +
    `<path d="M12,2 L0,7 L12,12 M0,7 L12,7"/></marker>` +
    `</defs>`
  );
}

// edgeLine draws one relationship as a polyline through ELK's routed points,
// with the crow's-foot marker its cardinality calls for at each end. `contract`
// is the source edge (for its kind); a section without geometry draws nothing.
function edgeLine(placed, contract) {
  const section = (placed.sections || [])[0];
  if (!section) return "";

  const pts = [section.startPoint, ...(section.bendPoints || []), section.endPoint]
    .map((p) => `${round(p.x)},${round(p.y)}`)
    .join(" ");

  const { from, to } = edgeMarkers(contract ? contract.kind : "");
  const startAttr = from ? ` marker-start="url(#er-marker-${markerName(from)})"` : "";
  const endAttr = to ? ` marker-end="url(#er-marker-${markerName(to)})"` : "";

  return (
    `<polyline class="${edgeClasses(contract)}" points="${pts}" fill="none"` +
    `${startAttr}${endAttr}/>`
  );
}

// edgeClasses derives an edge's classes from the contract's explicit fields
// (1.7.0): origin separates schema foreign-key edges from Eloquent-association
// edges (issue #26 requires both shown and distinguishable), and a reconciled
// edge additionally carries er-edge-unresolved so the repair is visible at a
// glance (issue #36). Pre-1.7.0 contracts have no origin field; the old label
// sniff ("references" / "references (col)" means schema FK) stays as the
// fallback.
function edgeClasses(contract) {
  const edge = contract || {};
  const origin = edge.origin || sniffOrigin(edge.label);
  const classes = ["er-edge", origin === "schema" ? "er-edge-schema" : "er-edge-eloquent"];
  if (edge.unresolved) classes.push("er-edge-unresolved");
  return classes.join(" ");
}

// sniffOrigin infers an edge's origin from its label, the pre-1.7.0 heuristic.
function sniffOrigin(label) {
  return /^references(\s|$)/.test(String(label || "")) ? "schema" : "eloquent";
}

// markerName maps a marker end kind to its <marker> id suffix.
function markerName(end) {
  return end === MANY ? "many" : end === ONE ? "one" : "";
}

// round keeps ELK's sub-pixel coordinates from bloating the markup while
// staying visually identical.
function round(n) {
  return Math.round(n * 100) / 100;
}

// entityBox draws one table: a positioned group carrying data-table (the focus
// hook), the box outline, the header band with the table name, and one row per
// column with its name, type, and PK/FK/UK badge.
function entityBox(placed, node) {
  const columns = (node && node.columns) || [];
  const rows = columns.map((col, i) => columnRow(col, i)).join("");

  return (
    `<g class="er-node" data-table="${escapeHtml(placed.id)}" ` +
    `transform="translate(${placed.x}, ${placed.y})">` +
    `<rect class="er-box" width="${placed.width}" height="${placed.height}" rx="6"/>` +
    `<rect class="er-header" width="${placed.width}" height="${HEADER_HEIGHT}" rx="6"/>` +
    `<text class="er-title" x="10" y="${HEADER_HEIGHT / 2}" dy="0.35em">` +
    `${escapeHtml(placed.id)}</text>` +
    rows +
    `</g>`
  );
}

// columnRow draws one column row at its vertical offset below the header: the
// key badge (when any), the column name, and its type on the right.
function columnRow(col, i) {
  const top = HEADER_HEIGHT + i * ROW_HEIGHT;
  const mid = top + ROW_HEIGHT / 2;
  const badge = col.key
    ? `<text class="er-badge er-badge-${escapeHtml(col.key.toLowerCase())}" ` +
      `x="10" y="${mid}" dy="0.35em">${escapeHtml(col.key)}</text>`
    : "";

  return (
    `<g class="er-row">` +
    badge +
    `<text class="er-col-name" x="44" y="${mid}" dy="0.35em">${escapeHtml(col.name)}</text>` +
    `<text class="er-col-type" x="210" y="${mid}" dy="0.35em" text-anchor="end">` +
    `${escapeHtml(col.type)}</text>` +
    `</g>`
  );
}
