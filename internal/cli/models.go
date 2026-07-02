package cli

// unlaravel models: the model view command (ADR 0008). There is no
// pre-existing renderer for models (unlike routes/er), so the "piped" and
// "styled" modes share one plain-text writer — a real terminal gets no extra
// styling here beyond what a pipe gets, since there is no golden contract to
// preserve either way and a second presentation would be pure duplication.

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

func addModelsCommand() {
	modelsCmd := &cobra.Command{
		Use:   "models [path]",
		Short: "List Eloquent models: mass assignment, casts, indexes",
		Long: `List every Eloquent Model extracted from app/Models, with its mass-
assignment state (Unguarded/Fillable/Guarded/Protected), declared casts, and
the indexes on its mapped table.

--json emits the models array from the unlaravel.json contract.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runModels,
	}
	modelsCmd.Flags().Bool("json", false, "Emit models as JSON")
	rootCmd.AddCommand(modelsCmd)
}

func runModels(cmd *cobra.Command, args []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	projectPath := projectPathArg(cmd, args)
	out := cmd.OutOrStdout()

	pm, err := engine.Analyze(projectPath)
	if err != nil {
		return err
	}

	if asJSON {
		return emitJSON(out, modelsJSON{
			SchemaVersion: pm.SchemaVersion,
			Models:        pm.Models,
		})
	}

	printModels(out, pm.Models)
	return nil
}

func printModels(out io.Writer, models []model.Model) {
	if len(models) == 0 {
		fmt.Fprintln(out, styleDim.Render("No models found."))
		return
	}

	for i, m := range models {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "%s  %s\n", styleBold.Render(m.Name), styleDim.Render("→ "+m.Table))
		fmt.Fprintln(out, "  "+massAssignmentLabel(m))

		if len(m.Casts) > 0 {
			fmt.Fprint(out, "  casts: ")
			for j, c := range m.Casts {
				if j > 0 {
					fmt.Fprint(out, ", ")
				}
				fmt.Fprintf(out, "%s:%s", c.Column, c.Type)
			}
			fmt.Fprintln(out)
		}

		if len(m.Relationships) > 0 {
			fmt.Fprint(out, "  relationships: ")
			for j, r := range m.Relationships {
				if j > 0 {
					fmt.Fprint(out, ", ")
				}
				fmt.Fprintf(out, "%s (%s → %s)", r.Method, r.Kind, r.Target)
			}
			fmt.Fprintln(out)
		}
	}
	fmt.Fprintf(out, "\n%d model(s)\n", len(models))
}

// massAssignmentLabel mirrors the dashboard's four mass-assignment states
// (design.md component inventory: "Unguarded/Fillable/Guarded/Protected
// pills"), reading the same nil-vs-empty distinction internal/model.Model
// documents as load-bearing.
func massAssignmentLabel(m model.Model) string {
	switch {
	case m.Guarded != nil && len(m.Guarded) == 0:
		return styleDanger.Render("UNGUARDED") + styleDim.Render(" (guarded = [])")
	case len(m.Fillable) > 0:
		return styleGood.Render("FILLABLE") + styleDim.Render(fmt.Sprintf(" (%d field(s))", len(m.Fillable)))
	case len(m.Guarded) > 0:
		return styleWarn.Render("GUARDED") + styleDim.Render(fmt.Sprintf(" (%d field(s))", len(m.Guarded)))
	default:
		return styleDim.Render("PROTECTED (no fillable/guarded declared)")
	}
}
