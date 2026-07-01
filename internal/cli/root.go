package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/mawsis/unlaravel/internal/analyze"
	"github.com/mawsis/unlaravel/internal/detector"
	"github.com/mawsis/unlaravel/internal/extract/controller"
	modelextract "github.com/mawsis/unlaravel/internal/extract/model"
	routeextract "github.com/mawsis/unlaravel/internal/extract/route"
	"github.com/mawsis/unlaravel/internal/extract/schema"
	"github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/render/er"
	"github.com/mawsis/unlaravel/internal/render/routemap"
	"github.com/mawsis/unlaravel/internal/symbol"
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

// controllersSubdir is the conventional location of Laravel controller classes,
// relative to the project root. It is scanned RECURSIVELY (via
// controller.ExtractDir), because a controller under
// app/Http/Controllers/Admin carries its subdirectory as a namespace segment.
var controllersSubdir = filepath.Join("app", "Http", "Controllers")

// routesSubdir is the conventional location of Laravel route files
// (routes/web.php, routes/api.php, ...), relative to the project root. A project
// without this directory simply has no routes to analyze — the pipeline reports
// zero and continues.
var routesSubdir = "routes"

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
// pipeline for this slice — Schema extraction, Eloquent Model extraction, and
// the two-phase Route→Controller resolution (ADR scope) — and reports exactly
// what happened, with no fabricated success messages.
//
// Pipeline:
//  1. Detect the Laravel project (artisan + composer.json) via the detector.
//  2. Locate database/migrations and extract Table nodes from their AST.
//  3. Locate app/Models (and classic app/) and extract Eloquent Model nodes
//     with their relationships from their AST.
//  4. Correlate Models against the Schema to surface Disagreements — a
//     relationship referencing a table or foreign-key column the Schema lacks.
//  5. Run the Route pipeline in two phases (ADR 0006): collect controllers and
//     build the symbol table (Phase 1), then extract routes and resolve them
//     against that table to fill in controller FQNs and surface Dead Routes —
//     routes whose Controller/Action edge dangles (Phase 2).
//  6. Assemble a model.ProjectModel from the detector + tables + models +
//     disagreements + routes + controllers + dead routes.
//  7. Optionally write the JSON contract (--output), then render the ER diagram
//     (which includes Eloquent relationship lines alongside the FK lines) and
//     the route map.
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

	// Honest stubs for capabilities not yet implemented in this slice. The
	// --routes flag is accepted but, in this slice, does not yet narrow the
	// pipeline to routes only; the full analysis (which includes routes) runs
	// regardless.
	if routesOnly {
		yellow.Println("⚠️  --routes does not yet narrow output; running the full analysis (which includes routes).")
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

	// 5. Run the two-phase Route pipeline (ADR 0006): controllers + symbol table,
	//    then routes resolved against them, yielding resolved routes and dead
	//    routes.
	routes, controllers, deadRoutes, err := extractRoutes(projectPath, models, green, yellow)
	if err != nil {
		return err
	}

	// 6. Assemble the Project Model.
	pm := buildProjectModel(project, tables, models, disagreements, routes, controllers, deadRoutes)

	// 7a. Optionally write the JSON output contract.
	if outputPath != "" {
		if err := writeProjectModel(pm, outputPath); err != nil {
			return err
		}
		green.Printf("✅ Wrote analysis to %s\n", outputPath)
	}

	// 7b. Report the Disagreement findings, if any.
	reportDisagreements(disagreements, green, yellow)

	// 7c. Render and print the ER diagram (FK + Eloquent relationship lines).
	yellow.Println("\n📊 Entity-Relationship diagram (Mermaid):")
	fmt.Println(er.Render(pm))

	// 7d. Render and print the route map, then the dead-route findings.
	yellow.Printf("\n🗺️  Routes (%d):\n", len(routes))
	fmt.Println(routemap.Render(pm))
	reportDeadRoutes(deadRoutes, green, yellow)

	cyan.Printf("🎉 Analysis complete: %d table(s), %d model(s), %d route(s) found in %s\n",
		len(tables), len(models), len(routes), projectPath)
	return nil
}

// extractRoutes runs the two-phase Route pipeline (ADR 0006) under projectPath.
//
// Phase 1 (collect): it recursively scans app/Http/Controllers for Controller
// classes (with their public Actions) and builds the project-wide symbol table
// from every relevant class file — the controllers plus the already-extracted
// Eloquent models — so a route's controller reference can be checked against the
// full set of declared classes, not only those under app/Http/Controllers.
//
// Phase 2 (resolve): it extracts the routes from routes/*.php (short controller
// names, group prefixes applied, resource macros expanded) and resolves them
// against the symbol table and controllers, filling in each resolvable route's
// controller FQN and returning a Dead Route finding for every route whose
// Controller/Action edge dangles.
//
// A project with no routes directory is reported honestly and yields no routes,
// controllers, or findings rather than an error — so analysis of a project
// without a routes/ directory continues instead of crashing. Read or catastrophic
// parse failures during collection or resolution abort with a wrapped error,
// because a controller or route file that goes unread would silently mis-resolve
// real edges into dead ones.
func extractRoutes(
	projectPath string,
	models []model.Model,
	green, yellow *color.Color,
) ([]model.Route, []model.Controller, []model.DeadRoute, error) {
	routesDir := filepath.Join(projectPath, routesSubdir)

	info, err := os.Stat(routesDir)
	if err != nil || !info.IsDir() {
		yellow.Printf("⚠️  No routes directory found at %s; no routes to analyze.\n", routesDir)
		return nil, nil, nil, nil
	}

	// Phase 1a: extract Controller classes (recursively — subdirectories are
	// namespace segments) from app/Http/Controllers.
	controllersDir := filepath.Join(projectPath, controllersSubdir)
	controllers, err := controller.ExtractDir(controllersDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to extract controllers from %s: %w", controllersDir, err)
	}
	green.Printf("✅ Extracted %d controller(s) from %s\n", len(controllers), controllersDir)

	// Phase 1b: build the symbol table from every relevant class file — the
	// controllers plus the model files (models are cheap to include and let a
	// route resolve to a class outside app/Http/Controllers without a false
	// dead-route finding, per ADR 0006).
	sym, err := buildSymbolTable(projectPath, controllersDir)
	if err != nil {
		return nil, nil, nil, err
	}

	// Phase 2a: extract the routes (short controller names, groups flattened,
	// resource macros expanded).
	routes, err := routeextract.ExtractDir(routesDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to extract routes from %s: %w", routesDir, err)
	}

	// Phase 2b: resolve routes against the controllers + symbol table, filling in
	// FQNs and surfacing dead routes.
	routeFiles, err := routeFilePaths(routesDir)
	if err != nil {
		return nil, nil, nil, err
	}
	resolved, deadRoutes, err := analyze.ResolveRoutes(routes, controllers, sym, routeFiles)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to resolve routes: %w", err)
	}

	green.Printf("✅ Extracted %d route(s) from %s\n", len(resolved), routesDir)
	return resolved, controllers, deadRoutes, nil
}

// buildSymbolTable builds the project-wide symbol table (ADR 0006, Phase 1) from
// every relevant declared class file: the controllers under controllersDir
// (scanned recursively, since subdirectories are namespace segments) plus the
// Eloquent model files. Including the model files lets a route that dispatches to
// a class outside app/Http/Controllers resolve to a real declared class rather
// than being reported as a false dead route.
//
// A file that cannot be read or catastrophically fails to parse aborts with a
// wrapped error: an omitted class would silently turn a real edge into a dead
// one.
func buildSymbolTable(projectPath, controllersDir string) (*symbol.Table, error) {
	controllerFiles, err := discoverPHPFiles(controllersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan controllers for the symbol table: %w", err)
	}

	modelFiles, err := collectModelFiles(projectPath)
	if err != nil {
		return nil, err
	}

	sym, err := symbol.Collect(append(controllerFiles, modelFiles...))
	if err != nil {
		return nil, fmt.Errorf("failed to build symbol table: %w", err)
	}
	return sym, nil
}

// discoverPHPFiles walks dir recursively and returns the paths of every *.php
// file under it, sorted lexically for deterministic collection order. A missing
// directory yields no files (not an error), so a project without the directory
// simply contributes nothing to the symbol table.
func discoverPHPFiles(dir string) ([]string, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, nil
	}

	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".php") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk %s: %w", dir, err)
	}

	sort.Strings(paths)
	return paths, nil
}

// routeFilePaths returns the *.php route files under routesDir, sorted lexically,
// as the resolution context for ResolveRoutes (their merged `use` imports qualify
// each route's short controller name, ADR 0006).
func routeFilePaths(routesDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(routesDir, phpGlob))
	if err != nil {
		return nil, fmt.Errorf("failed to scan %s for route files: %w", routesDir, err)
	}
	sort.Strings(matches)
	return matches, nil
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

// buildProjectModel assembles a Project Model from the detected project and the
// extracted tables, models, disagreements, routes, controllers, and dead routes,
// choosing the best available project name. Insertion order is preserved for
// deterministic output.
func buildProjectModel(
	project *detector.LaravelProject,
	tables []model.Table,
	models []model.Model,
	disagreements []model.Disagreement,
	routes []model.Route,
	controllers []model.Controller,
	deadRoutes []model.DeadRoute,
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
	for _, c := range controllers {
		pm.AddController(c)
	}
	for _, r := range routes {
		pm.AddRoute(r)
	}
	for _, dr := range deadRoutes {
		pm.AddDeadRoute(dr)
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
