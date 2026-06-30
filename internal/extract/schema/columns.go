package schema

import "github.com/mawsis/unlaravel/internal/model"

// This file holds the pure, AST-free knowledge of how a Laravel Blueprint
// column-builder method maps to model.Column values. Keeping it free of any
// php-parser types makes the mapping trivially table-testable: a method name
// (plus its first string argument) goes in, zero or more Columns come out.

// columnType is the canonical model.Column.Type recorded for a builder method.
// Laravel exposes the column builder under the method name itself (for example
// "string", "boolean"), so for most builders the type IS the method name. A few
// builders declare columns whose type differs from the method, captured below.
const (
	typeBigInteger = "bigInteger"
	typeTimestamp  = "timestamp"
	typeString     = "string"
)

// fixedColumnMethods maps Blueprint methods that emit one or more columns with
// fixed, conventional names and types — independent of any argument. For
// example timestamps() always emits created_at and updated_at. These are looked
// up before argument-named builders so they never consume a string argument.
var fixedColumnMethods = map[string][]model.Column{
	"timestamps": {
		{Name: "created_at", Type: typeTimestamp, Nullable: true},
		{Name: "updated_at", Type: typeTimestamp, Nullable: true},
	},
	"timestampsTz": {
		{Name: "created_at", Type: typeTimestamp, Nullable: true},
		{Name: "updated_at", Type: typeTimestamp, Nullable: true},
	},
	"softDeletes": {
		{Name: "deleted_at", Type: typeTimestamp, Nullable: true},
	},
	"softDeletesTz": {
		{Name: "deleted_at", Type: typeTimestamp, Nullable: true},
	},
	"rememberToken": {
		{Name: "remember_token", Type: typeString, Nullable: true},
	},
}

// namedBuilderMethods is the set of Blueprint methods whose first string
// argument is the column name and whose recorded type is the method name. These
// are the bread-and-butter typed columns (string, integer, json, uuid, ...).
var namedBuilderMethods = map[string]bool{
	"string":             true,
	"char":               true,
	"text":               true,
	"mediumText":         true,
	"longText":           true,
	"integer":            true,
	"bigInteger":         true,
	"unsignedBigInteger": true,
	"tinyInteger":        true,
	"smallInteger":       true,
	"mediumInteger":      true,
	"boolean":            true,
	"timestamp":          true,
	"date":               true,
	"dateTime":           true,
	"datetime":           true,
	"time":               true,
	"json":               true,
	"jsonb":              true,
	"uuid":               true,
	"ulid":               true,
	"decimal":            true,
	"float":              true,
	"double":             true,
	"enum":               true,
}

// foreignBuilderMethods is the set of Blueprint methods whose first string
// argument names a foreign-key column of conventional bigInteger type. The
// target table/column is resolved later from chained ->constrained()/
// ->references()->on() modifiers, so the reference is left unset here.
var foreignBuilderMethods = map[string]bool{
	"foreignId":   true,
	"foreignUuid": true,
	"foreignUlid": true,
}

// columnsForMethod returns the Column(s) a base Blueprint builder call declares,
// given the method name and its first string argument (the column name, when
// applicable). It returns nil when the method does not define a column at the
// model's level of detail (for example index or modifier methods). It is pure:
// no AST, no I/O, no mutation of inputs.
func columnsForMethod(method, firstStringArg string) []model.Column {
	if cols, ok := fixedColumnMethods[method]; ok {
		return cloneColumns(cols)
	}

	switch {
	case method == "id":
		// id() declares the conventional auto-incrementing primary key.
		return []model.Column{{Name: "id", Type: typeBigInteger, IsPrimaryKey: true}}

	case method == "uuid" && firstStringArg == "":
		// uuid() with no argument is Laravel's UUID primary key shortcut.
		return []model.Column{{Name: "uuid", Type: "uuid", IsPrimaryKey: true}}

	case foreignBuilderMethods[method]:
		if firstStringArg == "" {
			return nil
		}
		return []model.Column{{Name: firstStringArg, Type: typeBigInteger, IsForeignKey: true}}

	case namedBuilderMethods[method]:
		if firstStringArg == "" {
			return nil
		}
		return []model.Column{{Name: firstStringArg, Type: method}}
	}

	return nil
}

// cloneColumns returns a deep-enough copy of a fixed-column template so callers
// can mutate the returned columns (for example to attach modifiers) without
// corrupting the shared package-level template. Columns hold no nested mutable
// state at definition time (References is nil for fixed columns), so a shallow
// per-element copy suffices.
func cloneColumns(src []model.Column) []model.Column {
	out := make([]model.Column, len(src))
	copy(out, src)
	return out
}
