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

// Finding severities. These are the stable machine-readable values written to
// Finding.Severity, ordered blocker > warn > info from most to least severe so
// a consumer can rank findings without a lookup table. They let the dashboard
// group findings by severity and a CI gate fail only on blockers, without
// re-deriving severity from Kind.
const (
	// SeverityBlocker is a problem a consumer should treat as must-fix — an
	// unguarded model exposes every column to mass assignment.
	SeverityBlocker = "blocker"
	// SeverityWarn is a problem worth surfacing but not a hard stop — a dead
	// route or a Model↔Schema disagreement.
	SeverityWarn = "warn"
	// SeverityInfo is the least-severe level and the forward-compatible default
	// for any kind not yet in the severity table.
	SeverityInfo = "info"
)

// severityByKind is the ONE table mapping a Finding.Kind to its severity
// (issue #47). Per the roadmap: an unguarded model is a blocker (it silences
// Laravel's mass-assignment guard); a dead route or a Model↔Schema disagreement
// is a warning. It lives here, beside the kinds, so severity is never a literal
// at a call site — Verdict and any future producer stamp severity through
// SeverityFor, and adding a kind is a one-line edit in one place.
var severityByKind = map[string]string{
	FindingUnguarded:     SeverityBlocker,
	FindingDeadRoutes:    SeverityWarn,
	FindingDisagreements: SeverityWarn,
}

// SeverityFor returns the severity for a finding kind from the single
// kind→severity table, defaulting to SeverityInfo for an unknown kind so a
// newly added kind degrades to the least-alarming level rather than a scary
// one. This is the only sanctioned way to derive a finding's severity.
func SeverityFor(kind string) string {
	if sev, ok := severityByKind[kind]; ok {
		return sev
	}
	return SeverityInfo
}

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
	// Severity is the machine-readable level, one of SeverityBlocker,
	// SeverityWarn, or SeverityInfo, derived from Kind via the single
	// SeverityFor table. It lets the dashboard group findings by severity and a
	// CI gate rank them without re-deriving severity from Kind.
	Severity string `json:"severity"`
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
