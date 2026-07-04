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
	return &cobra.Command{
		Use:   "doctor [path]",
		Short: "Report a Laravel project's health verdict (non-zero exit on findings)",
		Long: `Report the health verdict for a Laravel project.

doctor runs the same static analysis as 'analyze' and prints an itemized verdict
of the problems it found:
• dead routes — routes whose controller/action edge dangles
• model↔schema disagreements — relationships referencing something the schema lacks
• unguarded models — models that wrote 'protected $guarded = []'

It exits non-zero when any finding is present, so it can gate CI. A clean project
prints a single line and exits zero.`,
		Args: cobra.MaximumNArgs(1),
		// Silence Cobra's own error/usage echo: the printed verdict is the user
		// message, and the sentinel errFindings exists only to set the exit code.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runDoctor,
	}
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

	out, clean := doctorReport(pm)
	fmt.Fprint(cmd.OutOrStdout(), out)

	if !clean {
		return errFindings
	}
	return nil
}

// doctorReport renders the itemized health verdict for pm and reports whether the
// project is clean. It is a pure function of the model's Findings — no I/O, no
// color state — so it is unit-testable and can't drift from what the command
// prints. It returns the human-readable report and clean == true iff there are no
// findings.
//
// The report mirrors the reassuring/warning split of reportDisagreements and
// reportDeadRoutes: a single "no issues" line when clean, otherwise a header and
// one line per finding using the finding's own pluralized Label, in the fixed
// emit order the model carries.
func doctorReport(pm *model.ProjectModel) (string, bool) {
	if len(pm.Findings) == 0 {
		return color.New(color.FgGreen).Sprintln("✅ No issues found. This project is healthy."), true
	}

	var b strings.Builder
	warn := color.New(color.FgRed)
	warn.Fprintf(&b, "⚠️  Found %s:\n", countLabel(len(pm.Findings)))
	for _, f := range pm.Findings {
		warn.Fprintf(&b, "  • %s\n", f.Label)
	}
	return b.String(), false
}

// countLabel pluralizes the finding-category count for the verdict header, so it
// reads "1 problem category" / "2 problem categories".
func countLabel(n int) string {
	if n == 1 {
		return "1 problem category"
	}
	return fmt.Sprintf("%d problem categories", n)
}
