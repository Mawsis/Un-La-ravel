package baseline

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/findings"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// Cycle 1 — tracer bullet: a dead-route item fingerprints to a non-empty,
// content-derived string. This proves the path Item → Fingerprint end to end.
func TestFingerprintDeadRoute(t *testing.T) {
	item := findings.Item{
		Kind:       model.FindingDeadRoutes,
		Method:     "GET",
		URI:        "/admin/comments",
		Controller: "CommentController",
	}

	got := Fingerprint(item)
	if got == "" {
		t.Fatal("Fingerprint returned empty string for a dead-route item")
	}
}

// Cycle 2 — a fingerprint depends only on a finding's identifying content, so
// two findings that identify the same thing fingerprint identically, and two
// that identify different things (including different kinds) fingerprint
// differently.
func TestFingerprintIdentity(t *testing.T) {
	deadA := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"}
	deadB := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"}
	deadOtherURI := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/comments", Controller: "PostController"}
	unguarded := findings.Item{Kind: model.FindingUnguarded, Class: "Post"}
	disagree := findings.Item{Kind: model.FindingDisagreements, Model: "Post", Relationship: "author"}

	// Same identity → same fingerprint.
	if Fingerprint(deadA) != Fingerprint(deadB) {
		t.Errorf("identical dead routes fingerprinted differently:\n  %q\n  %q", Fingerprint(deadA), Fingerprint(deadB))
	}

	// Different identity → different fingerprint (a changed identifying field).
	if Fingerprint(deadA) == Fingerprint(deadOtherURI) {
		t.Errorf("dead routes with different URIs share a fingerprint: %q", Fingerprint(deadA))
	}

	// Distinct across kinds: no two kinds may collide.
	fps := map[string]string{
		"dead":      Fingerprint(deadA),
		"unguarded": Fingerprint(unguarded),
		"disagree":  Fingerprint(disagree),
	}
	seen := map[string]string{}
	for name, fp := range fps {
		if other, dup := seen[fp]; dup {
			t.Errorf("kinds %s and %s collide on fingerprint %q", name, other, fp)
		}
		seen[fp] = name
	}
}

// Cycle 2b — a finding's fingerprint ignores fields NOT in its kind's identity.
// A dead route carries no relationship; setting one must not change its
// fingerprint. This is what makes fingerprints survive additions to the Item
// struct and guards against accidentally hashing the whole struct.
func TestFingerprintIgnoresIrrelevantFields(t *testing.T) {
	base := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"}
	polluted := base
	polluted.Model = "Whatever"      // not part of a dead route's identity
	polluted.Relationship = "author" // not part of a dead route's identity
	polluted.Class = "SomethingElse" // not part of a dead route's identity

	if Fingerprint(base) != Fingerprint(polluted) {
		t.Errorf("dead-route fingerprint changed when an out-of-kind field was set:\n  %q\n  %q",
			Fingerprint(base), Fingerprint(polluted))
	}
}

// TestFingerprintUnknownKind checks the forward-compatible default: an item of a
// kind not yet in the fingerprint table still fingerprints stably on its Kind
// alone, and stays distinct from a known kind — so a newly-added finding kind is
// baselineable before Fingerprint even learns its fields.
func TestFingerprintUnknownKind(t *testing.T) {
	future := findings.Item{Kind: "future_kind", URI: "/whatever"}
	same := findings.Item{Kind: "future_kind", URI: "/different"} // URI is not in the default identity

	if Fingerprint(future) == "" {
		t.Error("unknown-kind item fingerprinted to empty string")
	}
	if Fingerprint(future) != Fingerprint(same) {
		t.Errorf("unknown kind should fingerprint on Kind alone; %q != %q", Fingerprint(future), Fingerprint(same))
	}
	if Fingerprint(future) == Fingerprint(findings.Item{Kind: model.FindingUnguarded, Class: "future_kind"}) {
		t.Error("unknown kind collided with a known kind")
	}
}

// Cycle 3 — Subtract splits a findings list into survivors (not in the baseline)
// and suppressed (in the baseline), keyed by fingerprint and independent of
// item order.
func TestSubtractSurvivorsAndSuppressed(t *testing.T) {
	dead := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"}
	unguarded := findings.Item{Kind: model.FindingUnguarded, Class: "Post"}
	disagree := findings.Item{Kind: model.FindingDisagreements, Model: "Post", Relationship: "author"}

	// Baseline knows about the dead route and the unguarded model, not the
	// disagreement (a NEW finding).
	base := New([]findings.Item{unguarded, dead})

	survivors, suppressed, stale := Subtract([]findings.Item{dead, unguarded, disagree}, base)

	if len(survivors) != 1 || Fingerprint(survivors[0]) != Fingerprint(disagree) {
		t.Errorf("survivors = %v, want just the disagreement", fps(survivors))
	}
	if len(suppressed) != 2 {
		t.Errorf("suppressed = %v, want the dead route and the unguarded model", fps(suppressed))
	}
	if len(stale) != 0 {
		t.Errorf("stale = %v, want none (every baseline entry matched)", stale)
	}
}

// Cycle 4 — a baseline entry that matches no current finding (the finding was
// fixed) is reported in stale, not silently dropped. And with an empty findings
// list, every baseline entry is stale.
func TestSubtractReportsStale(t *testing.T) {
	dead := findings.Item{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"}
	fixed := findings.Item{Kind: model.FindingUnguarded, Class: "Post"} // in baseline, since fixed

	base := New([]findings.Item{dead, fixed})

	// Only the dead route is still present; the unguarded model was fixed.
	survivors, suppressed, stale := Subtract([]findings.Item{dead}, base)

	if len(survivors) != 0 {
		t.Errorf("survivors = %v, want none (the dead route is baselined)", fps(survivors))
	}
	if len(suppressed) != 1 || Fingerprint(suppressed[0]) != Fingerprint(dead) {
		t.Errorf("suppressed = %v, want just the dead route", fps(suppressed))
	}
	if len(stale) != 1 || stale[0] != Fingerprint(fixed) {
		t.Errorf("stale = %v, want the fixed unguarded model's fingerprint %q", stale, Fingerprint(fixed))
	}
}

// fps is a test helper: the fingerprints of a slice of items, for readable
// failure messages.
func fps(items []findings.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = Fingerprint(it)
	}
	return out
}
