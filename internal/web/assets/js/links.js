// Central link builder (design.md "Cross-navigation"): every entity mention
// anywhere in the dashboard becomes a link through this one function, so the
// mapping from "a route/model/table/finding" to "the view that explains it"
// is defined exactly once.
//
// hrefFor never touches the DOM or the router directly — it returns a plain
// href string built from the CURRENT router params (so path and any other
// state the destination view needs to stay valid are preserved), letting
// every view module stay a plain template generator.

// entityName mirrors internal/render/er/er.go's entityName exactly: trim,
// collapse internal whitespace runs to single underscores, uppercase. This
// is how the ER renderer names a Mermaid entity from a table name, and it's
// the string er.js's focusTable matches against the rendered diagram's
// entity labels.
//
// Uppercasing is done per-character via simpleUpper, NOT String.toUpperCase,
// because JS's toUpperCase applies full Unicode case mapping (e.g. German
// "ß" expands to "SS"), while Go's strings.ToUpper (what er.go actually
// calls) applies simple, one-codepoint-to-one-codepoint mapping and leaves
// "ß" unchanged. For a table name containing such a character, the two
// would silently diverge — er.js's findEntityGroup would never match
// Mermaid's actual rendered label and focusTable would no-op with no error.
export function entityName(table) {
  return simpleUpper(
    String(table || "")
      .trim()
      .split(/\s+/)
      .join("_")
  );
}

function simpleUpper(s) {
  return Array.from(s)
    .map((ch) => {
      const upper = ch.toUpperCase();
      // Only accept a per-character mapping that stays one character —
      // this is what makes it "simple" (Go's behavior), rejecting the
      // multi-character expansions JS's toUpperCase performs for a
      // handful of characters (ß, ﬁ, ﬀ, and similar ligatures/expansions).
      return Array.from(upper).length === 1 ? upper : ch;
    })
    .join("");
}

// baseParams returns a fresh URLSearchParams carrying only "path" from the
// current params — every link below builds on this rather than the full
// current param set, so navigating via a link never drags a stale filter or
// sort value from the view being left into the view being entered.
function baseParams(currentParams) {
  const next = new URLSearchParams();
  const path = currentParams.get("path");
  if (path) next.set("path", path);
  return next;
}

function hash(view, params) {
  const qs = params.toString();
  return "#/" + view + (qs ? "?" + qs : "");
}

// hrefFor builds the href for one cross-navigation link. currentParams is
// the router's current params (for the "path" carried forward); kind
// dispatches to the right view + focus param.
export function hrefFor(kind, value, currentParams) {
  const params = baseParams(currentParams);
  switch (kind) {
    case "route": {
      // Filter the Routes view to this route's URI — precise enough to
      // isolate one route without needing a dedicated per-route view.
      params.set("filter", value);
      return hash("routes", params);
    }
    case "controller": {
      params.set("filter", value);
      return hash("routes", params);
    }
    case "model": {
      params.set("filter", value);
      return hash("models", params);
    }
    case "table": {
      params.set("table", entityName(value));
      return hash("er", params);
    }
    case "findings":
      return hash("findings", params);
    default:
      return hash("overview", params);
  }
}
