package analyze

import (
	"reflect"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// tableWithColumns is a small helper that builds a model.Table carrying the
// named columns, keeping the test cases below terse and focused on the
// columns that matter to each scenario.
func tableWithColumns(name string, columns ...string) model.Table {
	t := model.NewTable(name)
	for _, c := range columns {
		t.Columns = append(t.Columns, model.Column{Name: c})
	}
	return t
}

// modelWith is a small helper that builds a model.Model mapped to the given
// table and carrying the given relationships in declaration order.
func modelWith(name, table string, rels ...model.Relationship) model.Model {
	m := model.NewModel(name)
	m.Table = table
	m.Relationships = append(m.Relationships, rels...)
	return m
}

// TestFindDisagreements drives the correlation through every contract case:
// a relationship to a known, present table (no finding); a relationship whose
// target table is missing (missing_table); an explicit belongsTo foreign key
// absent from the declaring model's own table (missing_fk_column); and the
// false-positive guards — an existing explicit FK and an implicit/conventional
// FK are both silent. Each case asserts the exact []Disagreement, including
// order, so the deterministic contract is locked.
func TestFindDisagreements(t *testing.T) {
	tests := []struct {
		name   string
		models []model.Model
		tables []model.Table
		want   []model.Disagreement
	}{
		{
			name: "relationship to a known present table yields no finding",
			models: []model.Model{
				modelWith("Post", "posts",
					model.Relationship{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "user_id"},
				),
				modelWith("User", "users"),
			},
			tables: []model.Table{
				tableWithColumns("posts", "id", "user_id"),
				tableWithColumns("users", "id"),
			},
			want: nil,
		},
		{
			name: "relationship to a missing target table yields missing_table",
			models: []model.Model{
				modelWith("Post", "posts",
					model.Relationship{Kind: "hasMany", Method: "comments", Target: "Comment"},
				),
				// Comment is a known model, but its table is absent from the schema.
				modelWith("Comment", "comments"),
			},
			tables: []model.Table{
				tableWithColumns("posts", "id"),
			},
			want: []model.Disagreement{
				{
					Model:        "Post",
					Relationship: "comments",
					Reason:       `target table "comments" for model "Comment" not found in schema`,
					Kind:         model.DisagreementMissingTable,
				},
			},
		},
		{
			name: "relationship to an unknown target model yields missing_table",
			models: []model.Model{
				modelWith("User", "users",
					model.Relationship{Kind: "belongsToMany", Method: "roles", Target: "Role"},
				),
			},
			tables: []model.Table{
				tableWithColumns("users", "id"),
			},
			want: []model.Disagreement{
				{
					Model:        "User",
					Relationship: "roles",
					Reason:       `target model "Role" is not among the extracted models, so its table cannot be resolved`,
					Kind:         model.DisagreementMissingTable,
				},
			},
		},
		{
			name: "belongsTo with explicit FK absent from schema yields missing_fk_column",
			models: []model.Model{
				modelWith("Post", "posts",
					model.Relationship{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "author_id"},
				),
				modelWith("User", "users"),
			},
			tables: []model.Table{
				// posts exists but lacks the explicit author_id column.
				tableWithColumns("posts", "id", "title"),
				tableWithColumns("users", "id"),
			},
			want: []model.Disagreement{
				{
					Model:        "Post",
					Relationship: "author",
					Reason:       `foreign key column "author_id" not found on table "posts"`,
					Kind:         model.DisagreementMissingFKColumn,
				},
			},
		},
		{
			name: "explicit FK that exists yields no finding",
			models: []model.Model{
				modelWith("Post", "posts",
					model.Relationship{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "author_id"},
				),
				modelWith("User", "users"),
			},
			tables: []model.Table{
				tableWithColumns("posts", "id", "author_id"),
				tableWithColumns("users", "id"),
			},
			want: nil,
		},
		{
			name: "implicit conventional FK is never synthesized or flagged",
			models: []model.Model{
				// belongsTo with NO explicit ForeignKey: the conventional user_id
				// is absent from posts, but the check must not guess it, so there
				// is no finding even though a synthesized key would be missing.
				modelWith("Post", "posts",
					model.Relationship{Kind: "belongsTo", Method: "author", Target: "User"},
				),
				modelWith("User", "users"),
			},
			tables: []model.Table{
				tableWithColumns("posts", "id"),
				tableWithColumns("users", "id"),
			},
			want: nil,
		},
		{
			name: "explicit FK is only checked for belongsTo, not hasMany",
			models: []model.Model{
				// A non-belongsTo kind carrying an explicit ForeignKey absent from
				// the table must NOT be flagged: only belongsTo stores its FK
				// locally, so only it is probed.
				modelWith("User", "users",
					model.Relationship{Kind: "hasMany", Method: "posts", Target: "Post", ForeignKey: "author_id"},
				),
				modelWith("Post", "posts"),
			},
			tables: []model.Table{
				tableWithColumns("users", "id"),
				tableWithColumns("posts", "id"),
			},
			want: nil,
		},
		{
			name: "missing FK check is skipped when the declaring table is itself absent",
			models: []model.Model{
				// posts is absent from the schema: checkTarget reports the missing
				// table once (for the hasMany), and the belongsTo's FK-column check
				// must be skipped so we do not get a redundant column finding on a
				// non-existent table.
				modelWith("Post", "posts",
					model.Relationship{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "author_id"},
				),
				modelWith("User", "users"),
			},
			tables: []model.Table{
				tableWithColumns("users", "id"),
			},
			want: nil,
		},
		{
			name: "one relationship can yield both a missing_table and a missing_fk_column",
			models: []model.Model{
				// belongsTo to an unknown target (missing_table) AND an explicit FK
				// absent from the declaring table that DOES exist (missing_fk_column):
				// checkTarget is appended before checkForeignKey.
				modelWith("Post", "posts",
					model.Relationship{Kind: "belongsTo", Method: "author", Target: "Author", ForeignKey: "author_id"},
				),
			},
			tables: []model.Table{
				tableWithColumns("posts", "id", "title"),
			},
			want: []model.Disagreement{
				{
					Model:        "Post",
					Relationship: "author",
					Reason:       `target model "Author" is not among the extracted models, so its table cannot be resolved`,
					Kind:         model.DisagreementMissingTable,
				},
				{
					Model:        "Post",
					Relationship: "author",
					Reason:       `foreign key column "author_id" not found on table "posts"`,
					Kind:         model.DisagreementMissingFKColumn,
				},
			},
		},
		{
			name: "findings preserve model-discovery then declaration order",
			models: []model.Model{
				modelWith("Post", "posts",
					model.Relationship{Kind: "hasMany", Method: "comments", Target: "Comment"},
					model.Relationship{Kind: "belongsTo", Method: "author", Target: "Author"},
				),
				modelWith("User", "users",
					model.Relationship{Kind: "belongsToMany", Method: "roles", Target: "Role"},
				),
			},
			tables: []model.Table{
				tableWithColumns("posts", "id"),
				tableWithColumns("users", "id"),
			},
			want: []model.Disagreement{
				{
					Model:        "Post",
					Relationship: "comments",
					Reason:       `target model "Comment" is not among the extracted models, so its table cannot be resolved`,
					Kind:         model.DisagreementMissingTable,
				},
				{
					Model:        "Post",
					Relationship: "author",
					Reason:       `target model "Author" is not among the extracted models, so its table cannot be resolved`,
					Kind:         model.DisagreementMissingTable,
				},
				{
					Model:        "User",
					Relationship: "roles",
					Reason:       `target model "Role" is not among the extracted models, so its table cannot be resolved`,
					Kind:         model.DisagreementMissingTable,
				},
			},
		},
		{
			name:   "no models yields no findings",
			models: nil,
			tables: []model.Table{tableWithColumns("posts", "id")},
			want:   nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FindDisagreements(tt.models, tt.tables)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindDisagreements() = %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

// TestFindDisagreementsDoesNotMutateInputs verifies the function is pure: it
// allocates its own lookup maps and leaves the caller's slices untouched.
func TestFindDisagreementsDoesNotMutateInputs(t *testing.T) {
	models := []model.Model{
		modelWith("Post", "posts",
			model.Relationship{Kind: "belongsTo", Method: "author", Target: "Author", ForeignKey: "author_id"},
		),
	}
	tables := []model.Table{tableWithColumns("posts", "id")}

	wantModels := append([]model.Model(nil), models...)
	wantTables := append([]model.Table(nil), tables...)

	_ = FindDisagreements(models, tables)

	if !reflect.DeepEqual(models, wantModels) {
		t.Errorf("FindDisagreements() mutated the models slice: got %#v, want %#v", models, wantModels)
	}
	if !reflect.DeepEqual(tables, wantTables) {
		t.Errorf("FindDisagreements() mutated the tables slice: got %#v, want %#v", tables, wantTables)
	}
}
