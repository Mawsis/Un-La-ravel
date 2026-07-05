package findings

import (
	"strings"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// Item is a single, per-item finding: one dead route, one unguarded model, or
// one Model↔Schema disagreement — the granular counterpart to the category
// rollup a model.Finding carries (that rollup has a Count; this is one of the
// things being counted). It exists so the baseline (issue #49) can fingerprint
// and suppress findings ONE AT A TIME — a legacy project baselines the specific
// dead route it already knows about without silencing every future dead route.
//
// Item lives beside Verdict, not in internal/baseline, because "which models
// count as unguarded" is a findings-package rule (countUnguarded); flattening
// here keeps that rule in one place rather than duplicating it in the baseline.
// The identifying fields are a union across kinds: only the subset relevant to
// Kind is populated (a dead route has no Relationship; an unguarded model has
// only Class). Fingerprinting reads exactly the subset its kind defines.
type Item struct {
	// Kind is the finding category, one of the model.Finding* kind constants
	// (FindingDeadRoutes, FindingUnguarded, FindingDisagreements), so a consumer
	// branches on it without parsing anything.
	Kind string
	// Method, URI, Controller identify a dead route (mirroring DeadRoute).
	Method     string
	URI        string
	Controller string
	// Class identifies an unguarded model (the Model.Name).
	Class string
	// Model, Relationship identify a disagreement (mirroring Disagreement).
	Model        string
	Relationship string
}

// Rollup rebuilds category-level findings from a per-item slice: one model.Finding
// per kind, its Count the number of items of that kind and its Label pluralized to
// match, in the kind's first-seen order (which, for the output of Items or a
// subset of it, is the fixed emit order). It is the inverse of Items at the
// category level — the piece a caller needs after removing some items (baseline
// suppression, issue #49) so the surviving verdict reports honest post-removal
// counts, not the pre-removal ones.
//
// Label and severity come from newFinding, the same builder Verdict uses, so a
// rolled-up category is byte-identical to how Verdict would have labeled it at
// that count. An empty input yields a non-nil empty slice.
func Rollup(items []Item) []model.Finding {
	// counts accumulates per-kind totals; order preserves first appearance so the
	// output keeps the emit order without a separate sort.
	counts := make(map[string]int, 3)
	order := make([]string, 0, 3)
	for _, it := range items {
		if _, seen := counts[it.Kind]; !seen {
			order = append(order, it.Kind)
		}
		counts[it.Kind]++
	}

	rollups := make([]model.Finding, 0, len(order))
	for _, kind := range order {
		rollups = append(rollups, newFinding(kind, counts[kind]))
	}
	return rollups
}

// Items flattens pm's health findings into a per-item slice, in the same fixed
// understand-then-judge order Verdict uses (dead routes, then disagreements,
// then unguarded models) so the output is deterministic and golden-testable.
// Each category expands to one Item per underlying entry; a clean project
// yields an empty (non-nil) slice.
//
// Unguarded models are selected by isUnguarded, the SAME predicate Verdict's
// count uses, so Items and Verdict can never disagree about which models are
// unguarded.
func Items(pm *model.ProjectModel) []Item {
	items := make([]Item, 0)

	for _, dr := range pm.DeadRoutes {
		items = append(items, Item{
			Kind:       model.FindingDeadRoutes,
			Method:     dr.Method,
			URI:        dr.URI,
			Controller: dr.Controller,
		})
	}
	for _, d := range pm.Disagreements {
		items = append(items, Item{
			Kind:         model.FindingDisagreements,
			Model:        d.Model,
			Relationship: d.Relationship,
		})
	}
	for _, m := range pm.Models {
		if isUnguarded(m) {
			items = append(items, Item{
				Kind:  model.FindingUnguarded,
				Class: m.Name,
			})
		}
	}
	// Auth findings come last in the understand-then-judge order (issue #50): one
	// item per route reachable without authentication, split by verb so a
	// mutating route is a write finding (blocker) and a non-mutating one is a read
	// finding (warning). Authenticated and unknown routes emit nothing — a finding
	// is a problem, and only definitively-unauthenticated routes are the problem.
	// The route's own source order is preserved, keeping the output deterministic.
	for _, r := range pm.Routes {
		if Classify(r.Middleware) != model.AuthUnauthenticated {
			continue
		}
		items = append(items, Item{
			Kind:   authFindingKind(r.Method),
			Method: r.Method,
			URI:    r.URI,
		})
	}

	return items
}

// authFindingKind maps an unauthenticated route's HTTP verb to its finding kind:
// a mutating verb (POST/PUT/PATCH/DELETE) is an unauthenticated WRITE (blocker),
// anything else (GET/HEAD/OPTIONS) an unauthenticated READ (warning). The split
// is the reason the two kinds exist — the same missing-auth fact is graver on a
// route that changes state than on one that only reads.
func authFindingKind(method string) string {
	if isWriteMethod(method) {
		return model.FindingUnauthenticatedWrite
	}
	return model.FindingUnauthenticatedRead
}

// isWriteMethod reports whether an HTTP verb mutates state. The set is the four
// mutating verbs Laravel routes use; matching is case-insensitive so a route
// recorded as "post" classifies like "POST", though the extractor uppercases.
func isWriteMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}
