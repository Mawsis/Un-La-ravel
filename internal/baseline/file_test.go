package baseline

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/findings"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// Cycle 5 — the on-disk baseline is JSON with a version field and sorted
// entries, and Marshal is byte-deterministic: the SAME set produces the SAME
// bytes regardless of the order items were added in. This is what makes a
// committed baseline file diff cleanly.
func TestMarshalDeterministicAndSorted(t *testing.T) {
	dead := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"}
	unguarded := findings.Item{Kind: model.FindingUnguarded, Class: "Post"}
	disagree := findings.Item{Kind: model.FindingDisagreements, Model: "Post", Relationship: "author"}

	// Two baselines with the SAME members added in DIFFERENT orders.
	a := New([]findings.Item{dead, unguarded, disagree})
	b := New([]findings.Item{disagree, dead, unguarded})

	bytesA, err := Marshal(a)
	if err != nil {
		t.Fatalf("Marshal(a): %v", err)
	}
	bytesB, err := Marshal(b)
	if err != nil {
		t.Fatalf("Marshal(b): %v", err)
	}

	if !bytes.Equal(bytesA, bytesB) {
		t.Errorf("Marshal is not deterministic across insertion order:\n--- a ---\n%s\n--- b ---\n%s", bytesA, bytesB)
	}

	// The file carries a version field.
	var probe struct {
		Version string   `json:"version"`
		Entries []string `json:"entries"`
	}
	if err := json.Unmarshal(bytesA, &probe); err != nil {
		t.Fatalf("marshaled baseline is not valid JSON: %v", err)
	}
	if probe.Version == "" {
		t.Error("marshaled baseline has no version field")
	}

	// Entries are sorted.
	if len(probe.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(probe.Entries))
	}
	for i := 1; i < len(probe.Entries); i++ {
		if probe.Entries[i-1] > probe.Entries[i] {
			t.Errorf("entries are not sorted: %q before %q", probe.Entries[i-1], probe.Entries[i])
		}
	}
}

// Cycle 5b — Parse is the inverse of Marshal: a marshaled baseline parses back to
// a set that suppresses exactly the same findings.
func TestMarshalParseRoundTrip(t *testing.T) {
	dead := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"}
	unguarded := findings.Item{Kind: model.FindingUnguarded, Class: "Post"}
	orig := New([]findings.Item{dead, unguarded})

	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// A finding in the original baseline is still suppressed by the parsed one;
	// one that was never in it still survives.
	other := findings.Item{Kind: model.FindingDisagreements, Model: "Post", Relationship: "author"}
	survivors, suppressed, stale := Subtract([]findings.Item{dead, unguarded, other}, parsed)
	if len(suppressed) != 2 {
		t.Errorf("round-tripped baseline suppressed %d, want 2", len(suppressed))
	}
	if len(survivors) != 1 || Fingerprint(survivors[0]) != Fingerprint(other) {
		t.Errorf("round-tripped baseline should let the un-baselined disagreement survive; got %v", fps(survivors))
	}
	if len(stale) != 0 {
		t.Errorf("stale = %v, want none", stale)
	}
}

// TestParseVersionCompatibility pins the major-version compatibility policy:
// a same-major version (including a bare major with no minor, and a higher minor)
// is accepted; a different major or an empty version is rejected. Table-driven so
// each case documents one rule.
func TestParseVersionCompatibility(t *testing.T) {
	cases := []struct {
		name    string
		version string
		ok      bool
	}{
		{"current", CurrentVersion, true},
		{"bare major", "1", true},     // "1" shares major "1" with "1.0"
		{"higher minor", "1.9", true}, // within-major, format only adds
		{"next major", "2.0", false},  // incompatible shape
		{"empty", "", false},          // unparseable → rejected, not silently empty
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(`{"version":"` + tc.version + `","entries":[]}`)
			_, err := Parse(data)
			if tc.ok && err != nil {
				t.Errorf("Parse(version=%q) = %v, want ok", tc.version, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("Parse(version=%q) = ok, want error", tc.version)
			}
		})
	}
}

// Cycle 5c — Parse rejects a baseline whose version it does not understand,
// rather than silently treating unknown-format entries as an empty (or partial)
// suppression set — a silently-empty baseline would fail the gate on findings the
// user believed were suppressed.
func TestParseRejectsUnknownVersion(t *testing.T) {
	bad := `{"version":"999.0","entries":[]}`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Error("Parse accepted an unknown version; want an error")
	} else if !strings.Contains(err.Error(), "version") {
		t.Errorf("error %q should mention the version", err)
	}
}
