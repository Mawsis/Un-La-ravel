// er-markers.js maps a graph edge's cardinality kind to the crow's-foot marker
// at each end of the drawn edge (issue #26). It is pure — no SVG, no DOM — so
// the reading a viewer decodes ("one" side vs "many" side) is unit-tested on
// its own; er-svg.js turns these end names into the actual marker paths.

// The two crow's-foot end kinds. ONE draws a single perpendicular tick (exactly
// one); MANY draws the three-line crow's foot (many). A null end draws nothing.
export const ONE = "one";
export const MANY = "many";

// edgeMarkers returns { from, to } — the marker end at the edge's source (from)
// and target (to) tables. The cardinality kinds mirror the server's graph
// contract: a schema foreign key and hasMany are one-to-many (the parent is the
// "one", the FK-bearing child is the "many"); belongsTo is many-to-one; hasOne
// is one-to-one; belongsToMany is many-to-many. An unrecognized kind yields no
// markers so the edge still draws as a plain line.
export function edgeMarkers(kind) {
  switch (kind) {
    case "one-to-many":
      return { from: ONE, to: MANY };
    case "many-to-one":
      return { from: MANY, to: ONE };
    case "one-to-one":
      return { from: ONE, to: ONE };
    case "many-to-many":
      return { from: MANY, to: MANY };
    default:
      return { from: null, to: null };
  }
}
