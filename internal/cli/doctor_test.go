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

	out, clean := doctorReport(pm)

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

	out, clean := doctorReport(pm)

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
