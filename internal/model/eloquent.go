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

// Cast is one column's Eloquent attribute-cast declaration, sourced from
// either a `protected $casts` property or (Laravel 11+) a `casts(): array`
// method.
type Cast struct {
	// Column is the model attribute name being cast (for example
	// "email_verified_at").
	Column string `json:"column"`
	// Type is the cast type (for example "datetime", "integer", "boolean").
	Type string `json:"type"`
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

	// Fillable, Guarded, and Casts deliberately DEPART from this file's usual
	// non-nil-slice convention (contrast Relationships above, and NewModel
	// below). The nil-ness of Fillable and Guarded is load-bearing and carries
	// distinct Laravel mass-assignment semantics that must survive to JSON and
	// to any consumer of this Model:
	//
	//   - Fillable == nil AND Guarded == nil means the model declared NEITHER
	//     property. Laravel's default in this state is guarded-by-omission:
	//     mass assignment is fully protected until one of the two is set.
	//   - Guarded == []string{} (non-nil, empty) means the source explicitly
	//     wrote `protected $guarded = [];` — Laravel's documented "everything
	//     is mass-assignable" escape hatch. This is a MATERIALLY DIFFERENT,
	//     riskier state than Guarded == nil, and a future doctor/lint rule
	//     needs to be able to tell the two apart from the JSON alone.
	//   - The same nil-vs-empty distinction applies symmetrically to Fillable.
	//
	// Consequently, nothing in the extraction path (visitor, builder,
	// buildModel) may coerce a nil Fillable or Guarded to []string{}, or vice
	// versa. This is the one exception to the package's "non-nil slices by
	// default" rule.
	//
	// Casts does NOT carry this exception: it stays non-nil-preferred
	// (empty []Cast{} rather than nil) like Relationships, because there is no
	// analogous distinct meaning to "the model declared an empty casts list"
	// worth preserving.
	Fillable []string `json:"fillable"`
	Guarded  []string `json:"guarded"`
	Casts    []Cast   `json:"casts"`
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

// NewModel returns a Model with the given class name and non-nil
// Relationships and Casts slices, so JSON serialization yields
// "relationships": [] and "casts": [] rather than null for a model that has
// not yet had either appended. Fillable and Guarded are deliberately left at
// their zero value (nil): per the field-level doc comment above, nil vs. an
// empty slice is load-bearing for those two and must not be coerced here.
func NewModel(name string) Model {
	return Model{
		Name:          name,
		Relationships: []Relationship{},
		Casts:         []Cast{},
	}
}
