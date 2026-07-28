// model-graph.js builds the MODEL-centric graph for the Model page (issue #70)
// and turns it into ELK input. It is deliberately a different graph from the ER
// diagram's, and the distinction is load-bearing rather than cosmetic:
//
//   ER diagram (er-graph.js)  — nodes are SCHEMA TABLES, edges are foreign keys
//                               and Eloquent associations projected onto tables.
//                               Sized by column count, ports per column row.
//   Model graph (this file)   — nodes are ELOQUENT MODELS, edges are declared
//                               Relationships labeled by kind, centered on ONE
//                               model and bounded to its DIRECT neighbours.
//
// Treating one as the other would conflate Schema and Model, which the domain
// glossary keeps apart on purpose. What IS shared is the visual chrome — the
// node/edge boxes and the settle animation (ADR 0009) — which the page reuses
// by feeding this graph through the same renderer, not by reusing the ER
// diagram's graph.
//
// Both builders here are pure (no DOM, no ELK), so the graph's shape and the
// layout facts are unit-tested directly; ELK only turns the input into
// coordinates the SVG emitter draws.

// Box geometry for a model node. A model node is a single labeled box (name +
// table), not a column table, so it is a fixed size — unlike the ER diagram's
// nodes, which grow with their column count. The center node is drawn wider so
// the subject of the page reads as the subject.
export const MODEL_NODE_WIDTH = 168;
export const MODEL_NODE_HEIGHT = 52;
export const CENTER_NODE_WIDTH = 200;
export const CENTER_NODE_HEIGHT = 60;

// buildModelGraph derives the model-centric graph for `name` from the whole
// Project Model: the named model at the center, one node per DIRECTLY related
// model (in either direction), and one edge per declared Relationship, labeled
// by its kind.
//
// The graph is bounded to ONE HOP on purpose. Following the transitive closure
// would redraw the whole ER diagram under a different name and drown the fact
// the page exists to show — this model's own bindings.
//
// An unknown model yields { found: false } with empty nodes/edges rather than
// throwing, so a deep link to a model absent from this analysis degrades to the
// page's not-found state instead of a blank diagram.
export function buildModelGraph(name, projectModel) {
  const models = (projectModel || {}).models || [];
  const self = models.find((m) => m.name === name) || null;
  if (!self) {
    return { center: name, found: false, nodes: [], edges: [] };
  }

  // Nodes are accumulated through addNode so each model appears exactly once
  // (a self-referential relation must not draw its model twice), while the
  // ARRAY preserves insertion order — center first, then outbound targets in
  // declaration order, then inbound sources in model-discovery order.
  // Determinism is a hard convention (CLAUDE.md): no Set/Map iteration order
  // ever reaches the output.
  const nodes = [];
  const seen = new Set();
  const addNode = (model, isCenter) => {
    if (seen.has(model.name)) return;
    seen.add(model.name);
    nodes.push({
      name: model.name,
      table: model.table || "",
      center: isCenter === true,
      // unresolved marks a node the graph had to invent because a relationship
      // targets a model class the extractor never found (a package model, a
      // typo). Dropping the edge would hide a declared relation; presenting the
      // node as fully known would overstate what was extracted — so it exists
      // and says so, the same honesty the ER edge contract carries.
      unresolved: model.unresolved === true,
    });
  };
  const byName = new Map(models.map((m) => [m.name, m]));
  const resolve = (target) => byName.get(target) || { name: target, table: "", unresolved: true };

  addNode(self, true);

  const edges = [];

  // Outbound: this model's own declared relationships, in source order.
  (self.relationships || []).forEach((r) => {
    addNode(resolve(r.target));
    edges.push({
      from: self.name,
      to: r.target,
      // The KIND is the edge label (what the diagram draws); the METHOD is what
      // the reader greps for in the source, so both survive to the renderer.
      label: r.kind || "",
      kind: r.kind || "",
      method: r.method || "",
      direction: "outbound",
    });
  });

  // Inbound: every OTHER model's relationship that targets this one. A
  // self-referential relation was already emitted as outbound above and must
  // not be counted twice, so this pass skips the center model itself.
  models.forEach((m) => {
    if (m.name === self.name) return;
    (m.relationships || []).forEach((r) => {
      if (r.target !== self.name) return;
      addNode(m);
      edges.push({
        from: m.name,
        to: self.name,
        label: r.kind || "",
        kind: r.kind || "",
        method: r.method || "",
        direction: "inbound",
      });
    });
  });

  return { center: self.name, found: true, nodes, edges };
}

// buildModelElkGraph turns the model graph into ELK input: one child per model
// node, sized by whether it is the center, and one ELK edge per relationship.
// Edges attach to the node BOX (not to ports): a model-graph edge is a
// relationship between two classes, with no column to anchor to — the ER
// diagram's per-column ports have no meaning here.
//
// Edge ids are index-suffixed so two relationships between the same pair of
// models (a Post's author AND editor, both Users) stay distinct edges rather
// than colliding into one.
export function buildModelElkGraph(graph) {
  const nodes = (graph && graph.nodes) || [];
  const children = nodes.map((n) => ({
    id: n.name,
    width: n.center ? CENTER_NODE_WIDTH : MODEL_NODE_WIDTH,
    height: n.center ? CENTER_NODE_HEIGHT : MODEL_NODE_HEIGHT,
  }));

  // The same never-crashes backstop the ER diagram carries (issue #36): ELK
  // throws on an edge whose endpoint has no node, and one bad edge blanks the
  // whole diagram. buildModelGraph adds a node for every endpoint it emits, so
  // this filter should never drop anything — it makes a future regression
  // degrade to a missing edge instead of a blank page.
  const elkEdges = drawableModelEdges(graph).map((e, i) => ({
    id: "me" + i,
    sources: [e.from],
    targets: [e.to],
    labels: [{ text: e.label || "" }],
  }));

  return { id: "root", children, edges: elkEdges };
}

// drawableModelEdges returns the edges whose BOTH endpoints exist as nodes, in
// graph order. It is the single point of truth for which edges get laid out, so
// the SVG emitter can correlate an ELK edge index back into this same list.
export function drawableModelEdges(graph) {
  const known = new Set(((graph && graph.nodes) || []).map((n) => n.name));
  return ((graph && graph.edges) || []).filter((e) => known.has(e.from) && known.has(e.to));
}
