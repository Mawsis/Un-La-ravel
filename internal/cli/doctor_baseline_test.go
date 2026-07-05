package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/baseline"
	"github.com/Mawsis/Un-La-ravel/internal/findings"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// writeBaselineFile marshals a baseline from the given items plus one guaranteed
// stale fingerprint (a finding that does not exist in the fixture), writes it to
// a temp file, and returns the path. The stale entry lets the same fixture prove
// both suppression and stale-reporting in one run.
func writeBaselineFile(t *testing.T, items []findings.Item) string {
	t.Helper()
	// A phantom finding that the fixture never produces → guaranteed stale.
	stale := findings.Item{Kind: model.FindingUnguarded, Class: "PhantomModelThatDoesNotExist"}
	b := baseline.New(append(items, stale))
	data, err := baseline.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal baseline: %v", err)
	}
	path := filepath.Join(t.TempDir(), "unlaravel-baseline.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write baseline file: %v", err)
	}
	return path
}

// TestDoctorCommand_Baseline_SuppressesFromExitDecision drives the doctor
// command with a baseline that suppresses the fixture's two disagreements and its
// unguarded model, leaving only the dead route. The gate must STILL fail (the
// dead route survives), and the suppressed findings must be reported (the debt
// stays visible) rather than silently hidden.
func TestDoctorCommand_Baseline_SuppressesFromExitDecision(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	// Baseline the fixture's disagreements and unguarded model, but NOT the dead
	// route — so a survivor remains and the gate should fail on it.
	baselinePath := writeBaselineFile(t, []findings.Item{
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "editor"},
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "lens"},
		{Kind: model.FindingUnguarded, Class: "Category"},
	})

	cmd := newDoctorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{fixtureApp, "--baseline", baselinePath})

	err := cmd.Execute()

	if err == nil {
		t.Error("doctor --baseline returned nil error while an un-baselined dead route survives, want non-nil (gate fails on the survivor)")
	}
	// The surviving finding drives the gate and must appear.
	if !strings.Contains(out.String(), "dead route") {
		t.Errorf("doctor --baseline output missing the surviving dead route\n--- output ---\n%s", out.String())
	}
	// Suppressed findings are reported, not silently hidden.
	if !strings.Contains(strings.ToLower(out.String()), "suppress") {
		t.Errorf("doctor --baseline output should report suppressed findings\n--- output ---\n%s", out.String())
	}
}

// TestDoctorCommand_Baseline_SurvivorCountReflectsSuppression baselines exactly
// ONE of the fixture's two disagreements. The surviving report must say "1
// disagreement", not "2 disagreements": the count is over what SURVIVED, since a
// user reading "2 disagreements" above a "suppressed: 1" line would be told the
// project has three when it has two. Guards against reusing the pre-suppression
// rollup count.
func TestDoctorCommand_Baseline_SurvivorCountReflectsSuppression(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	// Suppress only Post.editor, leaving Post.lens as the single surviving
	// disagreement.
	baselinePath := writeBaselineFile(t, []findings.Item{
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "editor"},
	})

	cmd := newDoctorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{fixtureApp, "--baseline", baselinePath})

	_ = cmd.Execute()

	if strings.Contains(out.String(), "2 disagreement") {
		t.Errorf("survivor report shows the pre-suppression count '2 disagreements' after one was baselined\n--- output ---\n%s", out.String())
	}
	if !strings.Contains(out.String(), "1 disagreement") {
		t.Errorf("survivor report should show '1 disagreement' (the one survivor)\n--- output ---\n%s", out.String())
	}
}

// TestDoctorCommand_Baseline_AllSuppressedExitsZero baselines EVERY current
// fixture finding, so no survivor remains and the gate passes (exit zero) — the
// legacy-adoption workflow: a project with known findings adopts the gate and it
// goes green, gating only on NEW findings thereafter.
func TestDoctorCommand_Baseline_AllSuppressedExitsZero(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	baselinePath := writeBaselineFile(t, []findings.Item{
		{Kind: model.FindingDeadRoutes, Method: "DELETE", URI: "/admin/users/{id}", Controller: "UserController"},
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "editor"},
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "lens"},
		{Kind: model.FindingUnguarded, Class: "Category"},
	})

	cmd := newDoctorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{fixtureApp, "--baseline", baselinePath})

	err := cmd.Execute()

	if err != nil {
		t.Errorf("doctor --baseline (all findings baselined) returned %v, want nil (gate passes when every finding is suppressed)", err)
	}
}

// TestDoctorCommand_Baseline_StaleReported checks that a baseline entry matching
// no current finding (the phantom writeBaselineFile always adds) is reported as
// stale, not silently dropped — so a fixed-then-forgotten baseline entry stays
// visible for pruning.
func TestDoctorCommand_Baseline_StaleReported(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	baselinePath := writeBaselineFile(t, []findings.Item{
		{Kind: model.FindingDeadRoutes, Method: "DELETE", URI: "/admin/users/{id}", Controller: "UserController"},
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "editor"},
		{Kind: model.FindingDisagreements, Model: "Post", Relationship: "lens"},
		{Kind: model.FindingUnguarded, Class: "Category"},
	})

	cmd := newDoctorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{fixtureApp, "--baseline", baselinePath})

	_ = cmd.Execute()

	if !strings.Contains(strings.ToLower(out.String()), "stale") {
		t.Errorf("doctor --baseline output should report the stale phantom entry\n--- output ---\n%s", out.String())
	}
}

// TestDoctorCommand_Baseline_MissingFile_IsError checks that pointing --baseline
// at a nonexistent file fails loudly (non-zero, with a reason on stderr) rather
// than silently proceeding with no suppression — a silently-ignored baseline
// would fail CI on findings the user believed were suppressed.
func TestDoctorCommand_Baseline_MissingFile_IsError(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	cmd := newDoctorCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{fixtureApp, "--baseline", filepath.Join(t.TempDir(), "does-not-exist.json")})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("doctor --baseline <missing> returned nil error, want non-nil — a missing baseline must fail loudly, not silently disable suppression")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "baseline") {
		t.Errorf("missing-baseline error should mention the baseline; got stderr=%q", stderr.String())
	}
}
