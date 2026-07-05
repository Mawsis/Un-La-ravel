// er-settle.js holds the pure geometry of the ER "settle" animation (issue #30)
// — the un-ravel metaphor at the moment structure resolves. On first render,
// entity boxes ease from a rough, unresolved arrangement into the exact
// positions ELK laid out. This module owns only WHERE a node starts before it
// settles (a pure function, unit-tested without a browser); er.js owns the DOM
// choreography, the once-per-analysis gate, and the reduced-motion check.

// How far, as a fraction of a node's distance from center, its rough start is
// displaced outward. Restrained on purpose (issue #30: "restrained; plays
// once") — enough to read as a resolve, not a fling.
const SPREAD = 0.22;

// diagramCenter is the midpoint of the laid-out graph, derived from ELK's
// overall width/height. Nodes settle toward it, so the whole diagram converges
// on its own middle independent of the current pan/zoom.
export function diagramCenter(layout) {
  return { x: (layout.width || 0) / 2, y: (layout.height || 0) / 2 };
}

// settleOffset returns the {dx, dy} a node is displaced by at the START of the
// settle — outward from the diagram center, so the animation eases each box
// inward to its laid-out position (the tangle resolving). `node` is the
// resolved placement (x/y top-left, width/height); `center` is the diagram
// midpoint the boxes settle toward.
export function settleOffset(node, center) {
  const cx = node.x + node.width / 2;
  const cy = node.y + node.height / 2;
  return {
    dx: (cx - center.x) * SPREAD,
    dy: (cy - center.y) * SPREAD,
  };
}
