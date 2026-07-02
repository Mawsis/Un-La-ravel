// Package schema is the Schema Extractor: it turns a Laravel project's
// database/migrations/*.php files into the model's Table nodes (ADR 0001,
// CONTEXT.md glossary). It is a deep Extractor — it reads real migration AST via
// internal/phpast rather than booting Laravel (ADR 0003) — and produces only
// in-memory model values; persistence and rendering are someone else's job
// (ADR 0007).
//
// The package boundary is a single pure-ish entry point, Extract: migration
// file paths in, []model.Table out, with no I/O beyond reading the files it is
// given. Everything else (the AST visitor, the column/modifier mapping) is an
// unexported implementation detail.
package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// migrationGlob matches Laravel migration files within a migrations directory.
const migrationGlob = "*.php"

// Extract parses each migration file at the given paths and returns the tables
// they declare, as model.Table values in first-discovery order with columns in
// source-declaration order.
//
// Semantics across files:
//   - Schema::create('t', ...) introduces table t.
//   - Schema::table('t', ...) (in this or any later file) merges its columns
//     into the existing table t, creating a stub table if t was never created.
//
// Files are processed in the order given, so callers control discovery order by
// sorting paths (Laravel migrations sort lexically by their timestamp prefix).
//
// Errors: a file that cannot be read or catastrophically fails to parse aborts
// the whole extraction with a wrapped error, because a missing or unreadable
// migration means the resulting Schema would be silently incomplete. Recoverable
// per-file syntax diagnostics do NOT abort — the parser is fault-tolerant
// (ADR 0003) and a partially-valid migration still yields usable tables.
func Extract(paths []string) ([]model.Table, error) {
	merged := newMergedTables()

	for _, path := range paths {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("schema: extract %q: %w", path, err)
		}

		v := newSchemaVisitor()
		phpast.Walk(res.Root, v)

		for _, tb := range v.tables {
			merged.add(tb)
		}
	}

	return merged.tables(), nil
}

// ExtractDir is a convenience wrapper that discovers the migration files under
// dir (non-recursively, matching *.php), sorts them lexically so Laravel's
// timestamp-prefixed filenames are processed in chronological order, and runs
// Extract. A dir that does not exist or cannot be read yields a wrapped error.
func ExtractDir(dir string) ([]model.Table, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("schema: read migrations dir %q: %w", dir, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ok, _ := filepath.Match(migrationGlob, e.Name()); ok {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)

	return Extract(paths)
}

// mergedTables accumulates tables across files, keyed by name, while preserving
// first-discovery order for deterministic output.
type mergedTables struct {
	order  []string
	byName map[string]*tableBuilder
}

func newMergedTables() *mergedTables {
	return &mergedTables{byName: make(map[string]*tableBuilder)}
}

// add merges one file's table into the running set: a new name is recorded in
// discovery order; a known name has its columns and indexes appended (the
// Schema::table alteration case). Inputs are not mutated.
func (m *mergedTables) add(tb *tableBuilder) {
	existing, ok := m.byName[tb.name]
	if !ok {
		clone := &tableBuilder{name: tb.name}
		clone.columns = append(clone.columns, tb.columns...)
		clone.indexes = append(clone.indexes, tb.indexes...)
		m.byName[tb.name] = clone
		m.order = append(m.order, tb.name)
		return
	}
	existing.columns = append(existing.columns, tb.columns...)
	existing.indexes = append(existing.indexes, tb.indexes...)
}

// tables returns the accumulated tables as immutable model.Table values in
// discovery order. Each Table gets non-nil Columns and Indexes slices (via
// model.NewTable) so an empty table serializes as "columns": [] and
// "indexes": [] rather than null.
func (m *mergedTables) tables() []model.Table {
	out := make([]model.Table, 0, len(m.order))
	for _, name := range m.order {
		tb := m.byName[name]
		t := model.NewTable(name)
		t.Columns = append(t.Columns, tb.columns...)
		t.Indexes = append(t.Indexes, tb.indexes...)
		out = append(out, t)
	}
	return out
}
