package cli

// Tests for the subcommand-per-view CLI (ADR 0008). Every command is
// exercised through cobra's own Execute path with cmd.SetOut/SetErr, which
// simultaneously verifies the stream discipline (commands must write via
// cmd.OutOrStdout(), never bare fmt.Println, or these buffers stay empty).
//
// Four things are pinned:
//   - Piped (non-TTY) plain output: must equal the existing golden files in
//     testdata/ (fixture-app.golden.routemap, .golden.mermaid) — no new
//     goldens needed, since ADR 0008 keeps the renderer output as the pipe
//     contract.
//   - --json output: new goldens in internal/cli/testdata/, regenerated with
//     the same package-level -update flag e2e_test.go declares.
//   - Exit codes / error paths: a non-Laravel path fails every view command.
//   - Styled (TTY) output: a smoke test only (ADR 0008 §Testing discipline),
//     forcing lipgloss's Ascii color profile and calling printRoutesTable
//     directly so the assertion is content/alignment, never color codes or
//     terminal width — this is deliberately not golden-pinned byte-for-byte.
//
// Everything except the styled-output smoke test runs with isTTY stubbed to
// false (buffers are never *os.File, so it already returns false by default),
// exercising only the piped/--json paths that ARE byte-pinned.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

const cliGoldenDir = "testdata"

// execCommand runs name against the fixture app (or an explicit path) and
// returns stdout, stderr, and any error, via a fresh cobra tree so tests never
// share persisted flag state.
func execCommand(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	root := repoRoot(t)
	fixtureApp := filepath.Join(root, fixtureAppRel)

	// Substitute the fixture path placeholder so callers can write args
	// naturally (e.g. "routes", "--json") without repeating the path.
	full := make([]string, 0, len(args)+1)
	full = append(full, args[0])
	full = append(full, fixtureApp)
	full = append(full, args[1:]...)

	var outBuf, errBuf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(full)

	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// newTestRootCmd rebuilds the command tree fresh for each test so flag state
// (e.g. --json from a previous test) can never leak between cases. It mirrors
// init()'s registration exactly.
func newTestRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "unlaravel",
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	saved := rootCmd
	rootCmd = cmd
	defer func() { rootCmd = saved }()

	addAnalyzeCommand()
	addRoutesCommand()
	addModelsCommand()
	addERCommand()
	addOpenAPICommand()
	addFindingsCommand()

	return cmd
}

func goldenPath(name string) string {
	return filepath.Join(cliGoldenDir, name)
}

func compareOrUpdateGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)

	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create it)", path, err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch (run with -update to review and regenerate):\n--- got ---\n%s\n--- want ---\n%s", name, got, string(want))
	}
}

// --- Piped plain output == existing renderer goldens ---

func TestRoutes_PlainOutput_MatchesRouteMapGolden(t *testing.T) {
	root := repoRoot(t)
	want, err := os.ReadFile(filepath.Join(root, goldenRouteMapRel))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	stdout, stderr, err := execCommand(t, "routes")
	if err != nil {
		t.Fatalf("unlaravel routes: %v (stderr: %s)", err, stderr)
	}
	if stdout != string(want) {
		t.Errorf("routes plain output != testdata/fixture-app.golden.routemap\n--- got ---\n%s\n--- want ---\n%s", stdout, string(want))
	}
}

func TestER_PlainOutput_MatchesMermaidGolden(t *testing.T) {
	root := repoRoot(t)
	want, err := os.ReadFile(filepath.Join(root, goldenMermaidRel))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	stdout, stderr, err := execCommand(t, "er")
	if err != nil {
		t.Fatalf("unlaravel er: %v (stderr: %s)", err, stderr)
	}
	if stdout != string(want) {
		t.Errorf("er plain output != testdata/fixture-app.golden.mermaid\n--- got ---\n%s\n--- want ---\n%s", stdout, string(want))
	}
}

func TestOpenAPI_Output_MatchesOpenAPIGolden(t *testing.T) {
	root := repoRoot(t)
	want, err := os.ReadFile(filepath.Join(root, goldenOpenAPIRel))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	stdout, stderr, err := execCommand(t, "openapi")
	if err != nil {
		t.Fatalf("unlaravel openapi: %v (stderr: %s)", err, stderr)
	}
	// runOpenAPI writes via Fprintln, adding one trailing newline beyond the
	// golden's own trailing newline; trim both sides to the same normalized
	// form so this test tracks content, not that incidental formatting detail.
	gotTrimmed := strings.TrimRight(stdout, "\n")
	wantTrimmed := strings.TrimRight(string(want), "\n")
	if gotTrimmed != wantTrimmed {
		t.Errorf("openapi output != testdata/fixture-app.openapi.json\n--- got ---\n%s\n--- want ---\n%s", stdout, string(want))
	}
}

// --- --json goldens ---

func TestRoutes_JSON_Golden(t *testing.T) {
	stdout, stderr, err := execCommand(t, "routes", "--json")
	if err != nil {
		t.Fatalf("unlaravel routes --json: %v (stderr: %s)", err, stderr)
	}
	compareOrUpdateGolden(t, "routes.golden.json", stdout)
}

func TestModels_JSON_Golden(t *testing.T) {
	stdout, stderr, err := execCommand(t, "models", "--json")
	if err != nil {
		t.Fatalf("unlaravel models --json: %v (stderr: %s)", err, stderr)
	}
	compareOrUpdateGolden(t, "models.golden.json", stdout)
}

func TestER_JSON_Golden(t *testing.T) {
	stdout, stderr, err := execCommand(t, "er", "--json")
	if err != nil {
		t.Fatalf("unlaravel er --json: %v (stderr: %s)", err, stderr)
	}
	compareOrUpdateGolden(t, "er.golden.json", stdout)
}

func TestFindings_JSON_Golden(t *testing.T) {
	stdout, stderr, err := execCommand(t, "findings", "--json")
	if err != nil {
		t.Fatalf("unlaravel findings --json: %v (stderr: %s)", err, stderr)
	}
	compareOrUpdateGolden(t, "findings.golden.json", stdout)
}

// --- Plain (non-JSON) findings / models text goldens ---

func TestFindings_PlainOutput_Golden(t *testing.T) {
	stdout, stderr, err := execCommand(t, "findings")
	if err != nil {
		t.Fatalf("unlaravel findings: %v (stderr: %s)", err, stderr)
	}
	compareOrUpdateGolden(t, "findings.golden.txt", stdout)
}

func TestModels_PlainOutput_Golden(t *testing.T) {
	stdout, stderr, err := execCommand(t, "models")
	if err != nil {
		t.Fatalf("unlaravel models: %v (stderr: %s)", err, stderr)
	}
	compareOrUpdateGolden(t, "models.golden.txt", stdout)
}

func TestAnalyze_PlainOutput_Golden(t *testing.T) {
	stdout, stderr, err := execCommand(t, "analyze")
	if err != nil {
		t.Fatalf("unlaravel analyze: %v (stderr: %s)", err, stderr)
	}
	compareOrUpdateGolden(t, "analyze.golden.txt", stdout)
}

// --- Streams: --json must be the ONLY thing on stdout ---

func TestJSONMode_StdoutIsExclusivelyJSON(t *testing.T) {
	for _, cmdName := range []string{"routes", "models", "er", "findings"} {
		t.Run(cmdName, func(t *testing.T) {
			stdout, stderr, err := execCommand(t, cmdName, "--json")
			if err != nil {
				t.Fatalf("unlaravel %s --json: %v (stderr: %s)", cmdName, err, stderr)
			}
			if len(stdout) == 0 {
				t.Fatalf("unlaravel %s --json: empty stdout", cmdName)
			}
			if stdout[0] != '{' {
				t.Errorf("unlaravel %s --json: stdout does not start with '{' — got: %.40s", cmdName, stdout)
			}
		})
	}
}

// --- Exit codes / error paths ---

func TestViewCommands_NonLaravelPath_ReturnError(t *testing.T) {
	notLaravel := t.TempDir()

	for _, cmdName := range []string{"analyze", "routes", "models", "er", "openapi", "findings"} {
		t.Run(cmdName, func(t *testing.T) {
			var outBuf, errBuf bytes.Buffer
			cmd := newTestRootCmd()
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)
			cmd.SetArgs([]string{cmdName, notLaravel})

			if err := cmd.Execute(); err == nil {
				t.Errorf("unlaravel %s %s: expected error for non-Laravel path, got nil", cmdName, notLaravel)
			}
		})
	}
}

// --- Styled (TTY) output: smoke test only, per ADR 0008 §Testing discipline ---

// TestPrintRoutesTable_StyledSmoke forces lipgloss's Ascii color profile (no
// ANSI escapes) and calls printRoutesTable directly, asserting on content and
// column alignment — never on color codes or a fixed terminal width, since
// ADR 0008 explicitly keeps styled output out of the golden-file contract.
// This is the test that would have caught two real bugs during review: a
// styled string used as a Fprintf format argument, and column widths computed
// in bytes but padded in runes.
func TestPrintRoutesTable_StyledSmoke(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() { lipgloss.SetColorProfile(prevProfile) })

	// "/café/menu" is deliberately the WIDEST URI in the set (10 runes, 11
	// bytes). UTF-8 byte length is always >= rune length, so a column-width
	// computation that measures in bytes instead of runes (the bug this test
	// guards against) can only ever OVER-estimate a multi-byte column's
	// width — padRight itself is rune-safe, so the visible symptom is one
	// extra trailing space on the URI column, not misalignment between rows.
	pm := &model.ProjectModel{
		Routes: []model.Route{
			{Method: "GET", URI: "/café/menu", Controller: "PostController", Action: "index"},
			{Method: "DELETE", URI: "/posts", Controller: "PostController", Action: "destroy"},
		},
		DeadRoutes: []model.DeadRoute{
			{Method: "DELETE", URI: "/posts", Controller: "PostController", Action: "destroy"},
		},
	}

	var buf bytes.Buffer
	printRoutesTable(&buf, pm)
	got := buf.String()

	// No ANSI escape sequences under the forced Ascii profile.
	if strings.Contains(got, "\x1b[") {
		t.Errorf("printRoutesTable: unexpected ANSI escape sequence under Ascii color profile:\n%s", got)
	}

	// Content: every route and the dead-route marker are present.
	for _, want := range []string{"GET", "/café/menu", "DELETE", "/posts", "PostController@index", "PostController@destroy", "DEAD", "2 route(s)", "1 dead"} {
		if !strings.Contains(got, want) {
			t.Errorf("printRoutesTable: output missing %q:\n%s", want, got)
		}
	}

	// Column width: the URI column must be exactly as wide as "/café/menu"
	// (10 runes) plus the fixed 2-space column gap — CONTROLLER@ACTION must
	// start at rune offset 12 (METHOD "DELETE" padded to 6 + 2-space gap +
	// URI padded to 10 + 2-space gap). If the width were computed in bytes
	// (11) instead of runes (10), CONTROLLER@ACTION would start one rune
	// later on every row — a real, provable defect even though it never
	// misaligns rows relative to EACH OTHER (padRight is itself rune-safe,
	// so a too-wide shared width just adds uniform excess space).
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("printRoutesTable: expected at least 3 lines (header + 2 routes), got %d:\n%s", len(lines), got)
	}
	wantActionCol := len("DELETE") + 2 + len([]rune("/café/menu")) + 2 // 6 + 2 + 10 + 2 = 20
	for i, line := range lines {
		if col := runeIndex(line, "PostController"); col != -1 && col != wantActionCol {
			t.Errorf("printRoutesTable: line %d's CONTROLLER@ACTION starts at rune %d, want %d (URI column width must be measured in runes, not bytes):\n%s",
				i, col, wantActionCol, got)
		}
	}
}

// runeIndex returns the rune (not byte) offset of substr's first occurrence
// in s, or -1 if absent.
func runeIndex(s, substr string) int {
	byteIdx := strings.Index(s, substr)
	if byteIdx < 0 {
		return -1
	}
	return len([]rune(s[:byteIdx]))
}

// TestPrintRoutesTable_DeadCountSuffix_UnderColor forces a color-INJECTING
// profile (ANSI256, not Ascii) and asserts the styled dead-route count
// survives intact and printRoutesTable never panics. This is a regression
// guard for output correctness under real terminal color, not a proof
// against any specific bug: go vet -printf cannot flag a lipgloss.Render(...)
// call used as an fmt.Fprintf FORMAT argument (the format string isn't a
// literal at the call site), and empirically neither can a runtime content
// check — lipgloss's injected ANSI escapes for this palette never happen to
// contain a stray '%', so a styled-string-as-format-string mistake currently
// produces correct output by coincidence, not by any actual safety net. The
// real guard against that class of bug is code review discipline: always
// pass a style's .Render(...) result as a Print/Fprint VALUE, never as a
// Printf/Fprintf FORMAT argument.
func TestPrintRoutesTable_DeadCountSuffix_UnderColor(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prevProfile) })

	pm := &model.ProjectModel{
		Routes: []model.Route{
			{Method: "DELETE", URI: "/posts/{id}", Controller: "PostController", Action: "destroy"},
		},
		DeadRoutes: []model.DeadRoute{
			{Method: "DELETE", URI: "/posts/{id}", Controller: "PostController", Action: "destroy"},
		},
	}

	var buf bytes.Buffer
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("printRoutesTable panicked under a color-injecting profile: %v", r)
			}
		}()
		printRoutesTable(&buf, pm)
	}()

	// Strip ANSI escapes before asserting on content, since ANSI256 does
	// inject them here (unlike the Ascii-profile smoke test above).
	got := stripANSI(buf.String())
	if !strings.Contains(got, "1 dead") {
		t.Errorf("printRoutesTable: dead-route count corrupted or missing under a color-injecting profile — got %q", got)
	}
}

// stripANSI removes SGR escape sequences (\x1b[...m) from s so content
// assertions can run against a color-injecting profile's output without
// needing to match exact escape codes.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == '\x1b':
			inEscape = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
