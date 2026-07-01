package model

// This file defines the Eloquent-related Node types of the Project Model
// (ADR 0001): the Model, its Relationships, and the Disagreement finding that
// records where a relationship references something the Schema lacks.
//
// A Model is one Eloquent class extracted from app/Models/*.php (or wherever a
// class extends an Eloquent base). A Relationship is an Eloquent association
// (hasMany/hasOne/belongsTo/belongsToMany) declared by a `$this-><kind>(...)`
// call inside one of the Model's own methods. A Disagreement is the new domain
// term this slice introduces (see vault CONTEXT.md): a relationship whose
// target table or foreign-key column is absent from the extracted Schema.
//
// These are pure data types: no parsing, no I/O, no correlation logic. The
// Model→Schema correlation that produces Disagreements lives in an extractor;
// this package only carries the resulting values.

// Relationship is one Eloquent association declared on a Model. It captures the
// relationship kind, the PHP method that declares it, and the related Model the
// association points at, plus the explicit key columns when the source code
// supplied them.
type Relationship struct {
	// Kind is the Eloquent relationship kind, one of the four supported in this
	// slice: "hasMany", "hasOne", "belongsTo", or "belongsToMany". It is the
	// name of the `$this-><kind>(...)` method call that declared the
	// association.
	Kind string `json:"kind"`
	// Method is the PHP method name that wraps the relationship call (for
	// example "author" for a method `public function author()`). This is the
	// name application code uses to access the relation.
	Method string `json:"method"`
	// Target is the related Model class name (for example "User"), resolved
	// from the first argument of the relationship call — either a `User::class`
	// constant or a string class reference reduced to its last namespace
	// segment.
	Target string `json:"target"`
	// ForeignKey is the explicit foreign-key column when the source supplied one
	// (for example "author_id" in `belongsTo(User::class, 'author_id')`).
	// Omitted from JSON when empty, in which case the FK follows Laravel's
	// implicit convention.
	ForeignKey string `json:"foreign_key,omitempty"`
	// LocalKey is the explicit local key column when the source supplied one.
	// Omitted from JSON when empty, in which case the local key follows
	// Laravel's implicit convention.
	LocalKey string `json:"local_key,omitempty"`
}

// Model is one Eloquent model class within the Project Model, extracted from a
// class that extends an Eloquent base. Relationships preserve their
// source-declaration order.
type Model struct {
	// Name is the model class name (for example "Post").
	Name string `json:"name"`
	// Table is the database table the model maps to: the explicit
	// `protected $table` value when present, otherwise the table name inferred
	// from the class name via Laravel's snake_case-and-pluralize convention.
	Table string `json:"table"`
	// Relationships are the Eloquent associations declared on the model, in
	// source-declaration order.
	Relationships []Relationship `json:"relationships"`
}

// Disagreement kinds. These are the stable machine-readable values written to
// Disagreement.Kind so consumers can branch without parsing the human-readable
// Reason. Defined once here; never hardcode the literal elsewhere.
const (
	// DisagreementMissingTable marks a relationship whose target Model resolves
	// to a table that is absent from the extracted Schema.
	DisagreementMissingTable = "missing_table"
	// DisagreementMissingFKColumn marks a relationship whose explicit foreign-key
	// column is absent from the table it should belong to in the Schema.
	DisagreementMissingFKColumn = "missing_fk_column"
)

// Disagreement is a finding that an Eloquent Relationship references something
// the extracted Schema does not contain — a missing target table or a missing
// foreign-key column. It is the new domain term this slice introduces (see
// vault CONTEXT.md): the Model and the Schema disagree about what exists.
type Disagreement struct {
	// Model is the name of the Model whose relationship triggered the finding.
	Model string `json:"model"`
	// Relationship is the PHP method name of the offending relationship (the
	// Relationship.Method value).
	Relationship string `json:"relationship"`
	// Reason is a human-readable explanation, for example
	// `target table "users" not found in schema` or
	// `foreign key column "author_id" not found on table "posts"`.
	Reason string `json:"reason"`
	// Kind is the machine-readable category, one of DisagreementMissingTable or
	// DisagreementMissingFKColumn, so consumers can branch without parsing
	// Reason.
	Kind string `json:"kind"`
}

// NewModel returns a Model with the given class name and a non-nil
// Relationships slice, so JSON serialization yields "relationships": [] rather
// than null for a model that has not yet had relationships appended.
func NewModel(name string) Model {
	return Model{
		Name:          name,
		Relationships: []Relationship{},
	}
}
