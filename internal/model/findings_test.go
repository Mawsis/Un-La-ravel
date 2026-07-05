package model

import "testing"

// TestSeverityForMapsEachKind locks the single kind→severity table (issue #47):
// unguarded models are blockers; dead routes and disagreements are warnings.
// An unknown kind degrades to info rather than crashing or defaulting to a
// scarier level — a forward-compatible, non-alarming fallback.
func TestSeverityForMapsEachKind(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{FindingUnguarded, SeverityBlocker},
		{FindingDeadRoutes, SeverityWarn},
		{FindingDisagreements, SeverityWarn},
		{"some_future_kind", SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := SeverityFor(tt.kind); got != tt.want {
				t.Errorf("SeverityFor(%q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}
