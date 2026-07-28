package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	domain "github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// modelGlob matches PHP source files within a models directory.
const modelGlob = "*.php"

// Extract parses each PHP file at the given paths and returns the Eloquent
// models they declare, as domain.Model values in first-discovery order with
// relationships in source-declaration order.
//
// A file may declare several classes; one domain.Model is emitted per class that
// extends an Eloquent base (Model, Authenticatable, or Pivot). Classes that do
// not extend an Eloquent base are skipped entirely — no Model node is produced.
// A model with no relationships is emitted with an empty (non-nil) Relationships
// slice.
//
// Each model's Table is its explicit `protected $table` value when present,
// otherwise the table name inferred from the class name via TableName (Laravel's
// snake_case-and-pluralize convention).
//
// Each model's Fillable, Guarded, and Hidden reflect whether the source
// declared `protected $fillable` / `protected $guarded` / `protected $hidden`
// at all: each is nil when its property is not declared, and a declared
// property (even an empty array literal) yields a non-nil slice — see
// domain.Model's doc comment for why this nil-vs-empty distinction is
// preserved rather than normalised away. Casts collects `protected $casts`
// and/or a Laravel 11 `casts(): array` method, with the method's result taking
// precedence when both are present; it is always non-nil.
//
// Files are processed in the order given, so callers control discovery order by
// sorting paths. The same class name appearing in two files yields two Model
// values; this slice does no project-wide deduplication (ADR 0006's symbol table
// is out of scope — correlation is light and by-name, done by the caller).
//
// Errors: a file that cannot be read or catastrophically fails to parse aborts
// the whole extraction with a wrapped error, because a missing or unreadable
// model file means the resulting model set would be silently incomplete.
// Recoverable per-file syntax diagnostics do NOT abort — the parser is
// fault-tolerant (ADR 0003) and a partially-valid model still yields usable
// relationships.
func Extract(paths []string) ([]domain.Model, error) {
	var models []domain.Model

	for _, path := range paths {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("model: extract %q: %w", path, err)
		}

		v := newModelVisitor()
		phpast.Walk(res.Root, v)

		for _, mb := range v.models {
			models = append(models, buildModel(mb))
		}
	}

	return models, nil
}

// ExtractDir is a convenience wrapper that discovers the PHP files under dir
// (non-recursively, matching *.php), sorts them lexically for deterministic
// discovery order, and runs Extract. A dir that does not exist or cannot be read
// yields a wrapped error.
//
// Laravel models live in either app/Models or, in classic layouts, directly in
// app/; both are just directories of PHP files, so the caller points ExtractDir
// (or Extract, with explicit paths) at whichever location applies.
func ExtractDir(dir string) ([]domain.Model, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("model: read models dir %q: %w", dir, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ok, _ := filepath.Match(modelGlob, e.Name()); ok {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)

	return Extract(paths)
}

// buildModel converts a mutable modelBuilder into an immutable domain.Model. The
// table is resolved here (explicit $table wins, else inferred via TableName) and
// the relationships are copied into the model's non-nil slice so an empty model
// serializes as "relationships": [] rather than null.
//
// Fillable, Guarded, and Hidden are copied straight from the builder with NO
// coercion: mb.fillable / mb.guarded / mb.hidden are already nil exactly when
// the source did not declare that property, and non-nil (possibly empty)
// exactly when it did — see domain.Model's doc comment on
// Fillable/Guarded/Hidden. Casts is appended onto NewModel's non-nil starting
// slice, mirroring the Relationships pattern, since a model's cast list carries
// no analogous "not declared" meaning worth preserving.
func buildModel(mb *modelBuilder) domain.Model {
	m := domain.NewModel(mb.className)
	m.Table = resolveTable(mb)
	m.Relationships = append(m.Relationships, mb.relationships...)
	m.Fillable = mb.fillable
	m.Guarded = mb.guarded
	m.Hidden = mb.hidden
	m.Casts = append(m.Casts, mb.casts...)
	return m
}

// resolveTable returns the table a model maps to: its explicit `protected
// $table` value when the source declared one, otherwise the name inferred from
// the class name via Laravel's convention.
func resolveTable(mb *modelBuilder) string {
	if mb.explicitTable != "" {
		return mb.explicitTable
	}
	return TableName(mb.className)
}
