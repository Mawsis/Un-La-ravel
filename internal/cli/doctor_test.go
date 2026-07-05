package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// TestDoctorReportCleanProject verifies the report on a project with no findings
// is the single reassuring line and reports clean == true, so the command exits
// zero.
func TestDoctorReportCleanProject(t *testing.T) {
	pm := model.New("blog", "11.x") // no findings added

	out, clean := doctorReport(pm, "", nil)

	if !clean {
		t.Errorf("doctorReport() clean = false on a project with no findings, want true")
	}
	if !strings.Contains(out, "No issues found") {
		t.Errorf("doctorReport() clean output = %q, want it to mention 'No issues found'", out)
	}
}

// TestDoctorReportItemizesFindings verifies the report lists one line per finding
// using each finding's Label, in order, and reports clean == false so the command
// exits non-zero.
func TestDoctorReportItemizesFindings(t *testing.T) {
	pm := model.New("blog", "11.x").
		AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Count: 2, Label: "2 dead routes", View: "findings"}).
		AddFinding(model.Finding{Kind: model.FindingUnguarded, Count: 1, Label: "1 unguarded model", View: "findings"})

	out, clean := doctorReport(pm, "", nil)

	if clean {
		t.Errorf("doctorReport() clean = true with findings present, want false")
	}
	for _, want := range []string{"2 dead routes", "1 unguarded model"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctorReport() output missing %q\n--- output ---\n%s", want, out)
		}
	}
	// The findings appear in emit order (dead routes before unguarded).
	if i, j := strings.Index(out, "2 dead routes"), strings.Index(out, "1 unguarded model"); i > j {
		t.Errorf("doctorReport() did not preserve finding order; got:\n%s", out)
	}
}

// TestDoctorReportBelowThresholdPasses verifies that with a --fail-on threshold,
// findings strictly below the threshold do not fail the gate (clean == true), yet
// are still itemized, and the report states why the gate passed. Here the only
// findings are warnings and the threshold is blocker, so the gate passes.
func TestDoctorReportBelowThresholdPasses(t *testing.T) {
	pm := model.New("blog", "11.x").
		AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 2, Label: "2 dead routes", View: "findings"})

	out, clean := doctorReport(pm, model.SeverityBlocker, nil)

	if !clean {
		t.Errorf("doctorReport(threshold=blocker) clean = false with only warn findings, want true (gate passes)")
	}
	// Below-threshold findings must still be printed.
	if !strings.Contains(out, "2 dead routes") {
		t.Errorf("doctorReport() dropped a below-threshold finding from the output:\n%s", out)
	}
	// And the report explains why the gate passed despite findings existing.
	if !strings.Contains(out, model.SeverityBlocker) {
		t.Errorf("doctorReport() passing output should name the threshold %q; got:\n%s", model.SeverityBlocker, out)
	}
}

// TestDoctorReportWarnThresholdPassesOnInfoOnly verifies the AC case
// "--fail-on warn passes on info-only": an info finding is below the warn
// threshold, so the gate passes while the finding still prints.
func TestDoctorReportWarnThresholdPassesOnInfoOnly(t *testing.T) {
	pm := model.New("blog", "11.x").
		AddFinding(model.Finding{Kind: "some_info_kind", Severity: model.SeverityInfo, Count: 1, Label: "1 note", View: "findings"})

	out, clean := doctorReport(pm, model.SeverityWarn, nil)

	if !clean {
		t.Errorf("doctorReport(threshold=warn) clean = false with only an info finding, want true (gate passes on info-only)")
	}
	if !strings.Contains(out, "1 note") {
		t.Errorf("doctorReport() dropped the below-threshold info finding:\n%s", out)
	}
}

// TestDoctorReportWarnThresholdFailsOnWarn verifies the AC case "--fail-on warn
// fails on warn": a warn finding is at the threshold, so the gate fails.
func TestDoctorReportWarnThresholdFailsOnWarn(t *testing.T) {
	pm := model.New("blog", "11.x").
		AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 1, Label: "1 dead route", View: "findings"})

	_, clean := doctorReport(pm, model.SeverityWarn, nil)

	if clean {
		t.Errorf("doctorReport(threshold=warn) clean = true with a warn finding, want false (gate fails at threshold)")
	}
}

// TestDoctorReportAtThresholdFails verifies a finding at or above the threshold
// fails the gate (clean == false). A blocker with threshold blocker must fail.
func TestDoctorReportAtThresholdFails(t *testing.T) {
	pm := model.New("blog", "11.x").
		AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 2, Label: "2 dead routes", View: "findings"}).
		AddFinding(model.Finding{Kind: model.FindingUnguarded, Severity: model.SeverityBlocker, Count: 1, Label: "1 unguarded model", View: "findings"})

	_, clean := doctorReport(pm, model.SeverityBlocker, nil)

	if clean {
		t.Errorf("doctorReport(threshold=blocker) clean = true with a blocker finding present, want false (gate fails)")
	}
}

// TestDoctorReportDefaultThresholdFailsOnAny verifies the default (empty
// threshold) preserves today's behavior exactly: any finding fails the gate.
func TestDoctorReportDefaultThresholdFailsOnAny(t *testing.T) {
	pm := model.New("blog", "11.x").
		AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 1, Label: "1 dead route", View: "findings"})

	_, clean := doctorReport(pm, "", nil)

	if clean {
		t.Errorf("doctorReport(threshold=\"\") clean = true with a finding present, want false (default: any finding fails)")
	}
}

// TestDoctorCommand_AnalyzeError_IsReported drives the doctor command against a
// path that is NOT a Laravel project. engine.Analyze fails, so the command must
// exit non-zero — but because doctor sets SilenceErrors (to keep the errFindings
// exit-code sentinel quiet), a genuine analysis error must be printed explicitly
// or the user sees a bare non-zero exit with no explanation. This asserts the
// error reason reaches the command's error stream, distinguishing a real failure
// from the silent findings sentinel.
func TestDoctorCommand_AnalyzeError_IsReported(t *testing.T) {
	// An empty temp dir has no artisan file, so DetectLaravel (and thus
	// engine.Analyze) fails — a genuine analysis error, not a findings verdict.
	notLaravel := t.TempDir()

	cmd := newDoctorCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{notLaravel})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("doctor Execute() returned nil error on a non-Laravel path, want non-nil (non-zero exit)")
	}
	// The user must be told WHY it failed. With SilenceErrors set, the command
	// itself is responsible for surfacing the reason; a bare non-zero exit with
	// no message is the bug this guards against.
	if !strings.Contains(stderr.String(), "artisan") {
		t.Errorf("doctor analysis error was not reported to the error stream; want it to mention the cause (%q), got stderr=%q",
			"artisan", stderr.String())
	}
}

// TestDoctorCommand_InvalidFailOn_IsError drives the doctor command with a
// bogus --fail-on value. A typo'd threshold must not silently pass the gate; it
// must exit non-zero with a message naming the bad value and the valid ones, so a
// misconfigured CI gate fails loudly rather than turning green by accident.
func TestDoctorCommand_InvalidFailOn_IsError(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	cmd := newDoctorCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{fixtureApp, "--fail-on", "catastrophic"})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("doctor --fail-on catastrophic returned nil error, want non-nil (non-zero exit) — an invalid threshold must not silently pass")
	}
	if !strings.Contains(stderr.String(), "catastrophic") {
		t.Errorf("invalid --fail-on error should name the bad value; got stderr=%q", stderr.String())
	}
}

// TestDoctorCommand_FailOnBlocker_FixtureExitsNonZero drives the command with
// --fail-on blocker against the fixture, whose findings include an unguarded model
// (a blocker). The gate must fail (non-zero) and still itemize all findings,
// including the below-threshold warnings.
func TestDoctorCommand_FailOnBlocker_FixtureExitsNonZero(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	cmd := newDoctorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{fixtureApp, "--fail-on", "blocker"})

	err := cmd.Execute()

	if err == nil {
		t.Error("doctor --fail-on blocker returned nil error on a project with a blocker finding, want non-nil (non-zero exit)")
	}
	// Below-threshold warnings still print alongside the blocker.
	for _, want := range []string{"1 dead route", "1 unguarded model"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("doctor --fail-on blocker output missing %q\n--- output ---\n%s", want, out.String())
		}
	}
}

// TestDoctorCommand_FixtureApp_ExitsNonZero drives the actual doctor cobra
// command against the fixture app end-to-end (engine.Analyze → findings →
// report). The fixture has findings, so the command must return a non-nil error
// (which main.main maps to a non-zero exit — the CI gate) and its output must
// itemize the fixture's three findings.
func TestDoctorCommand_FixtureApp_ExitsNonZero(t *testing.T) {
	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	cmd := newDoctorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{fixtureApp})

	err := cmd.Execute()

	if err == nil {
		t.Error("doctor Execute() returned nil error on a project with findings, want non-nil (non-zero exit)")
	}
	for _, want := range []string{"1 dead route", "2 disagreements", "1 unguarded model"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("doctor output missing %q\n--- output ---\n%s", want, out.String())
		}
	}
}
