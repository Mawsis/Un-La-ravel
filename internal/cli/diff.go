package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/diff"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// errNewBlocker is the sentinel runDiff returns when the diff introduces a new
// blocker finding, so Execute → main.main maps it to a non-zero process exit
// (the CI gate) WITHOUT Cobra echoing a Go error — the printed report is the
// message. The diff command sets SilenceErrors/SilenceUsage so this sentinel
// never reaches the user as text. It mirrors doctor's errFindings.
var errNewBlocker = errors.New("diff: introduces a new blocker finding")

// addDiffCommand registers the 'diff' subcommand on the root command.
func addDiffCommand() {
	rootCmd.AddCommand(newDiffCommand())
}

// newDiffCommand builds the 'diff' subcommand: it reads two unlaravel.json
// contracts (an old and a new analysis of the same project), prints what changed
// per section, and — so it is usable as a CI gate — exits non-zero when the diff
// introduces a NEW blocker finding (a dead route, an unauthenticated write
// route, an unguarded model that the old model did not have). Returned as a fresh
// command (rather than mutating a package global) so it can be driven in
// isolation by a test.
func newDiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <old.json> <new.json>",
		Short: "Diff two unlaravel.json contracts (non-zero exit on a new blocker finding)",
		Long: `Compare two unlaravel.json analysis contracts and report what changed.

diff reads an OLD and a NEW unlaravel.json (each produced by 'analyze --output')
and enumerates, per section, what the change added, removed, or altered:
• routes    — added / removed / changed (by method+URI)
• tables    — added / removed / changed (by name)
• columns   — added / removed / changed across all tables (by table+name)
• models    — added / removed / changed (by name)
• findings  — added / removed / changed, including a severity change

It exits non-zero when the diff introduces a NEW blocker finding — one present in
the new model but not the old, or an existing finding whose severity rose to
blocker — so a PR that adds a dead route or an unauthenticated write route fails
its check. A diff that introduces no new blocker exits zero.

Two contracts within the same major schema version are compared field-by-field;
a major-version mismatch is refused with a clear error rather than compared
across a possible shape change.

Use --json to print the machine-readable DiffReport instead of the human summary.`,
		Args: cobra.ExactArgs(2),
		// Silence Cobra's own error/usage echo: the printed report is the user
		// message, and errNewBlocker exists only to set the exit code. A genuine
		// read/parse/version error is surfaced on stderr by runDiff itself.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runDiff,
	}
	// --json prints the serialized DiffReport (the same deterministic bytes the
	// module produces) instead of the human summary, for scripting.
	cmd.Flags().Bool("json", false, "print the machine-readable DiffReport JSON instead of the human summary")
	return cmd
}

// runDiff handles the diff command. Like doctor it is pure presentation over a
// pure module: it loads the two models, refuses an incompatible major-version
// pair with a clear error, computes the DiffReport, prints it (human or --json),
// and returns errNewBlocker when the diff introduces a new blocker so the process
// exits non-zero. A genuine load or serialize error is returned as-is (also
// non-zero, with its wrapped message surfaced on stderr).
func runDiff(cmd *cobra.Command, args []string) error {
	oldPath, newPath := args[0], args[1]

	old, err := loadModelFile(oldPath)
	if err != nil {
		color.New(color.FgRed).Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return err
	}
	new, err := loadModelFile(newPath)
	if err != nil {
		color.New(color.FgRed).Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return err
	}

	// Refuse a major-version mismatch before diffing: within a major the contract
	// only adds fields (a diff is meaningful), but across a major a field may have
	// been reshaped or removed, so a comparison could mislead. Tolerated where
	// fields allow; a clear error where they do not (issue #53).
	if !diff.VersionCompatible(old, new) {
		err := fmt.Errorf("schema version mismatch: old is %q, new is %q — cannot diff across incompatible major versions",
			old.SchemaVersion, new.SchemaVersion)
		color.New(color.FgRed).Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return err
	}

	report := diff.Diff(old, new)

	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		data, err := report.ToJSON()
		if err != nil {
			color.New(color.FgRed).Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprint(cmd.OutOrStdout(), diffReport(report))
	}

	if report.IntroducesBlocker() {
		return errNewBlocker
	}
	return nil
}

// loadModelFile reads and parses an unlaravel.json contract into a ProjectModel,
// wrapping both the read error and the parse error with the file path so a
// failure names which input was bad — never swallowed into a silent empty model.
func loadModelFile(path string) (*model.ProjectModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model %q: %w", path, err)
	}
	var pm model.ProjectModel
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, fmt.Errorf("parse model %q: %w", path, err)
	}
	return &pm, nil
}

// diffReport renders the human-readable summary of a DiffReport. It is a pure
// function of the report — no I/O, no color state beyond the strings it writes —
// so it is unit-testable and can't drift from what the command prints. A diff
// with no changes prints a single reassuring line; otherwise it prints one
// section block per non-empty section, and always itemizes finding changes (so a
// new blocker is visible in the output that accompanies the non-zero exit).
func diffReport(r diff.DiffReport) string {
	var b strings.Builder

	cyan := color.New(color.FgCyan)
	cyan.Fprintf(&b, "🔀 Diff %s → %s\n\n", r.SchemaVersionOld, r.SchemaVersionNew)

	if r.Empty() {
		color.New(color.FgGreen).Fprintln(&b, "✅ No changes between the two models.")
		return b.String()
	}

	writeRouteSection(&b, r.Routes)
	writeTableSection(&b, r.Tables)
	writeColumnSection(&b, r.Columns)
	writeModelSection(&b, r.Models)
	writeFindingSection(&b, r.Findings)

	if r.IntroducesBlocker() {
		color.New(color.FgRed).Fprintln(&b, "\n⛔ This diff introduces a new blocker finding — the gate fails.")
	}

	return b.String()
}

// writeRouteSection prints the route section when non-empty: a count line per
// added/removed route (by method+URI) and per changed route.
func writeRouteSection(b *strings.Builder, d diff.RouteDiff) {
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 {
		return
	}
	color.New(color.FgYellow).Fprintf(b, "Routes: +%d / -%d / ~%d\n", len(d.Added), len(d.Removed), len(d.Changed))
	for _, r := range d.Added {
		color.New(color.FgGreen).Fprintf(b, "  + %s %s\n", r.Method, r.URI)
	}
	for _, r := range d.Removed {
		color.New(color.FgRed).Fprintf(b, "  - %s %s\n", r.Method, r.URI)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(b, "  ~ %s %s\n", c.New.Method, c.New.URI)
	}
}

// writeTableSection prints the table section when non-empty.
func writeTableSection(b *strings.Builder, d diff.TableDiff) {
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 {
		return
	}
	color.New(color.FgYellow).Fprintf(b, "Tables: +%d / -%d / ~%d\n", len(d.Added), len(d.Removed), len(d.Changed))
	for _, t := range d.Added {
		color.New(color.FgGreen).Fprintf(b, "  + %s\n", t.Name)
	}
	for _, t := range d.Removed {
		color.New(color.FgRed).Fprintf(b, "  - %s\n", t.Name)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(b, "  ~ %s\n", c.New.Name)
	}
}

// writeColumnSection prints the column section when non-empty, naming each
// column by table.column.
func writeColumnSection(b *strings.Builder, d diff.ColumnDiff) {
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 {
		return
	}
	color.New(color.FgYellow).Fprintf(b, "Columns: +%d / -%d / ~%d\n", len(d.Added), len(d.Removed), len(d.Changed))
	for _, c := range d.Added {
		color.New(color.FgGreen).Fprintf(b, "  + %s.%s\n", c.Table, c.Column.Name)
	}
	for _, c := range d.Removed {
		color.New(color.FgRed).Fprintf(b, "  - %s.%s\n", c.Table, c.Column.Name)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(b, "  ~ %s.%s (%s → %s)\n", c.Table, c.New.Name, c.Old.Type, c.New.Type)
	}
}

// writeModelSection prints the model section when non-empty.
func writeModelSection(b *strings.Builder, d diff.ModelDiff) {
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 {
		return
	}
	color.New(color.FgYellow).Fprintf(b, "Models: +%d / -%d / ~%d\n", len(d.Added), len(d.Removed), len(d.Changed))
	for _, m := range d.Added {
		color.New(color.FgGreen).Fprintf(b, "  + %s\n", m.Name)
	}
	for _, m := range d.Removed {
		color.New(color.FgRed).Fprintf(b, "  - %s\n", m.Name)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(b, "  ~ %s\n", c.New.Name)
	}
}

// writeFindingSection prints the findings section when non-empty. Finding
// changes are always itemized by label (and severity transition), so a new
// blocker is legible in the report that accompanies the non-zero exit.
func writeFindingSection(b *strings.Builder, d diff.FindingDiff) {
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 {
		return
	}
	color.New(color.FgYellow).Fprintf(b, "Findings: +%d / -%d / ~%d\n", len(d.Added), len(d.Removed), len(d.Changed))
	for _, f := range d.Added {
		color.New(color.FgRed).Fprintf(b, "  + %s [%s]\n", f.Label, f.Severity)
	}
	for _, f := range d.Removed {
		color.New(color.FgGreen).Fprintf(b, "  - %s [%s]\n", f.Label, f.Severity)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(b, "  ~ %s [%s → %s]\n", c.New.Label, c.Old.Severity, c.New.Severity)
	}
}
