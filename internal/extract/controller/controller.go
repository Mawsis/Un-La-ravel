package controller

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	domain "github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/phpast"
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
	var controllers []domain.Controller

	for _, path := range paths {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("controller: extract %q: %w", path, err)
		}

		namespace := phpast.NamespaceName(res.Root)

		v := newControllerVisitor()
		phpast.Walk(res.Root, v)

		for _, cb := range v.classes {
			controllers = append(controllers, buildController(namespace, cb))
		}
	}

	return controllers, nil
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
	paths, err := discoverPHPFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("controller: discover controllers dir %q: %w", dir, err)
	}
	sort.Strings(paths)

	return Extract(paths)
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
