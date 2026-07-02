package schema_test

// These are external-behavior tests for the Schema Extractor. They import the
// package as schema_test (black-box) and assert only on the public contract:
// given a migration file, Extract returns the expected []model.Table — table
// names, and each column's Name/Type/Nullable/IsPrimaryKey/IsForeignKey/
// References. AST internals and unexported helpers are deliberately not touched.

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/extract/schema"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// fk is a small helper to build a non-nil *ForeignKeyRef in expectations.
func fk(table, column string) *model.ForeignKeyRef {
	return &model.ForeignKeyRef{Table: table, Column: column}
}

// idx is a small helper to build a model.Index in expectations, mirroring fk.
func idx(unique bool, cols ...string) model.Index {
	return model.Index{Columns: cols, Unique: unique}
}

// idxNamed is idx with an explicit index name.
func idxNamed(name string, unique bool, cols ...string) model.Index {
	return model.Index{Name: name, Columns: cols, Unique: unique}
}

func TestExtract(t *testing.T) {
	tests := []struct {
		name    string   // sub-test name / pattern under test
		files   []string // testdata filenames, in extraction order
		want    []model.Table
		pattern string // human note on what behavior this pins down
	}{
		{
			name:    "create_simple",
			files:   []string{"create_simple.php"},
			pattern: "basic Schema::create with id() PK and a string column",
			want: []model.Table{
				{Name: "tags", Columns: []model.Column{
					{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
					{Name: "label", Type: "string"},
				}},
			},
		},
		{
			name:    "timestamps_expansion",
			files:   []string{"create_timestamps.php"},
			pattern: "timestamps() expands to created_at+updated_at; ->nullable() modifier",
			want: []model.Table{
				{Name: "sessions", Columns: []model.Column{
					{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
					{Name: "ip_address", Type: "string", Nullable: true},
					{Name: "created_at", Type: "timestamp", Nullable: true},
					{Name: "updated_at", Type: "timestamp", Nullable: true},
				}},
			},
		},
		{
			name:    "foreignid_fk",
			files:   []string{"create_foreignid.php"},
			pattern: "foreignId()->constrained() flags FK (bare constrained => no References)",
			want: []model.Table{
				{Name: "comments", Columns: []model.Column{
					{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
					{Name: "post_id", Type: "bigInteger", IsForeignKey: true},
					{Name: "body", Type: "text"},
				}},
			},
		},
		{
			name:    "alter_table_stub",
			files:   []string{"alter_table.php"},
			pattern: "Schema::table (ALTER) on an as-yet-unseen table creates a stub table with the altered column",
			want: []model.Table{
				{Name: "comments", Columns: []model.Column{
					{Name: "approved", Type: "boolean"},
				}},
			},
		},
		{
			name:    "classic_named_class",
			files:   []string{"classic_style.php"},
			pattern: "classic 'class X extends Migration' shape is matched on Schema:: calls, not class structure; the fixture's chained ->unique() also emits a table-level Index",
			want: []model.Table{
				{
					Name: "roles",
					Columns: []model.Column{
						{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
						{Name: "name", Type: "string"},
					},
					Indexes: []model.Index{idx(true, "name")},
				},
			},
		},
		{
			name:    "create_then_alter_merge",
			files:   []string{"create_foreignid.php", "alter_table.php"},
			pattern: "Schema::table in a later file merges (appends) columns into the existing table, preserving order",
			want: []model.Table{
				{Name: "comments", Columns: []model.Column{
					{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
					{Name: "post_id", Type: "bigInteger", IsForeignKey: true},
					{Name: "body", Type: "text"},
					{Name: "approved", Type: "boolean"},
				}},
			},
		},
		// --- foreign-key branch coverage ---
		{
			name:    "fk_explicit_references_on",
			files:   []string{"create_fk_explicit.php"},
			pattern: "foreignId()->references('id')->on('users') emits IsForeignKey=true with References{Table:users,Column:id}",
			want: []model.Table{
				{Name: "posts", Columns: []model.Column{
					{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
					{Name: "user_id", Type: "bigInteger", IsForeignKey: true,
						References: fk("users", "id")},
					{Name: "title", Type: "string"},
				}},
			},
		},
		{
			name:    "fk_constrained_with_explicit_table",
			files:   []string{"create_fk_constrained_table.php"},
			pattern: "foreignId()->constrained('teams') emits IsForeignKey=true with References{Table:teams,Column:''} — explicit table, implicit column",
			want: []model.Table{
				{Name: "memberships", Columns: []model.Column{
					{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
					{Name: "team_id", Type: "bigInteger", IsForeignKey: true,
						References: fk("teams", "")},
					{Name: "role", Type: "string"},
				}},
			},
		},
		// --- columnsForMethod edge-case coverage ---
		{
			name:    "uuid_no_arg_primary_key",
			files:   []string{"create_uuid_pk.php"},
			pattern: "uuid() with no argument is Laravel's UUID PK shortcut: Name=uuid, Type=uuid, IsPrimaryKey=true",
			want: []model.Table{
				{Name: "tokens", Columns: []model.Column{
					{Name: "uuid", Type: "uuid", IsPrimaryKey: true},
					{Name: "value", Type: "string"},
				}},
			},
		},
		// --- applyModifiers "primary" modifier coverage ---
		{
			name:    "primary_modifier",
			files:   []string{"create_with_primary_modifier.php"},
			pattern: "->primary() modifier sets IsPrimaryKey=true on a string column; ->nullable() sets Nullable=true",
			want: []model.Table{
				{Name: "settings", Columns: []model.Column{
					{Name: "key", Type: "string", IsPrimaryKey: true},
					{Name: "value", Type: "text", Nullable: true},
				}},
			},
		},
		// --- ExprStaticCall ignored-call branches ---
		{
			name:    "ignored_static_calls",
			files:   []string{"create_with_ignored_calls.php"},
			pattern: "Schema::drop() and non-Schema static calls (DB::statement) are silently ignored; only Schema::create columns are extracted",
			want: []model.Table{
				{Name: "profiles", Columns: []model.Column{
					{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
					{Name: "bio", Type: "string", Nullable: true},
				}},
			},
		},
		// --- index extraction coverage ---
		{
			name:    "chained_unique_modifier",
			files:   []string{"create_chained_unique.php"},
			pattern: "->unique() chained on a single-column builder emits a table-level unique Index over that column",
			want: []model.Table{
				{
					Name: "accounts",
					Columns: []model.Column{
						{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
						{Name: "email", Type: "string"},
					},
					Indexes: []model.Index{idx(true, "email")},
				},
			},
		},
		{
			name:    "chained_index_modifier",
			files:   []string{"create_chained_index.php"},
			pattern: "->index() chained on a single-column builder emits a table-level non-unique Index over that column",
			want: []model.Table{
				{
					Name: "logs",
					Columns: []model.Column{
						{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
						{Name: "level", Type: "string"},
					},
					Indexes: []model.Index{idx(false, "level")},
				},
			},
		},
		{
			name:    "standalone_index_call",
			files:   []string{"create_standalone_index.php"},
			pattern: "$table->index(['a','b'], 'idx_a_b') emits a named, non-unique, composite Index",
			want: []model.Table{
				{
					Name: "events",
					Columns: []model.Column{
						{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
						{Name: "a", Type: "string"},
						{Name: "b", Type: "string"},
					},
					Indexes: []model.Index{idxNamed("idx_a_b", false, "a", "b")},
				},
			},
		},
		{
			name:    "standalone_unique_call_no_name",
			files:   []string{"create_standalone_unique.php"},
			pattern: "$table->unique(['x']) with no name argument emits an Index with an empty Name",
			want: []model.Table{
				{
					Name: "widgets",
					Columns: []model.Column{
						{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
						{Name: "x", Type: "string"},
					},
					Indexes: []model.Index{idx(true, "x")},
				},
			},
		},
		{
			name:    "standalone_composite_primary_call",
			files:   []string{"create_standalone_primary.php"},
			pattern: "$table->primary(['a','b']) emits a unique composite Index and does NOT set IsPrimaryKey on any column",
			want: []model.Table{
				{
					Name: "role_user",
					Columns: []model.Column{
						{Name: "a", Type: "string"},
						{Name: "b", Type: "string"},
					},
					Indexes: []model.Index{idx(true, "a", "b")},
				},
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

			got, err := schema.Extract(paths)
			if err != nil {
				t.Fatalf("Extract(%v) returned error: %v", tt.files, err)
			}

			assertTables(t, got, tt.want)
		})
	}
}

// TestExtractDir exercises the directory-discovery wrapper end to end against
// the same testdata snippets. Because ExtractDir sorts lexically, the alter
// file (alter_table.php) sorts before create_foreignid.php, so the merge runs
// in the "alter-first" direction: the stub is created by the alter, then the
// create appends. This pins down that ordering behavior.
func TestExtractDir(t *testing.T) {
	got, err := schema.ExtractDir("testdata")
	if err != nil {
		t.Fatalf("ExtractDir returned error: %v", err)
	}

	byName := make(map[string]model.Table, len(got))
	for _, tb := range got {
		byName[tb.Name] = tb
	}

	// Sanity: the distinct tables across all snippets must be present.
	for _, name := range []string{"tags", "sessions", "comments", "roles"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("ExtractDir: expected a %q table, got tables %v", name, tableNames(got))
		}
	}

	// comments is defined across alter_table.php (sorts first) and
	// create_foreignid.php; lexical order makes the alter stub come first.
	wantComments := []model.Column{
		{Name: "approved", Type: "boolean"},
		{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
		{Name: "post_id", Type: "bigInteger", IsForeignKey: true},
		{Name: "body", Type: "text"},
	}
	if diff := columnsEqual(byName["comments"].Columns, wantComments); diff != "" {
		t.Errorf("ExtractDir comments columns mismatch: %s", diff)
	}
}

func TestExtractDir_missingDir(t *testing.T) {
	if _, err := schema.ExtractDir(filepath.Join("testdata", "does-not-exist")); err == nil {
		t.Fatal("ExtractDir on a missing directory: expected error, got nil")
	}
}

// assertTables compares produced tables against expectations by name and by
// column, with focused error messages so a failure points at the exact field.
func assertTables(t *testing.T, got, want []model.Table) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("table count = %d, want %d (got %v, want %v)",
			len(got), len(want), tableNames(got), tableNames(want))
	}

	for i := range want {
		if got[i].Name != want[i].Name {
			t.Errorf("table[%d].Name = %q, want %q", i, got[i].Name, want[i].Name)
			continue
		}
		if diff := columnsEqual(got[i].Columns, want[i].Columns); diff != "" {
			t.Errorf("table %q columns mismatch: %s", want[i].Name, diff)
		}
		if diff := indexesEqual(got[i].Indexes, want[i].Indexes); diff != "" {
			t.Errorf("table %q indexes mismatch: %s", want[i].Name, diff)
		}
	}
}

// indexesEqual returns "" when the two index slices are equal, or a
// human-readable description of the first difference otherwise. Only test
// cases that populate want.Indexes assert on it; cases that leave it nil
// compare against a nil got only when Extract also produces none — a table
// that this PRD adds no index coverage for still has a non-nil-but-empty
// Indexes slice (via model.NewTable), so len(nil) == len([]) == 0 keeps
// pre-existing test cases passing unchanged.
func indexesEqual(got, want []model.Index) string {
	if len(got) != len(want) {
		return "count = " + itoa(len(got)) + ", want " + itoa(len(want)) +
			" (got " + indexNames(got) + ", want " + indexNames(want) + ")"
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			return "index[" + itoa(i) + "] = " + describeIndex(got[i]) +
				", want " + describeIndex(want[i])
		}
	}
	return ""
}

func indexNames(idxs []model.Index) string {
	s := "["
	for i, ix := range idxs {
		if i > 0 {
			s += " "
		}
		s += describeIndex(ix)
	}
	return s + "]"
}

func describeIndex(ix model.Index) string {
	s := "{"
	if ix.Name != "" {
		s += ix.Name + ":"
	}
	for i, c := range ix.Columns {
		if i > 0 {
			s += ","
		}
		s += c
	}
	if ix.Unique {
		s += " unique"
	}
	return s + "}"
}

// columnsEqual returns "" when the two column slices are equal, or a
// human-readable description of the first difference otherwise.
func columnsEqual(got, want []model.Column) string {
	if len(got) != len(want) {
		return "count = " + itoa(len(got)) + ", want " + itoa(len(want)) +
			" (got " + columnNames(got) + ", want " + columnNames(want) + ")"
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			return "column[" + itoa(i) + "] = " + describeColumn(got[i]) +
				", want " + describeColumn(want[i])
		}
	}
	return ""
}

func tableNames(ts []model.Table) []string {
	out := make([]string, len(ts))
	for i, tb := range ts {
		out[i] = tb.Name
	}
	return out
}

func columnNames(cs []model.Column) string {
	s := "["
	for i, c := range cs {
		if i > 0 {
			s += " "
		}
		s += c.Name
	}
	return s + "]"
}

func describeColumn(c model.Column) string {
	s := c.Name + ":" + c.Type
	if c.Nullable {
		s += " null"
	}
	if c.IsPrimaryKey {
		s += " pk"
	}
	if c.IsForeignKey {
		s += " fk"
	}
	if c.References != nil {
		s += " ->" + c.References.Table + "." + c.References.Column
	}
	return s
}

// itoa is a tiny dependency-free int-to-string to keep the test helpers free of
// fmt noise in hot error paths.
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
