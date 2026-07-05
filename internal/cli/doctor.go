package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/baseline"
	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/findings"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// errFindings is the sentinel RunE returns when doctor found at least one
// finding, so Execute → main.main maps it to a non-zero process exit (the CI
// gate) WITHOUT Cobra echoing a Go error — the itemized verdict already printed
// is the message. The doctor command sets SilenceErrors/SilenceUsage so this
// sentinel never reaches the user as text.
var errFindings = errors.New("doctor: project has findings")

// addDoctorCommand registers the 'doctor' subcommand on the root command.
func addDoctorCommand() {
	rootCmd.AddCommand(newDoctorCommand())
}

// newDoctorCommand builds the 'doctor' subcommand. doctor is the CLI face of the
// health verdict (issue #27, lifted server-side): it runs the same
// engine.Analyze the dashboard does, prints the itemized verdict computed into
// the Project Model (internal/findings), and — so it is usable as a CI gate —
// exits non-zero when the project has any finding. Returned as a fresh command
// (rather than mutating a package global) so it can be driven in isolation by a
// test.
func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Report a Laravel project's health verdict (non-zero exit on findings)",
		Long: `Report the health verdict for a Laravel project.

doctor runs the same static analysis as 'analyze' and prints an itemized verdict
of the problems it found:
• dead routes — routes whose controller/action edge dangles
• model↔schema disagreements — relationships referencing something the schema lacks
• unguarded models — models that wrote 'protected $guarded = []'

It exits non-zero when any finding is present, so it can gate CI. A clean project
prints a single line and exits zero.

Use --fail-on <severity> to gate on severity: only findings at or above the given
level (blocker > warn > info) fail the gate; less-severe findings still print but
do not fail. Omitting --fail-on preserves the default: any finding fails.

Use --baseline <file> to suppress already-known findings recorded in a committed
baseline file: baselined findings are excluded from the exit-code decision but
still reported, so a legacy project can adopt the gate without fixing its history
first. Baseline entries that no longer match any current finding are reported as
stale (they do not fail the gate) so the baseline can be pruned.`,
		Args: cobra.MaximumNArgs(1),
		// Silence Cobra's own error/usage echo: the printed verdict is the user
		// message, and the sentinel errFindings exists only to set the exit code.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runDoctor,
	}
	// --fail-on gates CI on severity. Empty (unset) means the default: any finding
	// fails. A non-empty value is validated in runDoctor against the model's
	// severities so a typo fails loudly rather than silently passing the gate.
	cmd.Flags().String("fail-on", "", "fail only on findings at or above this severity (blocker|warn|info); default fails on any finding")
	// --baseline suppresses findings recorded in a committed baseline file from
	// the exit-code decision (still reporting them), so a legacy project can adopt
	// the gate without fixing history first. Empty (unset) means no suppression.
	cmd.Flags().String("baseline", "", "suppress findings recorded in this baseline file from the exit-code decision")
	return cmd
}

// validFailOn is the set of accepted --fail-on values, mirroring the model's
// severities. It exists so an invalid threshold is rejected with a clear message
// rather than silently disabling the gate (an unknown threshold would otherwise
// match nothing). Defined here — the CLI boundary — because validating user flag
// input is a presentation concern, while the ranking itself lives in the model.
var validFailOn = map[string]bool{
	model.SeverityBlocker: true,
	model.SeverityWarn:    true,
	model.SeverityInfo:    true,
}

// runDoctor handles the doctor command. Like analyzeProject it is pure
// presentation over engine.Analyze: it gets the assembled Project Model (findings
// already computed by the engine), prints the itemized verdict via doctorReport,
// and returns errFindings when the project is not clean so the process exits
// non-zero. A genuine analysis error is returned as-is (also non-zero, with the
// wrapped message).
func runDoctor(cmd *cobra.Command, args []string) error {
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	// Validate --fail-on before any analysis. An unknown threshold matches no
	// severity, so left unchecked it would silently pass every gate — the opposite
	// of what a CI author asked for. Fail fast with a message naming the bad value
	// and the valid ones. SilenceErrors is set, so surface it on stderr ourselves.
	failOn, _ := cmd.Flags().GetString("fail-on")
	if failOn != "" && !validFailOn[failOn] {
		err := fmt.Errorf("invalid --fail-on value %q: want one of %s, %s, %s",
			failOn, model.SeverityBlocker, model.SeverityWarn, model.SeverityInfo)
		color.New(color.FgRed).Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return err
	}

	// Load the baseline before analysis so a bad path fails fast, before the
	// expensive analyze. A missing or unparseable baseline is a hard error: left
	// to silently disable suppression, it would fail CI on findings the user
	// believed were suppressed — the opposite of what --baseline asks for.
	baselinePath, _ := cmd.Flags().GetString("baseline")
	var base *baseline.Baseline
	var err error
	if baselinePath != "" {
		base, err = loadBaseline(baselinePath)
		if err != nil {
			color.New(color.FgRed).Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return err
		}
	}

	color.New(color.FgCyan).Fprintf(cmd.OutOrStdout(), "🩺 Diagnosing Laravel project: %s\n\n", projectPath)

	pm, err := engine.Analyze(projectPath)
	if err != nil {
		// doctor sets SilenceErrors so the errFindings sentinel stays quiet (the
		// printed verdict is the message). That silence must NOT swallow a genuine
		// analysis failure — surface its reason here so the user sees why doctor
		// exited non-zero, rather than a bare non-zero exit with no output.
		color.New(color.FgRed).Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return err
	}

	out, clean := doctorReport(pm, failOn, base)
	fmt.Fprint(cmd.OutOrStdout(), out)

	if !clean {
		return errFindings
	}
	return nil
}

// loadBaseline reads and parses a committed baseline file, wrapping both the read
// error and the parse error with "baseline" context so the failure names the
// feature the user invoked. A read failure (missing/unreadable file) and a parse
// failure (bad JSON or unsupported version) are both surfaced — never swallowed
// into a silent no-op suppression.
func loadBaseline(path string) (*baseline.Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline %q: %w", path, err)
	}
	base, err := baseline.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("load baseline %q: %w", path, err)
	}
	return base, nil
}

// doctorReport renders the itemized health verdict for pm and reports whether the
// gate passes. It is a pure function of the model's Findings, the failOn
// threshold, and an optional baseline — no I/O, no color state — so it is
// unit-testable and can't drift from what the command prints. It returns the
// human-readable report and clean == true iff the gate passes.
//
// failOn is the --fail-on threshold (issue #48), one of the model.Severity*
// values, or "" for the default. With "" the gate fails on ANY finding, exactly
// today's CI behavior. With a threshold, only findings AT OR ABOVE it fail the
// gate (via model.SeverityAtLeast); below-threshold findings are still itemized
// but the gate passes, with a line naming the threshold so a green CI log explains
// itself.
//
// base is the --baseline suppression set (issue #49), or nil for none. When set,
// findings whose per-item fingerprint is in the baseline are excluded from the
// gate decision and listed under a "suppressed" heading (the debt stays visible);
// baseline entries matching no current finding are listed as "stale" so the file
// can be pruned. Suppression composes with failOn: the gate fails only on a
// SURVIVING finding at or above the threshold.
//
// The report mirrors the reassuring/warning split of reportDisagreements and
// reportDeadRoutes: a single "no issues" line when there are no gating findings,
// otherwise a header and one line per finding using the finding's own pluralized
// Label, in the fixed emit order the model carries.
func doctorReport(pm *model.ProjectModel, failOn string, base *baseline.Baseline) (string, bool) {
	// Split the per-item findings against the baseline. With no baseline, every
	// item survives and nothing is suppressed or stale, so the branch below
	// reduces to the original rollup-based report.
	var suppressed []findings.Item
	var stale []string
	survivorFindings := pm.Findings
	if base != nil {
		var survivors []findings.Item
		survivors, suppressed, stale = baseline.Subtract(findings.Items(pm), base)
		// Rebuild the category rollups from only the survivors, so the gate AND the
		// itemized counts reflect what the baseline did NOT suppress — a category
		// with one of two items suppressed reports "1", not the pre-suppression "2".
		survivorFindings = findings.Rollup(survivors)
	}

	// Nothing gates and nothing to report about the baseline → the healthy line.
	if len(survivorFindings) == 0 && len(suppressed) == 0 && len(stale) == 0 {
		return color.New(color.FgGreen).Sprintln("✅ No issues found. This project is healthy."), true
	}

	// A surviving finding fails the gate when no threshold is set (default: any
	// finding) or when its severity is at or above the threshold.
	gateFailed := false
	for _, f := range survivorFindings {
		if failOn == "" || model.SeverityAtLeast(f.Severity, failOn) {
			gateFailed = true
			break
		}
	}

	var b strings.Builder
	if len(survivorFindings) > 0 {
		warn := color.New(color.FgRed)
		warn.Fprintf(&b, "⚠️  Found %s:\n", countLabel(len(survivorFindings)))
		for _, f := range survivorFindings {
			warn.Fprintf(&b, "  • %s\n", f.Label)
		}
	} else if base != nil {
		color.New(color.FgGreen).Fprintf(&b, "✅ No un-baselined findings.\n")
	}

	writeSuppressed(&b, suppressed)
	writeStale(&b, stale)

	if !gateFailed && len(survivorFindings) > 0 {
		// Surviving findings exist but all fall below the threshold — say so, so a
		// passing CI run explains why it did not fail despite the itemized findings.
		color.New(color.FgGreen).Fprintf(&b, "✅ No findings at or above %q — gate passes.\n", failOn)
	}

	return b.String(), !gateFailed
}

// writeSuppressed lists the findings the baseline suppressed, so the debt stays
// visible even though it does not gate. No-op when nothing was suppressed.
func writeSuppressed(b *strings.Builder, suppressed []findings.Item) {
	if len(suppressed) == 0 {
		return
	}
	dim := color.New(color.FgHiBlack)
	dim.Fprintf(b, "🔕 Suppressed by baseline (%d):\n", len(suppressed))
	for _, it := range suppressed {
		dim.Fprintf(b, "  • %s\n", itemLabel(it))
	}
}

// writeStale lists baseline entries that matched no current finding, prompting a
// prune. Stale entries do not gate. No-op when there are none.
func writeStale(b *strings.Builder, stale []string) {
	if len(stale) == 0 {
		return
	}
	note := color.New(color.FgYellow)
	note.Fprintf(b, "🧹 Stale baseline entries (%d) — no longer match any finding, consider pruning:\n", len(stale))
	for _, fp := range stale {
		note.Fprintf(b, "  • %s\n", fp)
	}
}

// itemLabel renders a per-item finding as a short human-readable identifier for
// the suppressed list, keyed by kind so each reads naturally (a dead route by its
// method+URI, an unguarded model by its class, a disagreement by model+relation).
func itemLabel(it findings.Item) string {
	switch it.Kind {
	case model.FindingDeadRoutes:
		return fmt.Sprintf("dead route %s %s → %s", it.Method, it.URI, it.Controller)
	case model.FindingUnguarded:
		return fmt.Sprintf("unguarded model %s", it.Class)
	case model.FindingDisagreements:
		return fmt.Sprintf("disagreement %s.%s", it.Model, it.Relationship)
	default:
		return it.Kind
	}
}

// countLabel pluralizes the finding-category count for the verdict header, so it
// reads "1 problem category" / "2 problem categories".
func countLabel(n int) string {
	if n == 1 {
		return "1 problem category"
	}
	return fmt.Sprintf("%d problem categories", n)
}
