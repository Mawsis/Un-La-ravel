package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/mawsis/unlaravel/internal/analyze"
	"github.com/mawsis/unlaravel/internal/detector"
	modelextract "github.com/mawsis/unlaravel/internal/extract/model"
	"github.com/mawsis/unlaravel/internal/extract/schema"
	"github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/render/er"
)

// migrationsSubdir is the conventional location of Laravel migration files,
// relative to the project root.
var migrationsSubdir = filepath.Join("database", "migrations")

// modelsSubdir is the conventional location of Eloquent model classes in a
// modern Laravel layout, relative to the project root. appSubdir is the classic
// layout where models live directly under app/. Both are scanned (the extractor
// decides which files are actually Eloquent models).
var (
	modelsSubdir = filepath.Join("app", "Models")
	appSubdir    = "app"
)

// phpGlob matches PHP source files within a directory.
const phpGlob = "*.php"

// rootCmd represents the base command when called without any subcommands
// In Cobra, commands are organized in a tree structure with a root command at the top
var rootCmd = &cobra.Command{
	Use:   "unlaravel",
	Short: "Un(la)ravel - Laravel project analysis tool",
	Long: color.New(color.FgCyan).Sprint(`
🔍 Un(la)ravel - Laravel Project Analysis Tool

Un(la)ravel helps you understand Laravel projects by providing deep insights into:
• Route analysis and auto-generated Swagger documentation
• Database schema analysis and ER diagrams  
• Request/Resource/Policy mapping
• Performance analysis and optimization suggestions
• Security analysis and vulnerability detection

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
	rootCmd.PersistentFlags().String("config", "", "Config file path (default is $HOME/.unlaravel.yaml)")

	// Add subcommands to the root command
	// We'll create these commands in separate files for better organization
	addAnalyzeCommand()
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
• Routes and middleware chains
• Database schema and relationships
• Model dependencies and policies
• Performance bottlenecks
• Security configurations`,
		// Args validation - Cobra provides several built-in validators
		Args: cobra.MaximumNArgs(1), // Accept 0 or 1 arguments (path is optional)
		RunE: analyzeProject,        // The actual function that does the work
	}

	// Add command-specific flags
	// These flags only apply to the analyze command, not globally
	analyzeCmd.Flags().BoolP("routes", "r", false, "Analyze routes only")
	analyzeCmd.Flags().BoolP("database", "d", false, "Analyze database schema only")
	analyzeCmd.Flags().BoolP("swagger", "s", false, "Generate Swagger documentation")
	analyzeCmd.Flags().StringP("output", "o", "", "Output file path for analysis results")

	// Add the command to root
	rootCmd.AddCommand(analyzeCmd)
}

// analyzeProject handles the analyze command. It runs the real analysis
// pipeline for this slice — Schema extraction plus Eloquent Model extraction
// (ADR scope) — and reports exactly what happened, with no fabricated success
// messages.
//
// Pipeline:
//  1. Detect the Laravel project (artisan + composer.json) via the detector.
//  2. Locate database/migrations and extract Table nodes from their AST.
//  3. Locate app/Models (and classic app/) and extract Eloquent Model nodes
//     with their relationships from their AST.
//  4. Correlate Models against the Schema to surface Disagreements — a
//     relationship referencing a table or foreign-key column the Schema lacks.
//  5. Assemble a model.ProjectModel from the detector + tables + models +
//     disagreements.
//  6. Optionally write the JSON contract (--output), then render the ER diagram
//     (which now includes Eloquent relationship lines alongside the FK lines).
func analyzeProject(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	routesOnly, _ := cmd.Flags().GetBool("routes")
	generateSwagger, _ := cmd.Flags().GetBool("swagger")
	outputPath, _ := cmd.Flags().GetString("output")

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
		fmt.Printf("  • Routes only: %v\n", routesOnly)
		fmt.Printf("  • Generate Swagger: %v\n", generateSwagger)
		fmt.Printf("  • Output path: %s\n", outputPath)
		fmt.Println()
	}

	// Honest stubs for capabilities not yet implemented in this slice.
	if routesOnly {
		return fmt.Errorf("route analysis is not yet implemented in this build")
	}
	if generateSwagger {
		yellow.Println("⚠️  Swagger generation is not yet implemented; skipping.")
	}

	// 1. Detect the Laravel project.
	project, err := detector.DetectLaravel(projectPath)
	if err != nil {
		return fmt.Errorf("not a Laravel project (%s): %w", projectPath, err)
	}
	green.Printf("✅ Laravel project detected (version: %s)\n", displayVersion(project.Version))

	// 2. Extract the database schema from migrations.
	tables, err := extractSchema(projectPath, green, yellow)
	if err != nil {
		return err
	}

	// 3. Extract the Eloquent models from app/Models (and classic app/).
	models, err := extractModels(projectPath, green, yellow)
	if err != nil {
		return err
	}

	// 4. Correlate Models against the Schema to surface Disagreements.
	disagreements := analyze.FindDisagreements(models, tables)

	// 5. Assemble the Project Model.
	pm := buildProjectModel(project, tables, models, disagreements)

	// 6a. Optionally write the JSON output contract.
	if outputPath != "" {
		if err := writeProjectModel(pm, outputPath); err != nil {
			return err
		}
		green.Printf("✅ Wrote analysis to %s\n", outputPath)
	}

	// 6b. Report the Disagreement findings, if any.
	reportDisagreements(disagreements, green, yellow)

	// 6c. Render and print the ER diagram (FK + Eloquent relationship lines).
	yellow.Println("\n📊 Entity-Relationship diagram (Mermaid):")
	fmt.Println(er.Render(pm))

	cyan.Printf("🎉 Analysis complete: %d table(s), %d model(s) found in %s\n",
		len(tables), len(models), projectPath)
	return nil
}

// extractSchema locates the migrations directory under projectPath and extracts
// the declared tables. A missing or empty migrations directory is reported
// honestly (and returns no tables) rather than treated as a failure.
func extractSchema(projectPath string, green, yellow *color.Color) ([]model.Table, error) {
	migrationsDir := filepath.Join(projectPath, migrationsSubdir)

	info, err := os.Stat(migrationsDir)
	if err != nil || !info.IsDir() {
		yellow.Printf("⚠️  No migrations directory found at %s; no schema to analyze.\n", migrationsDir)
		return nil, nil
	}

	tables, err := schema.ExtractDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract schema from %s: %w", migrationsDir, err)
	}

	if len(tables) == 0 {
		yellow.Printf("⚠️  No tables found in %s (migrations directory is empty or declares no tables).\n", migrationsDir)
		return nil, nil
	}

	green.Printf("✅ Extracted %d table(s) from %s\n", len(tables), migrationsDir)
	return tables, nil
}

// extractModels locates the Eloquent model files under projectPath and extracts
// the Model nodes (with their relationships) from their AST. Both the modern
// app/Models layout and the classic app/ layout are scanned; the extractor
// itself decides which PHP files are actually Eloquent models, so non-model
// files (and missing directories) are reported honestly and yield no models
// rather than an error.
//
// Files from both directories are gathered, de-duplicated, and sorted so
// discovery order is deterministic regardless of which layout a project uses.
func extractModels(projectPath string, green, yellow *color.Color) ([]model.Model, error) {
	paths, err := collectModelFiles(projectPath)
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		yellow.Printf("⚠️  No PHP files found under %s; no Eloquent models to analyze.\n",
			filepath.Join(projectPath, appSubdir))
		return nil, nil
	}

	models, err := modelextract.Extract(paths)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Eloquent models: %w", err)
	}

	green.Printf("✅ Extracted %d Eloquent model(s)\n", len(models))
	return models, nil
}

// collectModelFiles gathers the candidate PHP files for model extraction from
// both the modern app/Models directory and the classic app/ directory under
// projectPath. Each directory is scanned non-recursively for *.php; a missing
// directory is skipped silently (it simply contributes no files). The returned
// paths are de-duplicated and sorted lexically so discovery order is
// deterministic.
func collectModelFiles(projectPath string) ([]string, error) {
	dirs := []string{
		filepath.Join(projectPath, modelsSubdir),
		filepath.Join(projectPath, appSubdir),
	}

	seen := make(map[string]struct{})
	var paths []string
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, phpGlob))
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s for PHP files: %w", dir, err)
		}
		for _, p := range matches {
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}

	sort.Strings(paths)
	return paths, nil
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

// buildProjectModel assembles a Project Model from the detected project and the
// extracted tables, models, and disagreements, choosing the best available
// project name. Insertion order is preserved for deterministic output.
func buildProjectModel(
	project *detector.LaravelProject,
	tables []model.Table,
	models []model.Model,
	disagreements []model.Disagreement,
) *model.ProjectModel {
	pm := model.New(projectName(project), project.Version)
	for _, t := range tables {
		pm.AddTable(t)
	}
	for _, m := range models {
		pm.AddModel(m)
	}
	for _, d := range disagreements {
		pm.AddDisagreement(d)
	}
	return pm
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

// projectName returns a human-readable project name, preferring the composer
// "name" field and falling back to the project's directory name.
func projectName(project *detector.LaravelProject) string {
	if project.ComposerAnalysis != nil && project.ComposerAnalysis.ProjectName != "" {
		return project.ComposerAnalysis.ProjectName
	}
	abs, err := filepath.Abs(project.Path)
	if err == nil {
		return filepath.Base(abs)
	}
	return filepath.Base(project.Path)
}

// displayVersion returns a placeholder when the detected version is empty so
// output never shows a blank version.
func displayVersion(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}
