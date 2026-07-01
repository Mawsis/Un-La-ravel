package controller

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	domain "github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// Package controller extracts Controller nodes (ADR 0001) from a Laravel
// project's app/Http/Controllers directory. Each PHP file may declare one or
// more classes; one domain.Controller is emitted per class, carrying its fully
// qualified name (the file namespace joined to the class short name) and the
// public method names that can serve as routable Actions.
//
// The FQNs produced here are the keys the two-phase symbol table (ADR 0006)
// resolves Route controller references against during phase two — so this
// extractor is Phase-1 input for dead-route detection, not the resolver itself.

// controllerGlob matches PHP source files within a controllers directory.
const controllerGlob = "*.php"

// namespaceSeparator is the backslash that joins a file's namespace to a class's
// short name to form its fully qualified name (ADR 0006), for example
// "App\Http\Controllers" + "\" + "PostController".
const namespaceSeparator = `\`

// Extract parses each PHP file at the given paths and returns the Controller
// nodes they declare, as domain.Controller values in first-discovery order with
// actions in source-declaration order.
//
// A file may declare several classes; one domain.Controller is emitted per
// class. Every named top-level class is emitted — the extractor does NOT filter
// by base class, because Laravel controllers extend the app's own base
// controller (or, when invokable, nothing at all), and phase-two resolution
// (ADR 0006) decides relevance by matching a Route's controller reference to
// these FQNs. A class with no public actions is emitted with an empty (non-nil)
// Actions slice.
//
// Each controller's FQN is the file's namespace (via phpast.NamespaceName)
// joined to the class short name with a backslash; a class in the global
// namespace yields the bare class name as its FQN. Its Actions are the public,
// non-constructor method names (see isPublicMethod): a method with an explicit
// `public` modifier or no visibility modifier qualifies, while private,
// protected, and constructor methods are excluded. __invoke is included as the
// action of a single-action controller.
//
// Files are processed in the order given, so callers control discovery order by
// sorting paths. The same class name appearing in two files yields two
// Controller values; this slice does no project-wide deduplication (the symbol
// table that correlates by FQN is assembled by the caller, per ADR 0006).
//
// Errors: a file that cannot be read or catastrophically fails to parse aborts
// the whole extraction with a wrapped error, because a missing or unreadable
// controller file would silently drop FQNs the symbol table needs and turn valid
// routes into false dead-route findings. Recoverable per-file syntax diagnostics
// do NOT abort — the parser is fault-tolerant (ADR 0003).
func Extract(paths []string) ([]domain.Controller, error) {
	controllers, _, err := ExtractWithParams(paths)
	return controllers, err
}

// ActionParams maps a controller FQN to that controller's action-parameter
// type-hints: action name → the type-hint short names of that action's typed
// parameters, in declaration order (untyped parameters such as `$id` are
// omitted, see phpast.ParamTypeNames). Only actions with at least one typed
// parameter appear; a controller with no such actions is absent from the map.
//
// This is deliberately side data, kept OUT of domain.Controller's JSON contract
// (ADR 0004): the shipped unlaravel.json shape is unchanged. The analyze step
// (ADR 0006) consumes it to link a Route to a FormRequest — it resolves each
// short type name through the controller file's `use` imports + the symbol
// table, and when a resolved type is a known FormRequest FQN, the route that
// dispatches to that action gains the FormRequest as its request body.
//
// The FQN key matches domain.Controller.FQN exactly (file namespace joined to
// the class short name), so callers correlate the two by FQN. The inner map is
// keyed by the same action names that appear in the Controller's Actions slice.
type ActionParams map[string]map[string][]string

// ExtractWithParams parses each PHP file at the given paths and returns both the
// Controller nodes they declare (identical to Extract) AND an ActionParams side
// map carrying each action's parameter type-hints for the FormRequest↔Route link
// (ADR 0006). The Controller slice's JSON contract is unchanged; the type-hint
// data travels alongside it rather than inside it, so unlaravel.json's shape is
// untouched (ADR 0004).
//
// Controllers are returned in the same first-discovery order, with the same
// per-file abort-on-read/parse-failure semantics, as Extract — Extract is a thin
// wrapper that discards the second return value. The returned ActionParams is
// always non-nil (empty when no action has a typed parameter).
//
// A class name that repeats across files would collide in the FQN-keyed
// ActionParams; the same caveat applies to the symbol table (ADR 0006), which
// resolves Route references by FQN, so this map is intended to be consumed
// through that same FQN correlation and does no project-wide deduplication.
func ExtractWithParams(paths []string) ([]domain.Controller, ActionParams, error) {
	var controllers []domain.Controller
	actionParams := make(ActionParams)

	for _, path := range paths {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("controller: extract %q: %w", path, err)
		}

		namespace := phpast.NamespaceName(res.Root)

		v := newControllerVisitor()
		phpast.Walk(res.Root, v)

		for _, cb := range v.classes {
			controllers = append(controllers, buildController(namespace, cb))
			if len(cb.actionParams) > 0 {
				actionParams[qualify(namespace, cb.name)] = cb.actionParams
			}
		}
	}

	return controllers, actionParams, nil
}

// ExtractDir discovers the PHP files under dir (recursively — Laravel nests
// controllers in subdirectories such as Http/Controllers/Admin), sorts them
// lexically for deterministic discovery order, and runs Extract. A dir that does
// not exist or cannot be walked yields a wrapped error.
//
// Recursion is the key difference from the model extractor's flat ExtractDir:
// controller subdirectories carry namespace segments (Admin\PostController), and
// dropping them would break the FQNs the symbol table resolves against.
func ExtractDir(dir string) ([]domain.Controller, error) {
	controllers, _, err := ExtractDirWithParams(dir)
	return controllers, err
}

// ExtractDirWithParams is ExtractDir paired with ExtractWithParams: it discovers
// the PHP files under dir recursively, sorts them for deterministic discovery
// order, and returns both the Controller nodes and the ActionParams side map
// (see ExtractWithParams) for the FormRequest↔Route link. ExtractDir is a thin
// wrapper that discards the ActionParams.
func ExtractDirWithParams(dir string) ([]domain.Controller, ActionParams, error) {
	paths, err := discoverPHPFiles(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("controller: discover controllers dir %q: %w", dir, err)
	}
	sort.Strings(paths)

	return ExtractWithParams(paths)
}

// discoverPHPFiles walks dir recursively and returns the paths of every PHP
// source file beneath it. Directories are descended into; non-.php files are
// skipped. The returned paths are unsorted — callers sort for determinism.
func discoverPHPFiles(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ok, _ := filepath.Match(controllerGlob, d.Name()); ok {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// buildController converts a mutable classBuilder into an immutable
// domain.Controller. The FQN is formed here by joining the file namespace to the
// class short name, and the actions are copied into the controller's non-nil
// slice so a controller with no public methods serializes as "actions": []
// rather than null.
func buildController(namespace string, cb *classBuilder) domain.Controller {
	c := domain.NewController(cb.name, qualify(namespace, cb.name))
	c.Actions = append(c.Actions, cb.actions...)
	return c
}

// qualify joins a file namespace to a class short name to form a fully qualified
// class name (ADR 0006). A class in the global namespace (empty namespace)
// yields the bare short name, matching how PHP resolves an unqualified top-level
// class.
func qualify(namespace, shortName string) string {
	if namespace == "" {
		return shortName
	}
	return namespace + namespaceSeparator + shortName
}
