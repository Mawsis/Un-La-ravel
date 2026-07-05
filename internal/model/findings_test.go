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

// TestSeverityAtLeastRanksBlockerOverWarnOverInfo locks the severity ordering
// blocker > warn > info that a CI gate reads (issue #48): SeverityAtLeast reports
// whether sev is at or above threshold. This is the one place the order is
// encoded, so doctor --fail-on can rank without its own table.
func TestSeverityAtLeastRanksBlockerOverWarnOverInfo(t *testing.T) {
	tests := []struct {
		sev       string
		threshold string
		want      bool
	}{
		// A blocker meets any threshold.
		{SeverityBlocker, SeverityBlocker, true},
		{SeverityBlocker, SeverityWarn, true},
		{SeverityBlocker, SeverityInfo, true},
		// A warn meets warn/info but not blocker.
		{SeverityWarn, SeverityBlocker, false},
		{SeverityWarn, SeverityWarn, true},
		{SeverityWarn, SeverityInfo, true},
		// Info meets only the info threshold.
		{SeverityInfo, SeverityBlocker, false},
		{SeverityInfo, SeverityWarn, false},
		{SeverityInfo, SeverityInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.sev+"_vs_"+tt.threshold, func(t *testing.T) {
			if got := SeverityAtLeast(tt.sev, tt.threshold); got != tt.want {
				t.Errorf("SeverityAtLeast(%q, %q) = %v, want %v", tt.sev, tt.threshold, got, tt.want)
			}
		})
	}
}
