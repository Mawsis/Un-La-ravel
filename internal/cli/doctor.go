package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
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
do not fail. Omitting --fail-on preserves the default: any finding fails.`,
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

	out, clean := doctorReport(pm, failOn)
	fmt.Fprint(cmd.OutOrStdout(), out)

	if !clean {
		return errFindings
	}
	return nil
}

// doctorReport renders the itemized health verdict for pm and reports whether the
// gate passes. It is a pure function of the model's Findings and the failOn
// threshold — no I/O, no color state — so it is unit-testable and can't drift from
// what the command prints. It returns the human-readable report and clean == true
// iff the gate passes.
//
// failOn is the --fail-on threshold (issue #48), one of the model.Severity*
// values, or "" for the default. With "" the gate fails on ANY finding, exactly
// today's CI behavior. With a threshold, only findings AT OR ABOVE it fail the
// gate (via model.SeverityAtLeast); below-threshold findings are still itemized
// but the gate passes, with a line naming the threshold so a green CI log explains
// itself.
//
// The report mirrors the reassuring/warning split of reportDisagreements and
// reportDeadRoutes: a single "no issues" line when there are no findings,
// otherwise a header and one line per finding using the finding's own pluralized
// Label, in the fixed emit order the model carries.
func doctorReport(pm *model.ProjectModel, failOn string) (string, bool) {
	if len(pm.Findings) == 0 {
		return color.New(color.FgGreen).Sprintln("✅ No issues found. This project is healthy."), true
	}

	// A finding fails the gate when no threshold is set (default: any finding) or
	// when its severity is at or above the threshold. Below-threshold findings
	// still print — the gate passing does not hide them.
	gateFailed := false
	for _, f := range pm.Findings {
		if failOn == "" || model.SeverityAtLeast(f.Severity, failOn) {
			gateFailed = true
			break
		}
	}

	var b strings.Builder
	warn := color.New(color.FgRed)
	warn.Fprintf(&b, "⚠️  Found %s:\n", countLabel(len(pm.Findings)))
	for _, f := range pm.Findings {
		warn.Fprintf(&b, "  • %s\n", f.Label)
	}

	if !gateFailed {
		// Findings exist but all fall below the threshold — say so, so a passing
		// CI run explains why it did not fail despite the itemized findings above.
		color.New(color.FgGreen).Fprintf(&b, "✅ No findings at or above %q — gate passes.\n", failOn)
	}

	return b.String(), !gateFailed
}

// countLabel pluralizes the finding-category count for the verdict header, so it
// reads "1 problem category" / "2 problem categories".
func countLabel(n int) string {
	if n == 1 {
		return "1 problem category"
	}
	return fmt.Sprintf("%d problem categories", n)
}
