// Pure geometry for the Overview wow moment (issue #39 variant A): the
// abstract constellation the Laravel mark un-ravels into. Points, not the
// literal ER — a 500-table monolith and a 3-table starter both produce a
// legible cloud, because node counts are subsampled to a fixed budget while
// the tally shows the true numbers. No DOM, no canvas: this module computes
// where every entity point comes to rest and which threads connect them, so
// the layout is unit-testable and every replay is the same take (seeded RNG,
// screenshot/GIF-friendly). overview-wow.js owns the choreography.

// NODE_BUDGET caps the drawn points; kinds keep their relative weight.
export const NODE_BUDGET = 120;

// KIND_KEYS fixes the cluster order (and therefore each kind's ring anchor).
export const KIND_KEYS = ["tables", "models", "controllers", "routes", "requests"];

// mulberry32 is a tiny deterministic PRNG — the constellation is layout, not
// chance, so the same project must always produce the same sky.
export function mulberry32(seed) {
  return function () {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let z = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    z = (z + Math.imul(z ^ (z >>> 7), 61 | z)) ^ z;
    return ((z ^ (z >>> 14)) >>> 0) / 4294967296;
  };
}

// buildConstellation lays out the resolved end-state inside a w×h stage:
// one loose cluster per entity kind around a ring, node sizes weighted so
// tables read heaviest and routes finest, plus threads mirroring the
// project's real relation directions (model→table, route→controller,
// controller→model, request→controller). Returns { nodes, edges } where
// edges are index pairs into nodes — an edge can never reference a point
// that is not drawn.
export function buildConstellation(counts, w, h) {
  const rand = mulberry32(39);
  const safeCounts = KIND_KEYS.map((k) => Math.max(0, (counts && counts[k]) | 0));
  const total = safeCounts.reduce((a, b) => a + b, 0);

  const cx = w / 2;
  const cy = h / 2;
  // Elliptical spread: a wide short canvas fills sideways instead of leaving
  // a small cloud floating in the middle.
  const Rx = Math.min(w * 0.3, h * 0.85);
  const Ry = h * 0.34;
  const sizes = [3.2, 2.4, 2.4, 1.7, 2.0]; // per kind, in KIND_KEYS order

  const nodes = [];
  KIND_KEYS.forEach((_, ki) => {
    // Every kind present in the project keeps at least a few points; absent
    // kinds draw nothing. Weighting by true count preserves the project's
    // silhouette under the budget.
    const share = total > 0 ? safeCounts[ki] / total : 0;
    const n = safeCounts[ki] === 0 ? 0 : Math.max(3, Math.round(share * NODE_BUDGET));
    const angle = (ki / KIND_KEYS.length) * Math.PI * 2 - Math.PI / 2;
    const ax = cx + Math.cos(angle) * Rx * 0.62;
    const ay = cy + Math.sin(angle) * Ry * 0.55;
    for (let i = 0; i < n; i++) {
      const a = rand() * Math.PI * 2;
      const r = Math.sqrt(rand());
      nodes.push({
        kind: ki,
        tx: clamp(ax + Math.cos(a) * r * Rx * 0.42, 4, w - 4),
        ty: clamp(ay + Math.sin(a) * r * Ry * 0.5, 4, h - 4),
        size: sizes[ki],
      });
    }
  });

  const byKind = KIND_KEYS.map((_, ki) =>
    nodes.reduce((acc, n, idx) => (n.kind === ki ? (acc.push(idx), acc) : acc), [])
  );
  const edges = [];
  const connect = (fromKind, toKind, ratio) => {
    if (byKind[toKind].length === 0) return;
    for (const from of byKind[fromKind]) {
      if (rand() < ratio) {
        edges.push([from, byKind[toKind][Math.floor(rand() * byKind[toKind].length)]]);
      }
    }
  };
  connect(1, 0, 0.9); // models → tables
  connect(3, 2, 0.45); // routes → controllers
  connect(2, 1, 0.6); // controllers → models
  connect(4, 2, 0.8); // form requests → controllers

  return { nodes, edges };
}

function clamp(v, lo, hi) {
  return Math.min(Math.max(v, lo), hi);
}
