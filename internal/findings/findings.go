// Package findings computes the project's itemized health verdict from an
// already-assembled Project Model (ADR 0001): a small, ordered list of problem
// categories with nonzero counts. It is the server-side home of the verdict
// that issue #27 first shipped in the browser — lifting the logic out of
// JavaScript and into the contract so the CLI's `doctor` command, the
// unlaravel.json output, and the dashboard all read ONE source of truth.
//
// Like internal/analyze, it lives apart from the extractors because it spans
// several node types (routes, models, disagreements) that no single extractor
// owns. It touches no AST and does no I/O: Verdict is a pure, order-deterministic
// function over the model, suitable for golden tests.
//
// It imports only internal/model, so nothing depends on it that model cannot —
// no import cycle. The engine calls Verdict once, last, after every other node
// is assembled (the verdict reads DeadRoutes, Disagreements, and Models).
package findings

import (
	"fmt"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// Verdict computes the itemized health verdict for pm: one Finding per problem
// category that has a nonzero count, in the fixed understand-then-judge order the
// dashboard established (issue #27) — dead routes, then model↔schema
// disagreements, then unguarded models, then the two auth categories (issue #50).
// A category with a zero count is omitted.
//
// The result is a non-nil slice (empty for a clean project) so it serializes as
// "findings": [] rather than null, matching the model's non-nil-empty-slice
// convention.
//
// The category counts:
//   - dead routes   — len(pm.DeadRoutes): routes whose Controller/Action edge dangles.
//   - disagreements — len(pm.Disagreements): relationships referencing something
//     the Schema lacks.
//   - unguarded     — Models with a non-nil, EMPTY Guarded (`protected $guarded = []`,
//     Laravel's "everything is mass-assignable" escape hatch). A nil Guarded
//     (property omitted) is guarded-by-omission and is deliberately NOT counted —
//     the load-bearing nil-vs-empty distinction the Model documents.
//   - unauthenticated writes — mutating routes (POST/PUT/PATCH/DELETE) whose
//     flattened middleware does not authenticate (Classify → unauthenticated).
//   - unauthenticated reads  — the same for non-mutating verbs. Routes whose auth
//     is authenticated or unknown are NOT counted — precision over coverage.
func Verdict(pm *model.ProjectModel) []model.Finding {
	findings := make([]model.Finding, 0, 3)

	if n := len(pm.DeadRoutes); n > 0 {
		findings = append(findings, newFinding(model.FindingDeadRoutes, n))
	}
	if n := len(pm.Disagreements); n > 0 {
		findings = append(findings, newFinding(model.FindingDisagreements, n))
	}
	if n := countUnguarded(pm.Models); n > 0 {
		findings = append(findings, newFinding(model.FindingUnguarded, n))
	}
	// Auth findings come last (issue #50), writes before reads so the graver
	// blocker category leads. Counts are derived from the SAME per-route
	// classification Items uses (countAuth), so the rolled-up category counts here
	// can never disagree with the per-item flattening.
	writes, reads := countAuth(pm.Routes)
	if writes > 0 {
		findings = append(findings, newFinding(model.FindingUnauthenticatedWrite, writes))
	}
	if reads > 0 {
		findings = append(findings, newFinding(model.FindingUnauthenticatedRead, reads))
	}

	return findings
}

// countAuth returns how many routes are unauthenticated writes and how many are
// unauthenticated reads, classifying each route's flattened middleware with
// Classify and splitting the unauthenticated ones by verb (isWriteMethod). It is
// the ONE place the auth counts are derived, shared by Verdict's category rollup;
// Items independently emits one entry per unauthenticated route through the same
// two predicates, so the two views agree by construction. Authenticated and
// unknown routes contribute to neither count — only definitively-unauthenticated
// routes are findings.
func countAuth(routes []model.Route) (writes, reads int) {
	for _, r := range routes {
		if Classify(r.Middleware) != model.AuthUnauthenticated {
			continue
		}
		if isWriteMethod(r.Method) {
			writes++
		} else {
			reads++
		}
	}
	return writes, reads
}

// nounByKind maps a finding kind to the singular noun its Label pluralizes ("2
// dead routes", "1 disagreement"). It lives beside Verdict as the single source
// of a category's human noun, so Verdict and Rollup (which rebuilds rollups from
// survivor items) always label a category identically.
var nounByKind = map[string]string{
	model.FindingDeadRoutes:           "dead route",
	model.FindingDisagreements:        "disagreement",
	model.FindingUnguarded:            "unguarded model",
	model.FindingUnauthenticatedWrite: "unauthenticated write route",
	model.FindingUnauthenticatedRead:  "unauthenticated read route",
}

// viewByKind maps a finding kind to the dashboard view its detail lives in, so a
// clicked verdict entry lands where the reader can act on it. The original three
// kinds share the findings view; the auth kinds (issue #50) have their own Auth
// view under HEALTH, so they link there instead. A kind absent from the map
// falls back to the findings view (viewFor), keeping the itemized entry clickable
// for any future kind added before it earns a dedicated view.
var viewByKind = map[string]string{
	model.FindingUnauthenticatedWrite: "auth",
	model.FindingUnauthenticatedRead:  "auth",
}

// viewFor returns the dashboard view a finding kind links to, defaulting to the
// findings view for any kind without a dedicated one.
func viewFor(kind string) string {
	if v, ok := viewByKind[kind]; ok {
		return v
	}
	return "findings"
}

// isUnguarded reports whether m explicitly opted into mass-assignment via
// `protected $guarded = []` — a NON-NIL, EMPTY Guarded. A nil Guarded is
// guarded-by-omission (the property was never written) and is NOT unguarded.
// This load-bearing nil-vs-empty distinction is documented on model.Model; it
// lives here as the ONE predicate so Verdict's count and Items' flattening can
// never disagree about which models are unguarded.
func isUnguarded(m model.Model) bool {
	return m.Guarded != nil && len(m.Guarded) == 0
}

// countUnguarded returns the number of models that are unguarded per isUnguarded.
func countUnguarded(models []model.Model) int {
	count := 0
	for _, m := range models {
		if isUnguarded(m) {
			count++
		}
	}
	return count
}

// newFinding builds a Finding for a category, pluralizing the category's noun
// (from nounByKind) on count (so "1 dead route" but "2 dead routes"), matching
// the browser verdict's labels exactly. Severity is derived from the kind through
// the single model.SeverityFor table — never a literal here, so the mapping lives
// in one place. View is the kind's dashboard view (viewFor) so the itemized entry
// links to where its detail lives — the findings view for most kinds, the Auth
// view for the auth kinds. A kind with no registered noun falls back to the
// kind string itself, so an unlabeled category still renders something.
func newFinding(kind string, count int) model.Finding {
	noun, ok := nounByKind[kind]
	if !ok {
		noun = kind
	}
	return model.Finding{
		Kind:     kind,
		Severity: model.SeverityFor(kind),
		Count:    count,
		Label:    fmt.Sprintf("%d %s", count, pluralize(noun, count)),
		View:     viewFor(kind),
	}
}

// pluralize appends an "s" to noun unless count is exactly 1. The nouns here are
// all regular plurals, so a simple suffix suffices and keeps the labels matching
// the browser's original pluralization.
func pluralize(noun string, count int) string {
	if count == 1 {
		return noun
	}
	return noun + "s"
}
