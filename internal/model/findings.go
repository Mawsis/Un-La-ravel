package model

// Finding kinds. These are the stable machine-readable values written to
// Finding.Kind so consumers (the CLI's doctor command, the dashboard verdict
// reader, any CI gate reading unlaravel.json) can branch without parsing the
// human-readable Label. Defined once here; never hardcode the literal elsewhere.
//
// The set — and the ORDER the verdict emits them in — mirrors the browser's
// original health-verdict categories (issue #27): dead routes, then model↔schema
// disagreements, then unguarded models. Server and browser agree by construction
// because the browser now READS these findings rather than recomputing them.
const (
	// FindingDeadRoutes marks the "N dead route(s)" category — routes whose
	// Controller/Action edge dangles (ProjectModel.DeadRoutes).
	FindingDeadRoutes = "dead_routes"
	// FindingDisagreements marks the "N disagreement(s)" category — Eloquent
	// relationships that reference something the Schema lacks
	// (ProjectModel.Disagreements).
	FindingDisagreements = "disagreements"
	// FindingUnguarded marks the "N unguarded model(s)" category — Models that
	// explicitly wrote `protected $guarded = []`, Laravel's "everything is
	// mass-assignable" escape hatch (a non-nil, empty Model.Guarded).
	FindingUnguarded = "unguarded"
)

// Finding is one itemized entry of the project's health verdict: a single
// problem category with a nonzero count. It is the server-computed,
// contract-serialized form of the browser's original client-side verdict
// (issue #27) — lifting the verdict into the Project Model so the CLI, the
// unlaravel.json contract, and the dashboard all read ONE source of truth
// instead of each re-deriving it.
//
// A Finding is emitted only for a category whose count is greater than zero;
// a clean project has an empty findings array. See internal/findings for the
// computation.
type Finding struct {
	// Kind is the machine-readable category, one of FindingDeadRoutes,
	// FindingDisagreements, or FindingUnguarded, so consumers branch without
	// parsing Label.
	Kind string `json:"kind"`
	// Count is the number of distinct static findings in this category (always
	// greater than zero — zero-count categories are omitted).
	Count int `json:"count"`
	// Label is the human-readable, pluralized summary, for example
	// "2 dead routes" or "1 unguarded model".
	Label string `json:"label"`
	// View is the dashboard view a consumer should link to for the detail — the
	// findings view — so the itemized verdict stays clickable.
	View string `json:"view"`
}
