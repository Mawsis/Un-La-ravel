// Package engine runs the full Un(la)ravel analysis pipeline as a single,
// CLI-free function: a project path in, an assembled *model.ProjectModel out.
//
// It is the one place the analysis LOGIC lives (ADR 0001/0004: one Project
// Model, many renderers). Both the CLI (internal/cli) and the web server are
// just consumers — they call Analyze and then render the SAME model however
// they like. Nothing here prints, reads flags, or writes files: it has no
// dependency on cobra, github.com/fatih/color, or os.WriteFile, so any caller
// can drive it headlessly and get identical results.
//
// The pipeline runs in a fixed order (ADR 0006):
//
//	detect → schema → models → disagreements → routes (two-phase) →
//	  formrequests → link → build
//
// A project missing a conventional directory (database/migrations, app/Models,
// routes/, app/Http/Requests) is handled gracefully: that stage contributes
// empty results and the run continues, exactly as the CLI behaved before this
// logic was extracted.
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mawsis/Un-La-ravel/internal/analyze"
	"github.com/Mawsis/Un-La-ravel/internal/detector"
	"github.com/Mawsis/Un-La-ravel/internal/extract/controller"
	formrequestextract "github.com/Mawsis/Un-La-ravel/internal/extract/formrequest"
	modelextract "github.com/Mawsis/Un-La-ravel/internal/extract/model"
	routeextract "github.com/Mawsis/Un-La-ravel/internal/extract/route"
	"github.com/Mawsis/Un-La-ravel/internal/extract/schema"
	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/Mawsis/Un-La-ravel/internal/symbol"
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

// requestsSubdir is the conventional location of Laravel FormRequest classes,
// relative to the project root. It is scanned RECURSIVELY (like the controllers
// directory), because a request under app/Http/Requests/Admin carries its
// subdirectory as a namespace segment. A project without this directory simply
// has no FormRequests to analyze — the pipeline reports zero and continues.
var requestsSubdir = filepath.Join("app", "Http", "Requests")

// phpGlob matches PHP source files within a directory.
const phpGlob = "*.php"

// Analyze runs the full analysis pipeline across all six MVP node types —
// Schema, Eloquent Model, Route, Controller, Middleware, and FormRequest — for
// the Laravel project rooted at projectPath, returning the assembled Project
// Model. It performs no I/O beyond reading the project's source files and prints
// nothing: it is the reusable core both the CLI and the web server call.
//
// Pipeline:
//  1. Detect the Laravel project (artisan + composer.json) via the detector.
//  2. Locate database/migrations and extract Table nodes from their AST.
//  3. Locate app/Models (and classic app/) and extract Eloquent Model nodes
//     with their relationships from their AST.
//  4. Correlate Models against the Schema to surface Disagreements — a
//     relationship referencing a table or foreign-key column the Schema lacks.
//  5. Run the Route pipeline in two phases (ADR 0006): collect controllers (with
//     their action-parameter type-hints) and build the symbol table seeded with
//     controllers, models, and FormRequests (Phase 1), then extract routes and
//     resolve them against that table to fill in controller FQNs and surface
//     Dead Routes — routes whose Controller/Action edge dangles (Phase 2).
//  6. Extract FormRequest nodes from app/Http/Requests and link each resolved
//     route to the request body its action validates (ADR 0006), setting
//     Route.FormRequest.
//  7. Assemble a model.ProjectModel from the detector + tables + models +
//     disagreements + routes + controllers + dead routes + form requests.
//
// A missing routes/ or app/Http/Requests directory (and likewise a missing
// migrations or models directory) yields empty results for that stage rather
// than an error; the run continues.
func Analyze(projectPath string) (*model.ProjectModel, error) {
	// 1. Detect the Laravel project.
	project, err := detector.DetectLaravel(projectPath)
	if err != nil {
		return nil, fmt.Errorf("not a Laravel project (%s): %w", projectPath, err)
	}

	// 2. Extract the database schema from migrations.
	tables, err := extractSchema(projectPath)
	if err != nil {
		return nil, err
	}

	// 3. Extract the Eloquent models from app/Models (and classic app/).
	models, err := extractModels(projectPath)
	if err != nil {
		return nil, err
	}

	// 4. Correlate Models against the Schema to surface Disagreements.
	disagreements := analyze.FindDisagreements(models, tables)

	// 5. Run the two-phase Route pipeline (ADR 0006): controllers + symbol table
	//    (seeded with the models AND the FormRequest classes so a request-typed
	//    parameter resolves, per ADR 0006), then routes resolved against them,
	//    yielding resolved routes and dead routes. The pipeline also returns the
	//    controllers' action-parameter type-hints, the FQN→file map, and the
	//    symbol table — the inputs the FormRequest link needs.
	rp, err := extractRoutes(projectPath, models)
	if err != nil {
		return nil, err
	}

	// 6. Extract the FormRequest nodes from app/Http/Requests, then link each
	//    resolved route to the request body its action validates (ADR 0006).
	formRequests, err := extractFormRequests(projectPath)
	if err != nil {
		return nil, err
	}
	routes := analyze.LinkFormRequests(rp.routes, formRequests, rp.actionParams, rp.symbols, rp.controllerFiles)

	// 7. Assemble the Project Model.
	return buildProjectModel(project, tables, models, disagreements, routes, rp.controllers, rp.deadRoutes, formRequests), nil
}

// routePipeline bundles the artifacts the two-phase Route pipeline produces that
// downstream steps consume: the resolved routes, the extracted controllers, the
// dead-route findings, and — for the FormRequest link (ADR 0006) — the
// controllers' action-parameter type-hints, the project-wide symbol table, and
// the controller-FQN → source-file map that lets a parameter's short type name
// resolve against the imports of the file that declared its action.
type routePipeline struct {
	routes          []model.Route
	controllers     []model.Controller
	deadRoutes      []model.DeadRoute
	actionParams    controller.ActionParams
	symbols         *symbol.Table
	controllerFiles map[string]string
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
// A project with no routes directory yields no routes, controllers, or findings
// rather than an error — so analysis of a project without a routes/ directory
// continues instead of crashing. Read or catastrophic parse failures during
// collection or resolution abort with a wrapped error, because a controller or
// route file that goes unread would silently mis-resolve real edges into dead
// ones.
func extractRoutes(projectPath string, models []model.Model) (routePipeline, error) {
	routesDir := filepath.Join(projectPath, routesSubdir)

	info, err := os.Stat(routesDir)
	if err != nil || !info.IsDir() {
		return routePipeline{}, nil
	}

	// Phase 1a: extract Controller classes (recursively — subdirectories are
	// namespace segments) from app/Http/Controllers, together with each action's
	// parameter type-hints (the ActionParams side map feeding the FormRequest
	// link, ADR 0006) and the FQN→file map its resolution needs.
	controllersDir := filepath.Join(projectPath, controllersSubdir)
	controllers, actionParams, err := controller.ExtractDirWithParams(controllersDir)
	if err != nil {
		return routePipeline{}, fmt.Errorf("failed to extract controllers from %s: %w", controllersDir, err)
	}

	controllerFiles, err := controllerFileMap(controllersDir)
	if err != nil {
		return routePipeline{}, err
	}

	// Phase 1b: build the symbol table from every relevant class file — the
	// controllers plus the model files AND the FormRequest files. Including the
	// models lets a route resolve to a class outside app/Http/Controllers without
	// a false dead-route finding; including the FormRequests lets a controller
	// action's request-typed parameter resolve to a declared class so the
	// Route→FormRequest link can be made (all per ADR 0006).
	sym, err := buildSymbolTable(projectPath, controllersDir)
	if err != nil {
		return routePipeline{}, err
	}

	// Phase 2a: extract the routes (short controller names, groups flattened,
	// resource macros expanded).
	routes, err := routeextract.ExtractDir(routesDir)
	if err != nil {
		return routePipeline{}, fmt.Errorf("failed to extract routes from %s: %w", routesDir, err)
	}

	// Phase 2b: resolve routes against the controllers + symbol table, filling in
	// FQNs and surfacing dead routes.
	routeFiles, err := routeFilePaths(routesDir)
	if err != nil {
		return routePipeline{}, err
	}
	resolved, deadRoutes, err := analyze.ResolveRoutes(routes, controllers, sym, routeFiles)
	if err != nil {
		return routePipeline{}, fmt.Errorf("failed to resolve routes: %w", err)
	}

	return routePipeline{
		routes:          resolved,
		controllers:     controllers,
		deadRoutes:      deadRoutes,
		actionParams:    actionParams,
		symbols:         sym,
		controllerFiles: controllerFiles,
	}, nil
}

// controllerFileMap builds the controller-FQN → source-file map the FormRequest
// link needs (ADR 0006): a route resolves an action's request-typed parameter
// through the `use` imports of the very file that declared the action, so the
// linker must know which file that is. It walks controllersDir recursively (the
// same set the ActionParams keys come from), parses each PHP file, and records
// every top-level class it declares under its FQN (file namespace joined to the
// class short name), pointing at that file's path.
//
// A file that cannot be read or catastrophically fails to parse aborts with a
// wrapped error, mirroring the extractor: a missing entry would leave a real
// FormRequest link unresolved.
func controllerFileMap(controllersDir string) (map[string]string, error) {
	paths, err := discoverPHPFiles(controllersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan controllers for the FQN→file map: %w", err)
	}

	files := make(map[string]string, len(paths))
	for _, path := range paths {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse controller %s for the FQN→file map: %w", path, err)
		}
		namespace := phpast.NamespaceName(res.Root)
		for _, short := range phpast.DeclaredClasses(res.Root) {
			files[qualifyClass(namespace, short)] = path
		}
	}
	return files, nil
}

// qualifyClass joins a file namespace to a class short name to form a fully
// qualified class name (ADR 0006), matching the extractor and symbol-table
// convention: a class in the global namespace yields the bare short name, never
// a leading backslash.
func qualifyClass(namespace, shortName string) string {
	if namespace == "" {
		return shortName
	}
	return namespace + `\` + shortName
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

	requestFiles, err := discoverPHPFiles(filepath.Join(projectPath, requestsSubdir))
	if err != nil {
		return nil, fmt.Errorf("failed to scan FormRequests for the symbol table: %w", err)
	}

	classFiles := make([]string, 0, len(controllerFiles)+len(modelFiles)+len(requestFiles))
	classFiles = append(classFiles, controllerFiles...)
	classFiles = append(classFiles, modelFiles...)
	classFiles = append(classFiles, requestFiles...)

	sym, err := symbol.Collect(classFiles)
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
// the declared tables. A missing or empty migrations directory yields no tables
// rather than an error.
func extractSchema(projectPath string) ([]model.Table, error) {
	migrationsDir := filepath.Join(projectPath, migrationsSubdir)

	info, err := os.Stat(migrationsDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	tables, err := schema.ExtractDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract schema from %s: %w", migrationsDir, err)
	}
	return tables, nil
}

// extractModels locates the Eloquent model files under projectPath and extracts
// the Model nodes (with their relationships) from their AST. Both the modern
// app/Models layout and the classic app/ layout are scanned; the extractor
// itself decides which PHP files are actually Eloquent models, so non-model
// files (and missing directories) yield no models rather than an error.
//
// Files from both directories are gathered, de-duplicated, and sorted so
// discovery order is deterministic regardless of which layout a project uses.
func extractModels(projectPath string) ([]model.Model, error) {
	paths, err := collectModelFiles(projectPath)
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return nil, nil
	}

	models, err := modelextract.Extract(paths)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Eloquent models: %w", err)
	}
	return models, nil
}

// extractFormRequests locates the FormRequest classes under app/Http/Requests
// (scanned RECURSIVELY, since subdirectories are namespace segments) and
// extracts the FormRequest nodes — each with the request-body Fields parsed from
// its rules() method — from their AST. The extractor decides which classes
// actually extend a FormRequest base, so non-request PHP files (and a missing
// directory) yield no FormRequests rather than an error.
func extractFormRequests(projectPath string) ([]model.FormRequest, error) {
	requestsDir := filepath.Join(projectPath, requestsSubdir)

	paths, err := discoverPHPFiles(requestsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan FormRequests: %w", err)
	}

	if len(paths) == 0 {
		return nil, nil
	}

	formRequests, err := formrequestextract.Extract(paths)
	if err != nil {
		return nil, fmt.Errorf("failed to extract FormRequests: %w", err)
	}
	return formRequests, nil
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

// buildProjectModel assembles a Project Model from the detected project and the
// extracted tables, models, disagreements, routes, controllers, dead routes, and
// form requests, choosing the best available project name. Insertion order is
// preserved for deterministic output.
func buildProjectModel(
	project *detector.LaravelProject,
	tables []model.Table,
	models []model.Model,
	disagreements []model.Disagreement,
	routes []model.Route,
	controllers []model.Controller,
	deadRoutes []model.DeadRoute,
	formRequests []model.FormRequest,
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
	for _, fr := range formRequests {
		pm.AddFormRequest(fr)
	}
	return pm
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
