package symbol

import (
	"path/filepath"
	"strings"
	"testing"
)

// The file-based fixtures under testdata/ mirror the inline sources in
// symbol_test.go but drive the Phase-1 file loaders (AddFile / Collect) rather
// than Add. They prove the loaders parse real files, key each file's import map
// by its own path, and — the ADR 0006 anti-wrong-edge property — resolve two
// classes sharing a short name to different FQNs, chosen solely by the importing
// route file. Paths are kept as constants so a fixture rename surfaces here.
const (
	fixturePostController      = "post_controller.php"
	fixtureAdminPostController = "admin_post_controller.php"
	fixtureRoutesImportBase    = "routes_importing_base.php"
	fixtureRoutesImportAdmin   = "routes_importing_admin.php"
	fixtureRoutesNoImport      = "routes_no_import.php"
)

const (
	fqnBasePostController  = `App\Http\Controllers\PostController`
	fqnAdminPostController = `App\Http\Controllers\Admin\PostController`
	defaultControllerNS    = `App\Http\Controllers`
)

// td returns the testdata path for a fixture file name.
func td(name string) string { return filepath.Join("testdata", name) }

func TestAddFileRecordsDeclarationsAndImports(t *testing.T) {
	tbl := New()
	if err := tbl.AddFile(td(fixturePostController)); err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if err := tbl.AddFile(td(fixtureRoutesImportBase)); err != nil {
		t.Fatalf("AddFile: %v", err)
	}

	if !tbl.IsDeclared(fqnBasePostController) {
		t.Fatalf("AddFile did not record the declared class %q", fqnBasePostController)
	}

	fqn, found := tbl.Resolve("PostController", td(fixtureRoutesImportBase))
	if fqn != fqnBasePostController || !found {
		t.Fatalf("Resolve after AddFile = (%q, %v), want (%q, true)", fqn, found, fqnBasePostController)
	}
}

func TestAddFileMissingPathIsWrappedError(t *testing.T) {
	tbl := New()
	err := tbl.AddFile(td("does_not_exist.php"))
	if err == nil {
		t.Fatalf("AddFile of a missing path must return an error")
	}
	// The error is wrapped with the package prefix and the offending path so a
	// caller can tell which file broke collection (%w chain, ADR 0003).
	if !strings.Contains(err.Error(), "symbol: add file") {
		t.Fatalf("error %q missing the wrapped context prefix", err)
	}
	if !strings.Contains(err.Error(), "does_not_exist.php") {
		t.Fatalf("error %q should name the offending path", err)
	}
}

func TestCollectBuildsTableFromPaths(t *testing.T) {
	tbl, err := Collect([]string{
		td(fixturePostController),
		td(fixtureRoutesImportBase),
	})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if tbl == nil {
		t.Fatalf("Collect returned a nil table without an error")
	}

	fqn, found := tbl.Resolve("PostController", td(fixtureRoutesImportBase))
	if fqn != fqnBasePostController || !found {
		t.Fatalf("Collect table Resolve = (%q, %v), want (%q, true)", fqn, found, fqnBasePostController)
	}
}

func TestCollectStopsAtFirstUnparseableFile(t *testing.T) {
	tbl, err := Collect([]string{
		td(fixturePostController),
		td("missing.php"), // aborts here
		td(fixtureRoutesImportBase),
	})
	if err == nil {
		t.Fatalf("Collect must fail when a path cannot be loaded")
	}
	if tbl != nil {
		t.Fatalf("Collect must return a nil table on error, got %v", tbl)
	}
}

// TestCollectAntiWrongEdgeFromFiles is the ADR 0006 correctness property driven
// end-to-end through the file loaders: with BOTH PostController files and BOTH
// importing route files collected, a short "PostController" reference resolves to
// the base FQN from the base-importing file and to the admin FQN from the
// admin-importing file — two different edges from the same short name, decided
// solely by the naming file's use-map.
func TestCollectAntiWrongEdgeFromFiles(t *testing.T) {
	tbl, err := Collect([]string{
		td(fixturePostController),
		td(fixtureAdminPostController),
		td(fixtureRoutesImportBase),
		td(fixtureRoutesImportAdmin),
	})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// Both FQNs must be declared — the declared set is shared across files.
	if !tbl.IsDeclared(fqnBasePostController) {
		t.Fatalf("base PostController not declared")
	}
	if !tbl.IsDeclared(fqnAdminPostController) {
		t.Fatalf("admin PostController not declared")
	}

	baseFQN, baseFound := tbl.Resolve("PostController", td(fixtureRoutesImportBase))
	adminFQN, adminFound := tbl.Resolve("PostController", td(fixtureRoutesImportAdmin))

	if baseFQN != fqnBasePostController || !baseFound {
		t.Fatalf("base route resolved to (%q, %v), want (%q, true)", baseFQN, baseFound, fqnBasePostController)
	}
	if adminFQN != fqnAdminPostController || !adminFound {
		t.Fatalf("admin route resolved to (%q, %v), want (%q, true)", adminFQN, adminFound, fqnAdminPostController)
	}
	if baseFQN == adminFQN {
		t.Fatalf("same short name collapsed to one FQN (%q) across files — wrong edge", baseFQN)
	}
}

// TestCollectDefaultNamespaceFallback drives the no-import fixture through the
// file loaders: plain Resolve leaves the bare name dead, while ResolveWithDefault
// applies the Laravel default controller namespace to reach the declared class.
func TestCollectDefaultNamespaceFallback(t *testing.T) {
	tbl, err := Collect([]string{
		td(fixturePostController),
		td(fixtureRoutesNoImport),
	})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if fqn, found := tbl.Resolve("PostController", td(fixtureRoutesNoImport)); fqn != "PostController" || found {
		t.Fatalf("Resolve without import = (%q, %v), want unresolved dead edge", fqn, found)
	}

	fqn, found := tbl.ResolveWithDefault("PostController", td(fixtureRoutesNoImport), defaultControllerNS)
	if fqn != fqnBasePostController || !found {
		t.Fatalf("ResolveWithDefault = (%q, %v), want (%q, true)", fqn, found, fqnBasePostController)
	}
}

func TestCollectEmptyPathsYieldsEmptyTable(t *testing.T) {
	tbl, err := Collect(nil)
	if err != nil {
		t.Fatalf("Collect(nil): %v", err)
	}
	if tbl == nil {
		t.Fatalf("Collect(nil) must return an empty, usable table")
	}
	if _, found := tbl.Resolve("Anything", "nowhere.php"); found {
		t.Fatalf("empty table must resolve nothing")
	}
}
