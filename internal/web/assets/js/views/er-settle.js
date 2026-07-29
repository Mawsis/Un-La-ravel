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

// runSettle is the settle's DOM choreography, shared by every diagram that
// settles (ADR 0009 makes the animation itself shared, so the dance belongs
// here next to its geometry rather than being copied per view). It is the one
// impure function in this module and is deliberately parameterized only by what
// actually differs between diagrams:
//
//   root      — the mounted element to search within
//   selector  — the node groups to stage (g.er-node, g.model-node)
//   idAttr    — the attribute naming each group's placement id (data-table,
//               data-model)
//   laidOut   — ELK's output, supplying the center and each node's placement
//
// Callers own the POLICY around it — the ER diagram's once-per-page-session
// gate and the reduced-motion check stay with them, because "when does this
// play" differs per view while "what it does" does not.
//
// The staged nodes are captured by the rAF/cleanup closures rather than re-read
// off `root`, so if an overlapping render replaces the markup mid-settle, the
// pending cleanup runs harmlessly against the now-detached old nodes and never
// touches the new ones.
export function runSettle(root, selector, idAttr, laidOut, cleanupMs) {
  const center = diagramCenter(laidOut || {});
  const placedById = new Map(((laidOut && laidOut.children) || []).map((c) => [c.id, c]));

  const staged = [];
  for (const g of Array.from(root.querySelectorAll(selector))) {
    const placed = placedById.get(g.getAttribute(idAttr));
    if (!placed) continue;
    const { dx, dy } = settleOffset(placed, center);
    // Start displaced outward and faded; arm the transition so the flip eases.
    g.style.transform = `translate(${placed.x + dx}px, ${placed.y + dy}px)`;
    g.style.opacity = "0";
    g.classList.add("er-settling");
    staged.push({ g, placed });
  }
  if (staged.length === 0) return;

  // Next frame: flip to the resolved positions so the armed transition animates
  // the change. rAF (not a synchronous write) is what gives the browser a start
  // frame to interpolate from.
  requestAnimationFrame(() => {
    for (const { g, placed } of staged) {
      g.style.transform = `translate(${placed.x}px, ${placed.y}px)`;
      g.style.opacity = "1";
    }
    // Afterwards drop the inline overrides so nothing lingers to fight pan-zoom
    // or a later focus. transitionend is exact; the duration-matched timeout
    // covers a browser that drops the event (a tab backgrounded mid-transition).
    let done = false;
    const once = () => {
      if (done) return;
      done = true;
      for (const { g } of staged) {
        g.classList.remove("er-settling");
        g.style.transform = "";
        g.style.opacity = "";
      }
    };
    staged[0].g.addEventListener("transitionend", once, { once: true });
    setTimeout(once, cleanupMs);
  });
}
