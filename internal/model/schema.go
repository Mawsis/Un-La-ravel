package model

// This file defines the Schema-related Node types of the Project Model
// (ADR 0001). A Schema is the table-and-column structure Laravel would build
// if its migrations were run (see vault CONTEXT.md glossary). Here the Schema
// is represented as the ProjectModel.Schemas slice of Table values; each Table
// carries its ordered Columns. These are pure data types: no parsing, no I/O.

// Table is one database table within the Schema, extracted from
// database/migrations/*.php. Columns preserve their source-declaration order.
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

// Column is a single column on a Table. Type holds the Laravel column-builder
// method that declared it (for example "string", "bigInteger", "boolean"),
// not a database-native type, so renderers can map it as they see fit.
type Column struct {
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Nullable     bool           `json:"nullable"`
	IsPrimaryKey bool           `json:"is_primary_key"`
	IsForeignKey bool           `json:"is_foreign_key"`
	References   *ForeignKeyRef `json:"references,omitempty"`
}

// ForeignKeyRef is the target a foreign-key Column points at. Column is
// best-effort and may be empty when the migration relies on Laravel's
// implicit conventions (for example a bare ->constrained()).
type ForeignKeyRef struct {
	Table  string `json:"table"`
	Column string `json:"column,omitempty"`
}

// NewTable returns an empty Table with the given name and a non-nil Columns
// slice, so JSON serialization yields "columns": [] rather than null for a
// table that has not yet had columns appended.
func NewTable(name string) Table {
	return Table{
		Name:    name,
		Columns: []Column{},
	}
}
