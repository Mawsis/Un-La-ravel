package findings

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// TestItemsFlattensInVerdictOrder checks that Items expands each finding
// category to one Item per underlying entry, in the same fixed
// dead-routes → disagreements → unguarded order Verdict emits, and that the
// unguarded selection obeys the load-bearing nil-vs-empty Guarded rule.
func TestItemsFlattensInVerdictOrder(t *testing.T) {
	pm := model.New("test", "10.x")
	pm.DeadRoutes = []model.DeadRoute{
		{Method: "GET", URI: "/posts", Controller: "PostController", Action: "index"},
		{Method: "POST", URI: "/comments", Controller: "CommentController", Action: "store"},
	}
	pm.Disagreements = []model.Disagreement{
		{Model: "Post", Relationship: "author"},
	}
	pm.Models = []model.Model{
		{Name: "Guarded", Guarded: []string{"id"}}, // has a guard list → NOT unguarded
		{Name: "Omitted"},                          // nil Guarded → guarded-by-omission → NOT counted
		{Name: "WideOpen", Guarded: []string{}},    // non-nil empty → explicitly unguarded → counted
	}

	got := Items(pm)

	want := []Item{
		{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/posts", Controller: "PostController"},
		{Kind: model.FindingDeadRoutes, Method: "POST", URI: "/comments", Controller: "CommentController"},
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "author"},
		{Kind: model.FindingUnguarded, Class: "WideOpen"},
	}

	if len(got) != len(want) {
		t.Fatalf("Items returned %d items, want %d:\n got=%+v\nwant=%+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestRollupItemsRecountsPerCategory checks that Rollup rebuilds category
// findings from a per-item slice with counts and labels derived from THOSE items,
// in the fixed emit order, so a caller that removed some items (e.g. baseline
// suppression) gets an honest post-removal count rather than a stale one.
func TestRollupItemsRecountsPerCategory(t *testing.T) {
	items := []Item{
		{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/a", Controller: "A"},
		{Kind: model.FindingDeadRoutes, Method: "GET", URI: "/b", Controller: "B"},
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "lens"}, // one survivor
		{Kind: model.FindingUnguarded, Class: "Category"},
	}

	got := Rollup(items)

	want := []model.Finding{
		{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 2, Label: "2 dead routes", View: "findings"},
		{Kind: model.FindingDisagreements, Severity: model.SeverityWarn, Count: 1, Label: "1 disagreement", View: "findings"},
		{Kind: model.FindingUnguarded, Severity: model.SeverityBlocker, Count: 1, Label: "1 unguarded model", View: "findings"},
	}

	if len(got) != len(want) {
		t.Fatalf("Rollup returned %d categories, want %d:\n got=%+v\nwant=%+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rollup %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestRollupItemsEmpty checks that an empty item slice yields an empty (non-nil)
// rollup — the clean case a fully-suppressed baseline produces.
func TestRollupItemsEmpty(t *testing.T) {
	got := Rollup(nil)
	if got == nil {
		t.Fatal("Rollup(nil) = nil, want a non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Rollup(nil) len = %d, want 0", len(got))
	}
}

// TestItemsCleanProjectIsEmptyNonNil checks that a clean project yields a
// non-nil, empty slice — mirroring the model's non-nil-empty-slice convention so
// callers can range without a nil check.
func TestItemsCleanProjectIsEmptyNonNil(t *testing.T) {
	got := Items(model.New("test", "10.x"))
	if got == nil {
		t.Fatal("Items returned nil for a clean project; want a non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Items returned %d items for a clean project, want 0", len(got))
	}
}
