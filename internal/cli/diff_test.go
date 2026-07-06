package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// writeModelFile serializes a model to a temp unlaravel.json and returns its
// path, so a diff test can drive the command over real files the way a user does.
func writeModelFile(t *testing.T, pm *model.ProjectModel, name string) string {
	t.Helper()
	data, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("serialize %s: %v", name, err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// execDiff drives the diff subcommand in isolation over two files, capturing
// stdout, and returns the output and the command's error (nil == zero exit).
func execDiff(t *testing.T, oldPath, newPath string) (string, error) {
	t.Helper()
	cmd := newDiffCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{oldPath, newPath})
	err := cmd.Execute()
	return out.String(), err
}

// TestDiffCleanPairExitsZero: diffing a model against an added-warning variant
// (no new blocker) prints the report and exits zero.
func TestDiffCleanPairExitsZero(t *testing.T) {
	old := model.New("blog", "11.x")
	new := model.New("blog", "11.x")
	new.AddRoute(model.Route{Method: "GET", URI: "/about", Controller: "PageController", Action: "about", Auth: model.AuthUnauthenticated})

	out, err := execDiff(t, writeModelFile(t, old, "old.json"), writeModelFile(t, new, "new.json"))
	if err != nil {
		t.Fatalf("diff of a no-new-blocker pair returned error %v, want nil (zero exit)\n%s", err, out)
	}
	if !strings.Contains(out, "/about") {
		t.Errorf("diff output did not mention the added route:\n%s", out)
	}
}

// TestDiffNewBlockerExitsNonZero: diffing a clean model against one that adds a
// blocker finding prints the report AND returns a non-nil error, so the process
// exits non-zero — the CI gate a PR that adds an unauthenticated write route trips.
func TestDiffNewBlockerExitsNonZero(t *testing.T) {
	old := model.New("blog", "11.x")
	new := model.New("blog", "11.x")
	new.AddFinding(model.Finding{
		Kind: model.FindingUnauthenticatedWrite, Severity: model.SeverityBlocker,
		Count: 1, Label: "1 unauthenticated write route", View: "auth",
	})

	out, err := execDiff(t, writeModelFile(t, old, "old.json"), writeModelFile(t, new, "new.json"))
	if err == nil {
		t.Fatalf("diff introducing a blocker returned nil error, want non-nil (non-zero exit)\n%s", out)
	}
	if !strings.Contains(out, "unauthenticated write") {
		t.Errorf("diff output did not itemize the new blocker finding:\n%s", out)
	}
}

// TestDiffMajorVersionMismatchErrors: two models whose schema versions differ in
// major version are refused with a clear error naming both versions, rather than
// silently compared across a shape change.
func TestDiffMajorVersionMismatchErrors(t *testing.T) {
	old := model.New("blog", "11.x")
	old.SchemaVersion = "1.9.0"
	new := model.New("blog", "11.x")
	new.SchemaVersion = "2.0.0"

	out, err := execDiff(t, writeModelFile(t, old, "old.json"), writeModelFile(t, new, "new.json"))
	if err == nil {
		t.Fatalf("diff across a major-version mismatch returned nil error, want non-nil\n%s", out)
	}
	if !strings.Contains(out, "1.9.0") || !strings.Contains(out, "2.0.0") {
		t.Errorf("mismatch error did not name both versions:\n%s", out)
	}
}

// TestDiffMissingFileErrors: an unreadable input path fails fast with a clear
// error naming the file, not a panic or a bare non-zero exit.
func TestDiffMissingFileErrors(t *testing.T) {
	old := model.New("blog", "11.x")
	oldPath := writeModelFile(t, old, "old.json")
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")

	out, err := execDiff(t, oldPath, missing)
	if err == nil {
		t.Fatalf("diff over a missing new file returned nil error, want non-nil\n%s", out)
	}
	if !strings.Contains(out, "does-not-exist.json") {
		t.Errorf("missing-file error did not name the file:\n%s", out)
	}
}

// TestDiffHumanSummaryAllSections exercises the human summary across every
// section: an added route, a removed table, a changed column, an added model,
// and a changed finding all appear in the printed output.
func TestDiffHumanSummaryAllSections(t *testing.T) {
	old := model.New("blog", "11.x")
	postsOld := model.NewTable("posts")
	postsOld.Columns = append(postsOld.Columns, model.Column{Name: "votes", Type: "integer"})
	old.AddTable(postsOld)
	old.AddTable(model.NewTable("legacy"))
	old.AddModel(model.NewModel("Post"))
	old.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 1, Label: "1 dead route", View: "findings"})

	new := model.New("blog", "11.x")
	postsNew := model.NewTable("posts")
	postsNew.Columns = append(postsNew.Columns, model.Column{Name: "votes", Type: "bigInteger"})
	new.AddTable(postsNew)
	new.AddModel(model.NewModel("Post"))
	new.AddModel(model.NewModel("Comment"))
	new.AddRoute(model.Route{Method: "POST", URI: "/posts", Controller: "PostController", Action: "store", Auth: model.AuthAuthenticated})
	new.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 2, Label: "2 dead routes", View: "findings"})

	out, err := execDiff(t, writeModelFile(t, old, "old.json"), writeModelFile(t, new, "new.json"))
	if err != nil {
		t.Fatalf("mixed diff (no new blocker) returned error %v, want nil\n%s", err, out)
	}
	for _, want := range []string{"Routes:", "POST /posts", "Tables:", "legacy", "Columns:", "posts.votes", "Models:", "Comment", "Findings:", "2 dead routes"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestDiffJSONFlag: --json prints the machine-readable DiffReport (with the
// schema-version keys) rather than the human summary.
func TestDiffJSONFlag(t *testing.T) {
	old := model.New("blog", "11.x")
	new := model.New("blog", "11.x")
	new.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "C", Action: "i"})

	cmd := newDiffCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{writeModelFile(t, old, "old.json"), writeModelFile(t, new, "new.json"), "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--json diff returned error %v, want nil\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"schema_version_old"`) {
		t.Errorf("--json output is not the DiffReport JSON:\n%s", out.String())
	}
}
