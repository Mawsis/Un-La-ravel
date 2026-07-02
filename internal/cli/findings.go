package cli

// unlaravel findings: the findings view command (ADR 0008). Prints
// Disagreements and Dead Routes — or a reassuring "all clear" line — as
// findings are facts, not alarms (design.md principle 4).

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

func addFindingsCommand() {
	findingsCmd := &cobra.Command{
		Use:   "findings [path]",
		Short: "List Disagreements and Dead Routes",
		Long: `List every Disagreement (a Model Relationship referencing something the
Schema lacks) and every Dead Route (a Route whose Controller/Action edge
doesn't resolve). Both are findings, not errors — the tool reports them, it
does not fail on them.

--json emits the disagreements and dead_routes arrays from the
unlaravel.json contract.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runFindings,
	}
	findingsCmd.Flags().Bool("json", false, "Emit findings as JSON")
	rootCmd.AddCommand(findingsCmd)
}

func runFindings(cmd *cobra.Command, args []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	projectPath := projectPathArg(cmd, args)
	out := cmd.OutOrStdout()

	pm, err := engine.Analyze(projectPath)
	if err != nil {
		return err
	}

	if asJSON {
		return emitJSON(out, findingsJSON{
			SchemaVersion: pm.SchemaVersion,
			Disagreements: pm.Disagreements,
			DeadRoutes:    pm.DeadRoutes,
		})
	}

	printFindings(out, pm)
	return nil
}

func printFindings(cmd io.Writer, pm *model.ProjectModel) {
	if len(pm.Disagreements) == 0 && len(pm.DeadRoutes) == 0 {
		fmt.Fprintln(cmd, styleGood.Render("All clear — no disagreements, no dead routes."))
		return
	}

	if len(pm.Disagreements) > 0 {
		fmt.Fprintln(cmd, styleWarn.Render(fmt.Sprintf("Disagreements (%d):", len(pm.Disagreements))))
		for _, d := range pm.Disagreements {
			fmt.Fprintf(cmd, "  %s::%s — %s\n", d.Model, d.Relationship, d.Reason)
		}
	}

	if len(pm.DeadRoutes) > 0 {
		if len(pm.Disagreements) > 0 {
			fmt.Fprintln(cmd)
		}
		fmt.Fprintln(cmd, styleDanger.Render(fmt.Sprintf("Dead routes (%d):", len(pm.DeadRoutes))))
		for _, d := range pm.DeadRoutes {
			fmt.Fprintf(cmd, "  %s %s — %s\n", d.Method, d.URI, d.Reason)
		}
	}
}
