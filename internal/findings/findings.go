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
// disagreements, then unguarded models. A category with a zero count is omitted.
//
// The result is a non-nil slice (empty for a clean project) so it serializes as
// "findings": [] rather than null, matching the model's non-nil-empty-slice
// convention.
//
// The three category counts:
//   - dead routes   — len(pm.DeadRoutes): routes whose Controller/Action edge dangles.
//   - disagreements — len(pm.Disagreements): relationships referencing something
//     the Schema lacks.
//   - unguarded     — Models with a non-nil, EMPTY Guarded (`protected $guarded = []`,
//     Laravel's "everything is mass-assignable" escape hatch). A nil Guarded
//     (property omitted) is guarded-by-omission and is deliberately NOT counted —
//     the load-bearing nil-vs-empty distinction the Model documents.
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

	return findings
}

// nounByKind maps a finding kind to the singular noun its Label pluralizes ("2
// dead routes", "1 disagreement"). It lives beside Verdict as the single source
// of a category's human noun, so Verdict and Rollup (which rebuilds rollups from
// survivor items) always label a category identically.
var nounByKind = map[string]string{
	model.FindingDeadRoutes:    "dead route",
	model.FindingDisagreements: "disagreement",
	model.FindingUnguarded:     "unguarded model",
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
// in one place. View is always the findings view so the itemized entry stays
// clickable in the dashboard. A kind with no registered noun falls back to the
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
		View:     "findings",
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
