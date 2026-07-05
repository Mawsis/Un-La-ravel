package tablematch

import "testing"

// TestReconcile_OverPluralizedSiblingMatch is the tracer bullet (issue #36):
// an inferred table name the best-effort inflector over-pluralized
// ("waiter_callses") reconciles to the one real schema table that differs from
// it only by trailing pluralization ("waiter_calls"). This is the exact shape
// of the real-world crash: the edge builder can retarget instead of emitting
// an orphan edge.
func TestReconcile_OverPluralizedSiblingMatch(t *testing.T) {
	schema := []string{"users", "waiter_calls", "orders"}

	real, matched := Reconcile("waiter_callses", schema)

	if !matched {
		t.Fatalf("Reconcile(%q) matched = false, want true", "waiter_callses")
	}
	if real != "waiter_calls" {
		t.Errorf("Reconcile(%q) = %q, want %q", "waiter_callses", real, "waiter_calls")
	}
}

// TestReconcile_Table exercises the reconciliation rule across its behavior
// space: unambiguous singular/plural sibling matching only. Zero candidates
// and more-than-one candidate both decline — the diagram must never show a
// confidently wrong relationship — and no fuzzy matching is ever applied.
func TestReconcile_Table(t *testing.T) {
	cases := []struct {
		name     string
		inferred string
		schema   []string
		want     string
		matched  bool
	}{
		{
			name:     "over-pluralized sibilant plural reconciles to its sibling",
			inferred: "waiter_callses",
			schema:   []string{"users", "waiter_calls"},
			want:     "waiter_calls",
			matched:  true,
		},
		{
			name:     "over-pluralized uncountable reconciles to its sibling",
			inferred: "restaurant_staffs",
			schema:   []string{"restaurant_staff", "orders"},
			want:     "restaurant_staff",
			matched:  true,
		},
		{
			name:     "under-pluralized inferred name reconciles to its plural sibling",
			inferred: "lens",
			schema:   []string{"lenses", "users"},
			want:     "lenses",
			matched:  true,
		},
		{
			name:     "already-correct name passes through",
			inferred: "users",
			schema:   []string{"users", "orders"},
			want:     "users",
			matched:  true,
		},
		{
			name:     "no candidate declines",
			inferred: "waiter_callses",
			schema:   []string{"users", "orders"},
			matched:  false,
		},
		{
			name:     "two candidates decline as ambiguous",
			inferred: "boxes",
			schema:   []string{"box", "boxe"},
			matched:  false,
		},
		{
			name:     "unrelated table is not fuzzy-matched",
			inferred: "waiter_callses",
			schema:   []string{"waiter_call_logs"},
			matched:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			real, matched := Reconcile(tc.inferred, tc.schema)
			if matched != tc.matched {
				t.Fatalf("Reconcile(%q, %v) matched = %v, want %v",
					tc.inferred, tc.schema, matched, tc.matched)
			}
			if real != tc.want {
				t.Errorf("Reconcile(%q, %v) = %q, want %q",
					tc.inferred, tc.schema, real, tc.want)
			}
		})
	}
}
