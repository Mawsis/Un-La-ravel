// Package tablematch reconciles an inferred Eloquent table name against the
// real tables extracted from a project's schema (issue #36). The best-effort
// inflector can over-pluralize a model whose class name is already plural or
// uncountable (WaiterCalls -> waiter_callses, RestaurantStaff ->
// restaurant_staffs), producing an inferred table that does not exist; this
// package answers "which real table did that clearly mean?" so the ER edge
// builder can retarget instead of emitting an orphan edge, and the
// disagreement pass can suggest the fix.
//
// The matching rule is deliberately narrow (precision over coverage, ADR
// 0002): a real table is a candidate only when it differs from the inferred
// name by trailing pluralization — no edit distance, no fuzzy matching — and
// a match is reported only when exactly one candidate exists. This package is
// pure and depends on nothing but the standard library; it is the single
// point of truth for the reconciliation guarantee.
package tablematch

// Reconcile resolves an inferred table name against the real schema tables.
// It reports the one real table that differs from inferred only by trailing
// pluralization, or matched=false when zero or more than one candidate
// exists. An inferred name that is itself a real table passes through
// unchanged.
func Reconcile(inferred string, schemaTables []string) (string, bool) {
	candidates := []string{}
	for _, table := range schemaTables {
		if table == inferred {
			return inferred, true
		}
		if siblings(inferred, table) {
			candidates = append(candidates, table)
		}
	}
	if len(candidates) != 1 {
		return "", false
	}
	return candidates[0], true
}

// siblings reports whether two distinct names differ only by trailing
// pluralization: stripping a regular plural suffix from either one yields the
// other. The relation is symmetric, covering both an over-pluralized inferred
// name ("waiter_callses" ~ "waiter_calls") and an under-pluralized one
// ("lens" ~ "lenses").
func siblings(inferred, table string) bool {
	return contains(singularForms(inferred), table) ||
		contains(singularForms(table), inferred)
}

// singularForms returns the candidate singulars of a name under the regular
// plural suffix rules, mirroring the inflector's forward rules in reverse:
// "-ies" -> "-y", "-es" -> drop "es", "-s" -> drop "s". A name with no plural
// suffix has no singular forms.
func singularForms(name string) []string {
	forms := []string{}
	if n := len(name); n > 3 && name[n-3:] == "ies" {
		forms = append(forms, name[:n-3]+"y")
	}
	if n := len(name); n > 2 && name[n-2:] == "es" {
		forms = append(forms, name[:n-2])
	}
	if n := len(name); n > 1 && name[n-1] == 's' {
		forms = append(forms, name[:n-1])
	}
	return forms
}

// contains reports whether want is among values.
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
