// Unit tests for the model-graph SVG emitter (issue #70).
//
// renderModelSvg is the pure transform from an ELK-laid-out model graph plus
// the source {nodes, edges} into the SVG markup the Model page mounts. It
// deliberately reuses the ER diagram's visual chrome — the same box/edge
// classes and the same data-* focus hooks the settle animation reads — while
// drawing a DIFFERENT graph: model boxes labeled by class name and table, and
// edges labeled by relationship kind. The tests below pin both halves: that the
// chrome is genuinely shared (so the settle and the styling apply unchanged),
// and that what is drawn is a model graph, never a table graph.

import { test } from "node:test";
import assert from "node:assert/strict";

import { renderModelSvg } from "../assets/js/views/model-svg.js";

// A center model with one neighbour, laid out. `layout` is shaped like ELK's
// output; `graph` is the source model graph the emitter reads labels from.
const twoNodes = {
  layout: {
    width: 500,
    height: 120,
    children: [
      { id: "Post", x: 0, y: 30, width: 200, height: 60 },
      { id: "User", x: 300, y: 34, width: 168, height: 52 },
    ],
    edges: [
      {
        sections: [{ startPoint: { x: 200, y: 60 }, endPoint: { x: 300, y: 60 } }],
      },
    ],
  },
  graph: {
    center: "Post",
    found: true,
    nodes: [
      { name: "Post", table: "posts", center: true, unresolved: false },
      { name: "User", table: "users", center: false, unresolved: false },
    ],
    edges: [
      { from: "Post", to: "User", label: "belongsTo", kind: "belongsTo", method: "author", direction: "outbound" },
    ],
  },
};

test("emits an <svg> sized to the laid-out graph", () => {
  const svg = renderModelSvg(twoNodes.layout, twoNodes.graph);
  assert.match(svg, /^<svg\b/);
  assert.match(svg, /viewBox="0 0 500 120"/);
});

test("each model is drawn as a node group carrying its name as the focus hook", () => {
  const svg = renderModelSvg(twoNodes.layout, twoNodes.graph);
  // data-model, NOT data-table: the hook names the MODEL this box is, so a
  // focus/deep-link resolves against the graph actually drawn. Reusing
  // data-table would make the ER diagram's focus code silently match a model
  // name against a table name.
  assert.match(svg, /data-model="Post"/);
  assert.match(svg, /data-model="User"/);
});

test("reuses the ER node chrome class so the shared settle animation applies", () => {
  const svg = renderModelSvg(twoNodes.layout, twoNodes.graph);
  // er-node is what the settle stages and what the CSS paints. Sharing it is
  // the point of "reuses the ER renderer's chrome" — a bespoke class would mean
  // reimplementing both.
  assert.match(svg, /class="[^"]*\ber-node\b/);
  assert.match(svg, /class="er-box"/);
});

test("the center model is marked so the page's subject reads as the subject", () => {
  const svg = renderModelSvg(twoNodes.layout, twoNodes.graph);
  const postGroup = /<g[^>]*data-model="Post"[^>]*>/.exec(svg)[0];
  const userGroup = /<g[^>]*data-model="User"[^>]*>/.exec(svg)[0];
  assert.match(postGroup, /model-node-center/);
  assert.doesNotMatch(userGroup, /model-node-center/);
});

test("a node shows the model name and its mapped table", () => {
  const svg = renderModelSvg(twoNodes.layout, twoNodes.graph);
  assert.match(svg, />Post</);
  assert.match(svg, />posts</);
  assert.match(svg, />User</);
  assert.match(svg, />users</);
});

test("an unresolved model node says so rather than implying it was extracted", () => {
  const svg = renderModelSvg(
    {
      width: 200,
      height: 60,
      children: [{ id: "Tenant", x: 0, y: 0, width: 168, height: 52 }],
      edges: [],
    },
    {
      center: "Post",
      found: true,
      nodes: [{ name: "Tenant", table: "", center: false, unresolved: true }],
      edges: [],
    }
  );
  assert.match(svg, /model-node-unresolved/);
});

test("an edge is drawn through ELK's routed points and labeled by relationship kind", () => {
  const svg = renderModelSvg(twoNodes.layout, twoNodes.graph);
  assert.match(svg, /<polyline[^>]*points="200,60 300,60"/);
  // The KIND is the label the reader decodes the diagram with.
  assert.match(svg, />belongsTo</);
});

test("edges are classed as model-graph edges, distinct from ER's schema/eloquent split", () => {
  const svg = renderModelSvg(twoNodes.layout, twoNodes.graph);
  // The ER diagram splits edges by ORIGIN (schema FK vs Eloquent association).
  // Every model-graph edge is an Eloquent relationship by construction, so that
  // split has no meaning here — carrying er-edge-schema would be a false claim.
  assert.match(svg, /class="[^"]*\bmodel-edge\b/);
  assert.doesNotMatch(svg, /er-edge-schema/);
});

test("an edge whose ELK section has no geometry draws nothing rather than a broken line", () => {
  const svg = renderModelSvg(
    { width: 10, height: 10, children: [], edges: [{ sections: [] }] },
    { center: "Post", found: true, nodes: [], edges: [{ from: "Post", to: "User", label: "hasMany" }] }
  );
  assert.doesNotMatch(svg, /<polyline/);
});

test("model names are escaped, never injected as markup", () => {
  // Model names come from parsed PHP source, not a validated identifier set.
  const svg = renderModelSvg(
    {
      width: 200,
      height: 60,
      children: [{ id: '<img src=x onerror=alert(1)>', x: 0, y: 0, width: 168, height: 52 }],
      edges: [],
    },
    {
      center: "x",
      found: true,
      nodes: [{ name: '<img src=x onerror=alert(1)>', table: "", center: false, unresolved: false }],
      edges: [],
    }
  );
  assert.doesNotMatch(svg, /<img/);
  assert.match(svg, /&lt;img/);
});

test("an empty graph emits a valid empty svg, not malformed markup", () => {
  const svg = renderModelSvg({ width: 0, height: 0, children: [], edges: [] }, { nodes: [], edges: [] });
  assert.match(svg, /^<svg\b/);
  assert.match(svg, /<\/svg>$/);
});
