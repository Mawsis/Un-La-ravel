package findings

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// deadRoute is a compact helper: a DeadRoute's content is irrelevant to the
// verdict, which counts them, so tests only need N of them.
func deadRoute() model.DeadRoute { return model.DeadRoute{Method: "GET", URI: "/x"} }

// disagreement is the disagreement analogue of deadRoute — content-irrelevant,
// counted.
func disagreement() model.Disagreement { return model.Disagreement{Model: "M", Relationship: "r"} }

// unguardedModel returns a Model that explicitly wrote `protected $guarded = []`
// — a non-nil, empty Guarded, the escape hatch the verdict flags.
func unguardedModel(name string) model.Model {
	m := model.NewModel(name)
	m.Guarded = []string{}
	return m
}

// guardedByOmissionModel returns a Model that declared NEITHER $fillable nor
// $guarded — a nil Guarded, which is guarded-by-omission and NOT a finding.
func guardedByOmissionModel(name string) model.Model {
	return model.NewModel(name) // NewModel leaves Guarded at nil.
}

// TestVerdictCleanProjectHasNoFindings verifies a project with no dead routes,
// no disagreements, and no unguarded models yields an empty (non-nil) verdict.
func TestVerdictCleanProjectHasNoFindings(t *testing.T) {
	pm := model.New("blog", "11.x").AddModel(guardedByOmissionModel("Post"))

	got := Verdict(pm)

	if got == nil {
		t.Fatal("Verdict() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Verdict() on a clean project returned %d findings, want 0: %+v", len(got), got)
	}
}

// TestVerdictItemizesEachCategory verifies each category is counted independently
// and labeled with the correct pluralization, and that a category with a zero
// count is omitted.
func TestVerdictItemizesEachCategory(t *testing.T) {
	tests := []struct {
		name  string
		build func() *model.ProjectModel
		want  []model.Finding
	}{
		{
			name: "one dead route",
			build: func() *model.ProjectModel {
				return model.New("a", "11.x").AddDeadRoute(deadRoute())
			},
			want: []model.Finding{
				{Kind: model.FindingDeadRoutes, Count: 1, Label: "1 dead route", View: "findings"},
			},
		},
		{
			name: "two dead routes pluralize",
			build: func() *model.ProjectModel {
				return model.New("a", "11.x").AddDeadRoute(deadRoute()).AddDeadRoute(deadRoute())
			},
			want: []model.Finding{
				{Kind: model.FindingDeadRoutes, Count: 2, Label: "2 dead routes", View: "findings"},
			},
		},
		{
			name: "one disagreement",
			build: func() *model.ProjectModel {
				return model.New("a", "11.x").AddDisagreement(disagreement())
			},
			want: []model.Finding{
				{Kind: model.FindingDisagreements, Count: 1, Label: "1 disagreement", View: "findings"},
			},
		},
		{
			name: "three disagreements pluralize",
			build: func() *model.ProjectModel {
				return model.New("a", "11.x").
					AddDisagreement(disagreement()).
					AddDisagreement(disagreement()).
					AddDisagreement(disagreement())
			},
			want: []model.Finding{
				{Kind: model.FindingDisagreements, Count: 3, Label: "3 disagreements", View: "findings"},
			},
		},
		{
			name: "one unguarded model",
			build: func() *model.ProjectModel {
				return model.New("a", "11.x").AddModel(unguardedModel("Post"))
			},
			want: []model.Finding{
				{Kind: model.FindingUnguarded, Count: 1, Label: "1 unguarded model", View: "findings"},
			},
		},
		{
			name: "two unguarded models pluralize",
			build: func() *model.ProjectModel {
				return model.New("a", "11.x").
					AddModel(unguardedModel("Post")).
					AddModel(unguardedModel("User"))
			},
			want: []model.Finding{
				{Kind: model.FindingUnguarded, Count: 2, Label: "2 unguarded models", View: "findings"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Verdict(tt.build())
			assertFindings(t, got, tt.want)
		})
	}
}

// TestVerdictUnguardedIgnoresGuardedByOmission verifies the unguarded category
// keys on a non-nil empty Guarded (`$guarded = []`) and does NOT count a model
// with a nil Guarded (property omitted), which Laravel guards by omission. This
// locks the load-bearing nil-vs-empty distinction the Model documents.
func TestVerdictUnguardedIgnoresGuardedByOmission(t *testing.T) {
	pm := model.New("a", "11.x").
		AddModel(unguardedModel("Risky")).       // $guarded = [] -> counted
		AddModel(guardedByOmissionModel("Safe")) // no $guarded  -> NOT counted

	got := Verdict(pm)

	want := []model.Finding{
		{Kind: model.FindingUnguarded, Count: 1, Label: "1 unguarded model", View: "findings"},
	}
	assertFindings(t, got, want)
}

// TestVerdictPreservesFixedCategoryOrder verifies that when all three categories
// fire, they are emitted in the fixed understand-then-judge order: dead routes,
// then disagreements, then unguarded models — the order the dashboard verdict has
// always shown.
func TestVerdictPreservesFixedCategoryOrder(t *testing.T) {
	pm := model.New("a", "11.x").
		AddModel(unguardedModel("Post")).
		AddDisagreement(disagreement()).
		AddDeadRoute(deadRoute())

	got := Verdict(pm)

	want := []model.Finding{
		{Kind: model.FindingDeadRoutes, Count: 1, Label: "1 dead route", View: "findings"},
		{Kind: model.FindingDisagreements, Count: 1, Label: "1 disagreement", View: "findings"},
		{Kind: model.FindingUnguarded, Count: 1, Label: "1 unguarded model", View: "findings"},
	}
	assertFindings(t, got, want)
}

// assertFindings compares two finding slices element-by-element and reports the
// first mismatch, so a diff points at the exact field that drifted.
func assertFindings(t *testing.T, got, want []model.Finding) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Verdict() returned %d findings, want %d\n got: %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
