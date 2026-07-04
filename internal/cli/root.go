package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/render/er"
	"github.com/Mawsis/Un-La-ravel/internal/render/openapi"
	"github.com/Mawsis/Un-La-ravel/internal/render/routemap"
)

// rootCmd represents the base command when called without any subcommands
// In Cobra, commands are organized in a tree structure with a root command at the top
var rootCmd = &cobra.Command{
	Use:   "unlaravel",
	Short: "Un(la)ravel - Laravel project analysis tool",
	Long: color.New(color.FgCyan).Sprint(`
🔍 Un(la)ravel - Laravel Project Analysis Tool

Un(la)ravel statically analyzes a Laravel project — no booting, no artisan,
no database connection — and builds a single Project Model of its schema,
Eloquent models, routes, controllers, and form requests. From that model it
generates:
• An entity-relationship diagram (Mermaid)
• An OpenAPI 3 specification
• A route map, with dead-route detection
• Model↔schema disagreement findings
• A health verdict via 'unlaravel doctor' (non-zero exit on findings — CI-ready)

Use 'unlaravel help [command]' for more information about a command.
	`),
	// RunE is executed when the root command is called without subcommands
	// The 'E' suffix means it returns an error (vs Run which doesn't)
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is provided, show help
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// init is a special Go function that runs automatically when the package is imported
// It's perfect for setting up package-level configuration like CLI flags and subcommands
func init() {
	// Add global flags that apply to all commands
	// Cobra automatically generates help text for these flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	// Add subcommands to the root command
	// We'll create these commands in separate files for better organization
	addAnalyzeCommand()
	addDoctorCommand()
	addServeCommand()
}

// addAnalyzeCommand adds the 'analyze' subcommand to the root command
// This demonstrates the Cobra pattern of organizing commands in separate functions
func addAnalyzeCommand() {
	analyzeCmd := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Analyze a Laravel project",
		Long: `Analyze a Laravel project and generate comprehensive insights.

The analyze command will scan the specified Laravel project (or current directory if no path provided)
and generate analysis reports for:
• Database schema and relationships (ER diagram)
• Eloquent models, with model↔schema disagreement findings
• Routes, controllers, and dead-route detection
• Form requests, linked to the routes that dispatch them`,
		// Args validation - Cobra provides several built-in validators
		Args: cobra.MaximumNArgs(1), // Accept 0 or 1 arguments (path is optional)
		RunE: analyzeProject,        // The actual function that does the work
	}

	// Add command-specific flags
	// These flags only apply to the analyze command, not globally
	analyzeCmd.Flags().StringP("output", "o", "", "Output file path for the unlaravel.json analysis contract")
	analyzeCmd.Flags().String("openapi", "", "Output file path for the generated OpenAPI 3 spec (JSON)")

	// Add the command to root
	rootCmd.AddCommand(analyzeCmd)
}

// analyzeProject handles the analyze command. The analysis LOGIC lives in
// internal/engine (ADR 0001/0004: one Project Model, many renderers) — this
// function is PURE PRESENTATION. It calls engine.Analyze to get the assembled
// Project Model, then reports what was found (deriving every count from that
// model), optionally writes the JSON contract (--output) and the OpenAPI 3 spec
// (--openapi), and renders the ER diagram and route map. The engine, not the
// CLI, owns detection, extraction, correlation, route resolution, and linking.
func analyzeProject(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	outputPath, _ := cmd.Flags().GetString("output")
	openAPIPath, _ := cmd.Flags().GetString("openapi")

	// Determine the project path (default: current directory).
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)

	cyan.Printf("🔍 Analyzing Laravel project: %s\n\n", projectPath)

	if verbose {
		yellow.Println("📋 Analysis options:")
		fmt.Printf("  • Output path: %s\n", outputPath)
		fmt.Printf("  • OpenAPI path: %s\n", openAPIPath)
		fmt.Println()
	}

	// Run the full analysis pipeline (detect → schema → models → disagreements →
	// routes(two-phase) → formrequests → link → build). All the logic — and its
	// error wrapping — lives in the engine; the CLI only presents the result.
	pm, err := engine.Analyze(projectPath)
	if err != nil {
		return err
	}

	// Report what the engine found, deriving every count from the returned model.
	green.Printf("✅ Laravel project detected (version: %s)\n", displayVersion(pm.LaravelVersion))
	green.Printf("✅ Extracted %d table(s)\n", len(pm.Schemas))
	green.Printf("✅ Extracted %d Eloquent model(s)\n", len(pm.Models))
	green.Printf("✅ Extracted %d controller(s)\n", len(pm.Controllers))
	green.Printf("✅ Extracted %d route(s)\n", len(pm.Routes))

	// Optionally write the JSON output contract.
	if outputPath != "" {
		if err := writeProjectModel(pm, outputPath); err != nil {
			return err
		}
		green.Printf("✅ Wrote analysis to %s\n", outputPath)
	}

	// Optionally write the OpenAPI 3 spec. It is explicit: --openapi must be given
	// for it to be emitted (it is never inferred from --output), so the two
	// artifacts are requested independently.
	if openAPIPath != "" {
		if err := writeOpenAPI(pm, openAPIPath); err != nil {
			return err
		}
		green.Printf("✅ Wrote OpenAPI 3 spec to %s\n", openAPIPath)
	}

	// Report the Disagreement findings, if any.
	reportDisagreements(pm.Disagreements, green, yellow)

	// Render and print the ER diagram (FK + Eloquent relationship lines).
	yellow.Println("\n📊 Entity-Relationship diagram (Mermaid):")
	fmt.Println(er.Render(pm))

	// Render and print the route map, then the dead-route findings.
	yellow.Printf("\n🗺️  Routes (%d):\n", len(pm.Routes))
	fmt.Println(routemap.Render(pm))
	reportDeadRoutes(pm.DeadRoutes, green, yellow)

	// Report the FormRequest count (the sixth and final MVP node type).
	green.Printf("✅ FormRequests: %d\n", len(pm.FormRequests))

	cyan.Printf("🎉 Analysis complete: %d table(s), %d model(s), %d route(s), %d form request(s) found in %s\n",
		len(pm.Schemas), len(pm.Models), len(pm.Routes), len(pm.FormRequests), projectPath)
	return nil
}

// reportDisagreements prints the Disagreement findings. With no findings it
// prints a single reassuring line; otherwise it prints a warning header and one
// line per finding, naming the Model, its relationship, and the reason. It makes
// no claim beyond what the correlation found.
func reportDisagreements(disagreements []model.Disagreement, green, yellow *color.Color) {
	if len(disagreements) == 0 {
		green.Println("✅ No Model↔Schema disagreements found.")
		return
	}

	yellow.Printf("\n⚠️  Disagreements (%d):\n", len(disagreements))
	for _, d := range disagreements {
		yellow.Printf("  • %s::%s — %s\n", d.Model, d.Relationship, d.Reason)
	}
}

// reportDeadRoutes prints the Dead Route findings (ADR 0006): routes whose
// Controller or Action edge dangles. With no findings it prints a single
// reassuring line; otherwise it prints a warning header and one line per finding,
// naming the route's method and URI and the reason it could not be resolved. It
// makes no claim beyond what phase-two resolution found.
func reportDeadRoutes(deadRoutes []model.DeadRoute, green, yellow *color.Color) {
	if len(deadRoutes) == 0 {
		green.Println("✅ No dead routes found.")
		return
	}

	yellow.Printf("\n⚠️  Dead routes (%d):\n", len(deadRoutes))
	for _, d := range deadRoutes {
		yellow.Printf("  • %s %s — %s\n", d.Method, d.URI, d.Reason)
	}
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

// writeOpenAPI renders the Project Model to an OpenAPI 3 spec (routes +
// FormRequest bodies, ADR 0004) and writes it to openAPIPath. The renderer reads
// ONLY the model, so this depends on the FormRequest links already being set on
// the routes.
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
