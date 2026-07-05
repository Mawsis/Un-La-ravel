package model_test

// These are external-behavior tests for the Eloquent Model Extractor. They
// import the package as model_test (black-box) and assert only on the public
// contract: given a PHP source file, Extract returns the expected
// []domain.Model — each model's Name, Table, and its Relationships' Kind /
// Method / Target / ForeignKey / LocalKey, in source-declaration order. The AST
// visitor internals and unexported helpers are deliberately not touched.
//
// The domain types package (internal/model) and the extractor package under
// test are BOTH named "model", so the domain package is imported as domain and
// the extractor under test is imported as its plain name.

import (
	"path/filepath"
	"reflect"
	"testing"

	extractmodel "github.com/Mawsis/Un-La-ravel/internal/extract/model"
	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// rel is a terse constructor for an expected Relationship; the variadic keys let
// each case supply only the explicit columns the source declares (FK, then
// local key), keeping the table compact while still asserting exact values.
func rel(kind, method, target string, keys ...string) domain.Relationship {
	r := domain.Relationship{Kind: kind, Method: method, Target: target}
	if len(keys) > 0 {
		r.ForeignKey = keys[0]
	}
	if len(keys) > 1 {
		r.LocalKey = keys[1]
	}
	return r
}

func TestExtract(t *testing.T) {
	tests := []struct {
		name    string   // sub-test name / pattern under test
		files   []string // testdata filenames, in extraction order
		want    []domain.Model
		pattern string // human note on what behavior this pins down
	}{
		{
			name:    "has_many",
			files:   []string{"has_many.php"},
			pattern: "hasMany(Book::class) → kind=hasMany, target=Book, no FK; table inferred Author→authors",
			want: []domain.Model{
				{Name: "Author", Table: "authors", Relationships: []domain.Relationship{
					rel("hasMany", "books", "Book"),
				}},
			},
		},
		{
			name:    "belongs_to_explicit_fk",
			files:   []string{"belongs_to_explicit_fk.php"},
			pattern: "belongsTo(Post::class, 'post_id') → ForeignKey captured from the 2nd arg",
			want: []domain.Model{
				{Name: "Comment", Table: "comments", Relationships: []domain.Relationship{
					rel("belongsTo", "post", "Post", "post_id"),
				}},
			},
		},
		{
			name:    "has_one",
			files:   []string{"has_one.php"},
			pattern: "hasOne(Account::class) → kind=hasOne, target=Account, no FK",
			want: []domain.Model{
				{Name: "Supplier", Table: "suppliers", Relationships: []domain.Relationship{
					rel("hasOne", "account", "Account"),
				}},
			},
		},
		{
			name:    "belongs_to_many",
			files:   []string{"belongs_to_many.php"},
			pattern: "belongsToMany(Role::class) → kind=belongsToMany, target=Role, no FK",
			want: []domain.Model{
				{Name: "User", Table: "users", Relationships: []domain.Relationship{
					rel("belongsToMany", "roles", "Role"),
				}},
			},
		},
		{
			name:    "class_const_target",
			files:   []string{"class_target.php"},
			pattern: "Customer::class first arg resolves to bare class name Customer",
			want: []domain.Model{
				{Name: "Order", Table: "orders", Relationships: []domain.Relationship{
					rel("belongsTo", "customer", "Customer"),
				}},
			},
		},
		{
			name:    "string_target_last_segment",
			files:   []string{"string_target.php"},
			pattern: "string target 'App\\Models\\Album' reduces to last namespace segment Album",
			want: []domain.Model{
				{Name: "Photo", Table: "photos", Relationships: []domain.Relationship{
					rel("belongsTo", "album", "Album"),
				}},
			},
		},
		{
			name:    "explicit_table_wins",
			files:   []string{"explicit_table.php"},
			pattern: "protected $table = 'articles' overrides the inferred BlogPost→blog_posts",
			want: []domain.Model{
				{Name: "BlogPost", Table: "articles", Relationships: []domain.Relationship{
					rel("belongsTo", "author", "User"),
				}},
			},
		},
		{
			name:    "sibilant_table_and_ignored_method_call",
			files:   []string{"sibilant_table.php"},
			pattern: "Box→boxes (sibilant plural); a non-relationship method call ($this->save()) is ignored, only belongsTo(Shelf) is kept",
			want: []domain.Model{
				{Name: "Box", Table: "boxes", Relationships: []domain.Relationship{
					rel("belongsTo", "shelf", "Shelf"),
				}},
			},
		},
		{
			name:    "non_model_ignored",
			files:   []string{"not_a_model.php"},
			pattern: "class extending no Eloquent base emits no Model, even with a relationship-shaped method body",
			want:    nil,
		},
		{
			name:    "no_relationships_empty_slice",
			files:   []string{"no_relationships.php"},
			pattern: "model with no relationship methods yields an empty, non-nil Relationships slice",
			want: []domain.Model{
				{Name: "Setting", Table: "settings", Relationships: []domain.Relationship{}},
			},
		},
		{
			name:    "discovery_order_across_files",
			files:   []string{"has_many.php", "has_one.php"},
			pattern: "models are returned in file (discovery) order: Author from the first file, then Supplier",
			want: []domain.Model{
				{Name: "Author", Table: "authors", Relationships: []domain.Relationship{
					rel("hasMany", "books", "Book"),
				}},
				{Name: "Supplier", Table: "suppliers", Relationships: []domain.Relationship{
					rel("hasOne", "account", "Account"),
				}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			paths := make([]string, len(tt.files))
			for i, f := range tt.files {
				paths[i] = filepath.Join("testdata", f)
			}

			got, err := extractmodel.Extract(paths)
			if err != nil {
				t.Fatalf("Extract(%v) returned error: %v", tt.files, err)
			}

			assertModels(t, got, tt.want)
		})
	}
}

// TestExtract_emptyRelationshipsIsNonNil pins the JSON-contract guarantee that a
// relationless model carries a non-nil empty slice (so it serializes as
// "relationships": [] rather than null). reflect.DeepEqual treats nil and empty
// slices as different, but this makes the intent explicit and independent of the
// table comparison.
func TestExtract_emptyRelationshipsIsNonNil(t *testing.T) {
	got, err := extractmodel.Extract([]string{filepath.Join("testdata", "no_relationships.php")})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one model, got %d", len(got))
	}
	if got[0].Relationships == nil {
		t.Error("Relationships is nil; want a non-nil empty slice for JSON []")
	}
	if len(got[0].Relationships) != 0 {
		t.Errorf("Relationships = %v, want empty", got[0].Relationships)
	}
}

// TestExtract_MassAssignmentAndCasts pins the Fillable/Guarded/Casts extraction
// contract, most importantly the nil-vs-empty-slice distinction on Fillable and
// Guarded (see domain.Model's doc comment): nil means "not declared", a non-nil
// (possibly empty) slice means "declared". reflect.DeepEqual-based helpers
// elsewhere in this file do not reliably distinguish nil from empty, so the
// nil-critical cases here use explicit `== nil` / `!= nil` checks instead.
func TestExtract_MassAssignmentAndCasts(t *testing.T) {
	t.Run("fillable_declared", func(t *testing.T) {
		got := extractOne(t, "fillable.php")

		if got.Fillable == nil {
			t.Fatal("Fillable is nil; want a declared non-nil slice")
		}
		want := []string{"title", "body"}
		if !reflect.DeepEqual(got.Fillable, want) {
			t.Errorf("Fillable = %v, want %v", got.Fillable, want)
		}
		if got.Guarded != nil {
			t.Errorf("Guarded = %v, want nil (not declared)", got.Guarded)
		}
	})

	t.Run("guarded_declared_empty", func(t *testing.T) {
		got := extractOne(t, "guarded_declared_empty.php")

		if got.Guarded == nil {
			t.Fatal("Guarded is nil; want a non-nil empty slice (explicit `$guarded = []`)")
		}
		if len(got.Guarded) != 0 {
			t.Errorf("Guarded = %v, want empty", got.Guarded)
		}
		if got.Fillable != nil {
			t.Errorf("Fillable = %v, want nil (not declared)", got.Fillable)
		}
	})

	t.Run("no_mass_assignment_declared", func(t *testing.T) {
		got := extractOne(t, "no_mass_assignment_declared.php")

		if got.Fillable != nil {
			t.Errorf("Fillable = %v, want nil (neither $fillable nor $guarded declared)", got.Fillable)
		}
		if got.Guarded != nil {
			t.Errorf("Guarded = %v, want nil (neither $fillable nor $guarded declared)", got.Guarded)
		}
	})

	t.Run("casts_property", func(t *testing.T) {
		got := extractOne(t, "casts_property.php")

		want := []domain.Cast{{Column: "email_verified_at", Type: "datetime"}}
		if !reflect.DeepEqual(got.Casts, want) {
			t.Errorf("Casts = %v, want %v", got.Casts, want)
		}
	})

	t.Run("casts_method", func(t *testing.T) {
		got := extractOne(t, "casts_method.php")

		want := []domain.Cast{
			{Column: "paid_at", Type: "datetime"},
			{Column: "total", Type: "integer"},
		}
		if !reflect.DeepEqual(got.Casts, want) {
			t.Errorf("Casts = %v, want %v", got.Casts, want)
		}
	})
}

// extractOne runs Extract on a single testdata file and returns its one
// expected model, failing the test if extraction errors or does not yield
// exactly one model.
func extractOne(t *testing.T, file string) domain.Model {
	t.Helper()

	got, err := extractmodel.Extract([]string{filepath.Join("testdata", file)})
	if err != nil {
		t.Fatalf("Extract(%q) returned error: %v", file, err)
	}
	if len(got) != 1 {
		t.Fatalf("Extract(%q) returned %d models, want 1", file, len(got))
	}
	return got[0]
}

// TestExtractDir exercises the directory-discovery wrapper. ExtractDir sorts the
// testdata files lexically, so this asserts on the full set by name rather than
// pinning a brittle whole-slice order, while still checking a couple of exact
// models. not_a_model.php must contribute no Model.
func TestExtractDir(t *testing.T) {
	got, err := extractmodel.ExtractDir("testdata")
	if err != nil {
		t.Fatalf("ExtractDir returned error: %v", err)
	}

	byName := make(map[string]domain.Model, len(got))
	for _, m := range got {
		byName[m.Name] = m
	}

	// Every Eloquent fixture must surface exactly once; the non-model must not.
	wantNames := []string{
		"Author", "Comment", "Supplier", "User", "Order",
		"Photo", "BlogPost", "Setting", "Box",
		"Post", "Category", "Account", "Invoice", "Widget",
	}
	for _, name := range wantNames {
		if _, ok := byName[name]; !ok {
			t.Errorf("ExtractDir: expected a %q model, got models %v", name, modelNames(got))
		}
	}
	if _, ok := byName["ReportBuilder"]; ok {
		t.Errorf("ExtractDir: non-Eloquent class ReportBuilder must not be emitted; got models %v", modelNames(got))
	}
	if len(got) != len(wantNames) {
		t.Errorf("ExtractDir: model count = %d, want %d (got %v)", len(got), len(wantNames), modelNames(got))
	}

	// Spot-check two exact models, including the explicit-table override.
	if diff := relationshipsEqual(byName["Comment"].Relationships,
		[]domain.Relationship{rel("belongsTo", "post", "Post", "post_id")}); diff != "" {
		t.Errorf("ExtractDir Comment relationships mismatch: %s", diff)
	}
	if got := byName["BlogPost"].Table; got != "articles" {
		t.Errorf("ExtractDir BlogPost.Table = %q, want %q (explicit $table)", got, "articles")
	}
}

func TestExtractDir_missingDir(t *testing.T) {
	if _, err := extractmodel.ExtractDir(filepath.Join("testdata", "does-not-exist")); err == nil {
		t.Fatal("ExtractDir on a missing directory: expected error, got nil")
	}
}

func TestExtract_missingFile(t *testing.T) {
	if _, err := extractmodel.Extract([]string{filepath.Join("testdata", "does-not-exist.php")}); err == nil {
		t.Fatal("Extract on a missing file: expected error, got nil")
	}
}

// TestTableName pins the inference convention used when a model declares no
// explicit $table: snake_case the class name, then pluralize the final word.
func TestTableName(t *testing.T) {
	tests := []struct {
		class string
		want  string
	}{
		{"User", "users"},
		{"BlogPost", "blog_posts"},
		{"Category", "categories"},
		{"Company", "companies"},
		{"Post", "posts"},
		{"Tag", "tags"},
		{"Box", "boxes"},      // sibilant -es
		{"Day", "days"},       // vowel+y keeps regular -s
		{"Person", "people"},  // irregular
		{"Child", "children"}, // irregular
		// Already-plural stems stay as they are instead of gaining a second
		// plural suffix (issue #36: WaiterCalls must not become waiter_callses).
		{"WaiterCalls", "waiter_calls"},
		{"ActivityLogs", "activity_logs"},
		// Genuinely singular s-endings still pluralize with -es.
		{"Status", "statuses"},
		{"Bus", "buses"},
		{"Class", "classes"},
		// Uncountables keep their form (RestaurantStaff -> restaurant_staff).
		{"RestaurantStaff", "restaurant_staff"},
		{"Sheep", "sheep"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.class, func(t *testing.T) {
			if got := extractmodel.TableName(tt.class); got != tt.want {
				t.Errorf("TableName(%q) = %q, want %q", tt.class, got, tt.want)
			}
		})
	}
}

// assertModels compares produced models against expectations by name, table, and
// relationships, with focused error messages so a failure points at the exact
// field. A nil want means "no models expected".
func assertModels(t *testing.T, got, want []domain.Model) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("model count = %d, want %d (got %v, want %v)",
			len(got), len(want), modelNames(got), modelNames(want))
	}

	for i := range want {
		if got[i].Name != want[i].Name {
			t.Errorf("model[%d].Name = %q, want %q", i, got[i].Name, want[i].Name)
			continue
		}
		if got[i].Table != want[i].Table {
			t.Errorf("model %q: Table = %q, want %q", want[i].Name, got[i].Table, want[i].Table)
		}
		if diff := relationshipsEqual(got[i].Relationships, want[i].Relationships); diff != "" {
			t.Errorf("model %q relationships mismatch: %s", want[i].Name, diff)
		}
	}
}

// relationshipsEqual returns "" when the two relationship slices are equal, or a
// human-readable description of the first difference otherwise. Length is
// compared first; nil vs empty is treated as equal here because callers assert
// the non-nil guarantee separately.
func relationshipsEqual(got, want []domain.Relationship) string {
	if len(got) != len(want) {
		return "count = " + itoa(len(got)) + ", want " + itoa(len(want)) +
			" (got " + relMethods(got) + ", want " + relMethods(want) + ")"
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			return "relationship[" + itoa(i) + "] = " + describeRel(got[i]) +
				", want " + describeRel(want[i])
		}
	}
	return ""
}

func modelNames(ms []domain.Model) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

func relMethods(rs []domain.Relationship) string {
	s := "["
	for i, r := range rs {
		if i > 0 {
			s += " "
		}
		s += r.Method
	}
	return s + "]"
}

func describeRel(r domain.Relationship) string {
	s := r.Kind + " " + r.Method + "->" + r.Target
	if r.ForeignKey != "" {
		s += " fk=" + r.ForeignKey
	}
	if r.LocalKey != "" {
		s += " lk=" + r.LocalKey
	}
	return s
}

// itoa is a tiny dependency-free int-to-string to keep the test helpers free of
// fmt noise in hot error paths (mirrors the schema test helper).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
