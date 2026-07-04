// Central link builder (design.md "Cross-navigation"): every entity mention
// anywhere in the dashboard becomes a link through this one function, so the
// mapping from "a route/model/table/finding" to "the view that explains it"
// is defined exactly once.
//
// hrefFor never touches the DOM or the router directly — it returns a plain
// href string built from the CURRENT router params (so path and any other
// state the destination view needs to stay valid are preserved), letting
// every view module stay a plain template generator.

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
      // The raw table name: the ER renderer owns the SVG and tags each entity
      // group with data-table="<name>" verbatim, so focus is a direct id
      // lookup — no name transform needed to match a rendered label.
      params.set("table", value);
      return hash("er", params);
    }
    case "findings":
      return hash("findings", params);
    default:
      return hash("overview", params);
  }
}
