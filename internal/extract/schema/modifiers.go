package schema

import "github.com/Mawsis/Un-La-ravel/internal/model"

// This file holds the pure logic for applying chained Blueprint modifiers to
// the column(s) a base builder declared. A "modifier" is a method invoked on the
// fluent column object after the base builder — for example the ->nullable() in
// $table->string('email')->nullable(). Modifiers are AST-free here: each is
// reduced to a method name plus its first string argument before reaching this
// file, so the mapping stays table-testable.

// chainCall is one link in a fluent column-definition chain, reduced to the
// minimum the schema model cares about: the invoked method name and its first
// string-literal argument (empty when absent). Token/position detail is dropped
// at the visitor boundary so this logic never touches the parser.
type chainCall struct {
	Method    string
	StringArg string
}

// applyModifiers returns new Columns with the chain's modifiers applied. It does
// not mutate its input: per the project's immutability rule it copies each
// column and returns fresh values. A modifier conceptually decorates "the
// column being defined"; for multi-column builders (timestamps) a modifier such
// as ->nullable() is applied to every emitted column, matching Laravel, where
// the modifier returns the same shared column-definition context.
func applyModifiers(cols []model.Column, chain []chainCall) []model.Column {
	if len(cols) == 0 {
		return cols
	}

	out := make([]model.Column, len(cols))
	copy(out, cols)

	ref := foreignKeyRefFromChain(chain)

	for i := range out {
		for _, c := range chain {
			switch c.Method {
			case "nullable":
				out[i].Nullable = true
			case "primary":
				out[i].IsPrimaryKey = true
			case "constrained", "references", "foreignId":
				out[i].IsForeignKey = true
			}
		}
		if ref != nil {
			out[i].IsForeignKey = true
			refCopy := *ref
			out[i].References = &refCopy
		}
	}

	return out
}

// foreignKeyRefFromChain resolves a foreign-key target from the modifier chain,
// supporting both Laravel forms:
//
//	->constrained('teams')            // explicit table, implicit "id"
//	->constrained()                   // implicit table+column (left best-effort empty)
//	->references('id')->on('teams')   // explicit column and table
//
// It returns nil when the chain declares no foreign-key relationship. The
// returned ref is freshly allocated so callers own it.
func foreignKeyRefFromChain(chain []chainCall) *model.ForeignKeyRef {
	var (
		hasFK  bool
		table  string
		column string
	)

	for _, c := range chain {
		switch c.Method {
		case "constrained":
			hasFK = true
			if c.StringArg != "" {
				table = c.StringArg
			}
		case "references":
			hasFK = true
			if c.StringArg != "" {
				column = c.StringArg
			}
		case "on":
			hasFK = true
			if c.StringArg != "" {
				table = c.StringArg
			}
		}
	}

	// A bare ->constrained() declares a foreign key but resolving its target
	// requires Laravel's naming conventions (and a known related model), which
	// is out of scope for static migration analysis. In that case the column is
	// still flagged IsForeignKey (see applyModifiers) but we attach no
	// References rather than emitting an empty, misleading {"table":""}.
	if !hasFK || table == "" {
		return nil
	}
	return &model.ForeignKeyRef{Table: table, Column: column}
}
