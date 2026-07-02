package cli

// analyzeProject and its command registration. Per ADR 0008, analyze no
// longer dumps every artifact — it prints a compact summary (project,
// version, six Node counts, findings digest, and a hint pointing at the
// relevant view command) and keeps --output/--openapi for writing the full
// unlaravel.json contract and OpenAPI 3 spec to a file. The full ER diagram
// and route map now live in their own `er` and `routes` commands.

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/render/openapi"
)

// addAnalyzeCommand adds the 'analyze' subcommand to the root command.
func addAnalyzeCommand() {
	analyzeCmd := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Analyze a Laravel project and print a compact summary",
		Long: `Analyze a Laravel project and print a compact summary.

Prints the project name, detected Laravel version, counts of every Node type
(tables, models, controllers, routes, form requests), and a one-line findings
digest (disagreements, dead routes). For the full detail behind any of those
counts, use the matching view command: 'unlaravel routes', 'unlaravel models',
'unlaravel er', or 'unlaravel findings'.

--output and --openapi still write the full unlaravel.json contract and
OpenAPI 3 spec to a file, independent of what's printed to the terminal.`,
		Args: cobra.MaximumNArgs(1),
		RunE: analyzeProject,
	}

	analyzeCmd.Flags().StringP("output", "o", "", "Output file path for the unlaravel.json analysis contract")
	analyzeCmd.Flags().String("openapi", "", "Output file path for the generated OpenAPI 3 spec (JSON)")

	rootCmd.AddCommand(analyzeCmd)
}

// analyzeProject handles the analyze command. The analysis LOGIC lives in
// internal/engine (ADR 0001/0004: one Project Model, many renderers) — this
// function is PURE PRESENTATION: it calls engine.Analyze, prints a compact
// summary, and optionally writes the JSON contract (--output) and the OpenAPI
// 3 spec (--openapi).
func analyzeProject(cmd *cobra.Command, args []string) error {
	outputPath, _ := cmd.Flags().GetString("output")
	openAPIPath, _ := cmd.Flags().GetString("openapi")
	projectPath := projectPathArg(cmd, args)

	out := cmd.OutOrStdout()

	pm, err := engine.Analyze(projectPath)
	if err != nil {
		return err
	}

	printSummary(out, projectPath, pm)

	if outputPath != "" {
		if err := writeProjectModel(pm, outputPath); err != nil {
			return err
		}
		fmt.Fprintln(out, styleGood.Render("Wrote analysis to "+outputPath))
	}

	if openAPIPath != "" {
		if err := writeOpenAPI(pm, openAPIPath); err != nil {
			return err
		}
		fmt.Fprintln(out, styleGood.Render("Wrote OpenAPI 3 spec to "+openAPIPath))
	}

	printFindingsDigest(out, pm)
	printHint(out, pm)

	return nil
}

// printSummary prints the project header line and the six Node-type counts.
func printSummary(out io.Writer, projectPath string, pm *model.ProjectModel) {
	fmt.Fprintln(out, styleHeading.Render("Un(la)ravel — "+projectPath))
	fmt.Fprintf(out, "%s %s\n", styleDim.Render("Laravel"), displayVersion(pm.LaravelVersion))
	fmt.Fprintf(out, "%d table(s) · %d model(s) · %d controller(s) · %d route(s) · %d form request(s)\n",
		len(pm.Schemas), len(pm.Models), len(pm.Controllers), len(pm.Routes), len(pm.FormRequests))
}

// printFindingsDigest prints a single-line summary of Disagreements and Dead
// Routes, or a reassuring "all clear" line when there are none.
func printFindingsDigest(out io.Writer, pm *model.ProjectModel) {
	total := len(pm.Disagreements) + len(pm.DeadRoutes)
	if total == 0 {
		fmt.Fprintln(out, styleGood.Render("No findings — no disagreements, no dead routes."))
		return
	}
	fmt.Fprintln(out, styleWarn.Render(fmt.Sprintf(
		"%d finding(s): %d disagreement(s), %d dead route(s)",
		total, len(pm.Disagreements), len(pm.DeadRoutes))))
}

// printHint points the user at the view command most relevant to what
// analyze just summarized, so a compact summary never dead-ends.
func printHint(out io.Writer, pm *model.ProjectModel) {
	hint := "unlaravel routes"
	if len(pm.DeadRoutes) > 0 || len(pm.Disagreements) > 0 {
		hint = "unlaravel findings"
	}
	fmt.Fprintln(out, styleDim.Render("hint: "+hint))
}

// writeProjectModel serializes the model to JSON and writes it to outputPath.
func writeProjectModel(pm *model.ProjectModel, outputPath string) error {
	data, err := pm.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize project model: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
	}
	return nil
}

// writeOpenAPI renders the Project Model to an OpenAPI 3 spec and writes it to
// openAPIPath.
func writeOpenAPI(pm *model.ProjectModel, openAPIPath string) error {
	data, err := openapi.Render(pm)
	if err != nil {
		return fmt.Errorf("failed to render OpenAPI spec: %w", err)
	}
	if err := os.WriteFile(openAPIPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write OpenAPI file %s: %w", openAPIPath, err)
	}
	return nil
}

// displayVersion returns a placeholder when the detected version is empty so
// output never shows a blank version.
func displayVersion(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}
