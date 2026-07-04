// er-graph.js builds the ELK "layered" input graph (issue #26) from the
// server's ER graph contract ({nodes, edges}, see internal/render/er/graph.go).
// It is a pure transform — no DOM, no ELK — so the load-bearing layout facts
// (node sizing, one fixed port per column row, FK edges routed to the exact
// referencing column) are unit-tested directly; ELK only turns this input into
// coordinates the SVG emitter draws.

// Box geometry, shared with the SVG emitter (er-svg.js) so a column row is
// drawn at exactly the y the port was placed at. HEADER_HEIGHT is the table
// name band; each column occupies ROW_HEIGHT below it. NODE_WIDTH is fixed so
// ELK lays boxes out on a predictable grid; the emitter draws text inside it.
export const HEADER_HEIGHT = 34;
export const ROW_HEIGHT = 22;
export const NODE_WIDTH = 220;

// nodeHeight is the box height for a table with n columns: the header band plus
// one row per column.
export function nodeHeight(columnCount) {
  return HEADER_HEIGHT + columnCount * ROW_HEIGHT;
}

// portId names the ELK port for one column of one table. FK edges target the
// port of their exact referencing column, so this id is the single point of
// truth the edge builder addresses a column row by. Table and column names are
// arbitrary strings parsed from PHP source (not validated SQL identifiers), so
// a plain delimiter join would let two distinct pairs alias — e.g. ("a","b::c")
// and ("a::b","c"). JSON-encoding the pair makes the id injective: quoting and
// the array structure keep every (table, column) mapping to a unique string.
export function portId(table, column) {
  return "port:" + JSON.stringify([String(table), String(column)]);
}

// buildElkGraph turns the ER contract into an ELK graph: one child node per
// table (sized for its columns, one fixed-position port per column row) and one
// ELK edge per relationship, attached to the exact referencing column's port
// where the label names one.
export function buildElkGraph(graph) {
  const nodes = (graph && graph.nodes) || [];
  const edges = (graph && graph.edges) || [];

  const columnsByTable = new Map(
    nodes.map((n) => [n.table, new Set((n.columns || []).map((c) => c.name))])
  );

  const children = nodes.map((node) => {
    const columns = node.columns || [];
    return {
      id: node.table,
      width: NODE_WIDTH,
      height: nodeHeight(columns.length),
      // FIXED_POS keeps each port on its column's row, so an FK edge attaches
      // to the exact referencing column and the SVG rows line up with the edges.
      layoutOptions: { "elk.portConstraints": "FIXED_POS" },
      ports: columns.map((col, i) => ({
        id: portId(node.table, col.name),
        // Vertical center of this column's row, measured from the box top.
        y: HEADER_HEIGHT + (i + 0.5) * ROW_HEIGHT,
      })),
    };
  });

  const elkEdges = edges.map((edge, i) => ({
    // Index-suffixed so multiple edges between the same table pair (e.g. a
    // schema FK and an Eloquent relation over the same pair) stay distinct.
    id: "e" + i,
    sources: [endpoint(edge.from, edge, columnsByTable)],
    targets: [endpoint(edge.to, edge, columnsByTable)],
    labels: [{ text: edge.label || "" }],
  }));

  return { id: "root", children, edges: elkEdges };
}

// endpoint resolves one end of an edge to either a column port or the table
// box. If the edge label names a column (e.g. "references (author_id)") and
// THIS table actually has that column, the edge attaches to that column's port;
// otherwise it attaches to the box (the whole node), which is right for
// Eloquent relations and for the parent side of a foreign key.
function endpoint(table, edge, columnsByTable) {
  const col = referencingColumn(edge.label);
  if (col && columnsByTable.get(table) && columnsByTable.get(table).has(col)) {
    return portId(table, col);
  }
  return table;
}

// referencingColumn pulls the column name out of a schema-FK label of the form
// "references (author_id)". Returns "" for labels without a "(...)" column,
// including Eloquent labels like "posts (hasMany)" whose parenthetical is the
// relationship kind, not a column — those never match a real column name so
// endpoint falls back to the box anyway, but returning "" avoids a needless
// lookup and documents the intent.
function referencingColumn(label) {
  const m = /^references \(([^)]+)\)$/.exec(String(label || ""));
  return m ? m[1] : "";
}
