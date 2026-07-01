package formrequest

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	domain "github.com/Mawsis/Un-La-ravel/internal/model"
)

// The fixtures under testdata/ each isolate one behaviour of the extractor: the
// string-syntax ruleset (string_rules), the array-syntax ruleset (array_rules),
// rules with arguments across both syntaxes (rule_args), the empty-but-non-nil
// Fields guarantee (empty_rules), the non-FormRequest class that must be ignored
// (not_a_form_request), unknown/custom rule preservation (unknown_rule), and the
// fully-qualified `extends` branch of recognition (fqn_extends). Fixture names
// are constants so a rename fails to compile here rather than silently skipping.
const (
	fixtureStringRules     = "string_rules.php"
	fixtureArrayRules      = "array_rules.php"
	fixtureRuleArgs        = "rule_args.php"
	fixtureEmptyRules      = "empty_rules.php"
	fixtureNotAFormRequest = "not_a_form_request.php"
	fixtureUnknownRule     = "unknown_rule.php"
	fixtureFQNExtends      = "fqn_extends.php"
)

// td returns the testdata path for a fixture file name.
func td(name string) string { return filepath.Join("testdata", name) }

// only returns the single FormRequest in frs, failing the test if the count is
// not exactly one — every single-class fixture must emit exactly one node.
func only(t *testing.T, frs []domain.FormRequest) domain.FormRequest {
	t.Helper()
	if len(frs) != 1 {
		t.Fatalf("want exactly 1 FormRequest, got %d: %+v", len(frs), frs)
	}
	return frs[0]
}

// TestExtract is the table-driven core: each case parses one fixture and asserts
// the ENTIRE emitted FormRequest struct (Name, FQN, and every Field with its
// ordered Rules) against a hand-written expectation, so the exact rule-token
// parsing — '|'-split vs array elements, single-arg vs multi-arg, args as raw
// strings — is pinned, not just field counts.
func TestExtract(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    domain.FormRequest
	}{
		{
			// String syntax: each value is one '|'-delimited string split into
			// tokens. 'max:255' yields a single raw-string arg "255" (the renderer,
			// not the extractor, converts it to a number).
			name:    "string ruleset splits on pipe",
			fixture: fixtureStringRules,
			want: domain.FormRequest{
				Name: "StoreArticleRequest",
				FQN:  `App\Http\Requests\StoreArticleRequest`,
				Fields: []domain.Field{
					{Name: "title", Rules: []domain.Rule{
						{Name: "required"},
						{Name: "string"},
						{Name: "max", Args: []string{"255"}},
					}},
					{Name: "body", Rules: []domain.Rule{
						{Name: "required"},
						{Name: "string"},
					}},
				},
			},
		},
		{
			// Array syntax: each value is a PHP array whose string elements are the
			// tokens (one Rule per element, NOT '|'-split). 'in:draft,published'
			// still splits its arg portion on ',' into two args.
			name:    "array ruleset one rule per element",
			fixture: fixtureArrayRules,
			want: domain.FormRequest{
				Name: "UpdateArticleRequest",
				FQN:  `App\Http\Requests\UpdateArticleRequest`,
				Fields: []domain.Field{
					{Name: "status", Rules: []domain.Rule{
						{Name: "nullable"},
						{Name: "string"},
						{Name: "in", Args: []string{"draft", "published"}},
					}},
					{Name: "slug", Rules: []domain.Rule{
						{Name: "required"},
						{Name: "string"},
					}},
				},
			},
		},
		{
			// Rules with arguments across both syntaxes: single-arg ('max:255'),
			// multi-arg list ('in:a,b,c'), and 'between:1,10' — all kept as raw
			// string args in source order.
			name:    "rule arguments split from colon suffix",
			fixture: fixtureRuleArgs,
			want: domain.FormRequest{
				Name: "FilterRequest",
				FQN:  `App\Http\Requests\FilterRequest`,
				Fields: []domain.Field{
					{Name: "size", Rules: []domain.Rule{
						{Name: "required"},
						{Name: "max", Args: []string{"255"}},
					}},
					{Name: "kind", Rules: []domain.Rule{
						{Name: "in", Args: []string{"a", "b", "c"}},
					}},
					{Name: "count", Rules: []domain.Rule{
						{Name: "integer"},
						{Name: "between", Args: []string{"1", "10"}},
					}},
				},
			},
		},
		{
			// Unknown/custom rules ('alpha_dash', 'uppercase') are preserved as
			// plain tokens (Name set, no Args), not dropped, in both syntaxes.
			name:    "unknown rules preserved source-faithfully",
			fixture: fixtureUnknownRule,
			want: domain.FormRequest{
				Name: "StoreCouponRequest",
				FQN:  `App\Http\Requests\StoreCouponRequest`,
				Fields: []domain.Field{
					{Name: "code", Rules: []domain.Rule{
						{Name: "required"},
						{Name: "alpha_dash"},
					}},
					{Name: "label", Rules: []domain.Rule{
						{Name: "required"},
						{Name: "uppercase"},
					}},
				},
			},
		},
		{
			// A fully-qualified `extends Illuminate\Foundation\Http\FormRequest`
			// (no `use`) is recognised via the parent name's last segment, exactly
			// like the unqualified form.
			name:    "fully qualified extends is recognised",
			fixture: fixtureFQNExtends,
			want: domain.FormRequest{
				Name: "StoreTagRequest",
				FQN:  `App\Http\Requests\StoreTagRequest`,
				Fields: []domain.Field{
					{Name: "name", Rules: []domain.Rule{
						{Name: "required"},
						{Name: "string"},
						{Name: "max", Args: []string{"50"}},
					}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frs, err := Extract([]string{td(tt.fixture)})
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			got := only(t, frs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FormRequest mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// TestExtractEmptyRulesYieldsEmptyNonNilFields proves a rules() returning [] is
// emitted as a FormRequest whose Fields is an empty but non-nil slice (so it
// serializes "fields": [] rather than null), never a crash.
func TestExtractEmptyRulesYieldsEmptyNonNilFields(t *testing.T) {
	frs, err := Extract([]string{td(fixtureEmptyRules)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	fr := only(t, frs)

	if fr.Name != "EmptyRequest" {
		t.Errorf("Name = %q, want EmptyRequest", fr.Name)
	}
	if fr.FQN != `App\Http\Requests\EmptyRequest` {
		t.Errorf("FQN = %q, want App\\Http\\Requests\\EmptyRequest", fr.FQN)
	}
	if fr.Fields == nil {
		t.Fatalf("Fields must be a non-nil empty slice, got nil (would serialize as null)")
	}
	if len(fr.Fields) != 0 {
		t.Errorf("Fields = %+v, want empty", fr.Fields)
	}
}

// TestExtractIgnoresNonFormRequest proves recognition is by the parent class,
// not by the directory or the mere presence of a rules() method: a plain class
// that does not extend a FormRequest base emits no node, even with a rules()
// body.
func TestExtractIgnoresNonFormRequest(t *testing.T) {
	frs, err := Extract([]string{td(fixtureNotAFormRequest)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(frs) != 0 {
		t.Fatalf("a non-FormRequest class must emit no node, got %d: %+v", len(frs), frs)
	}
}

// TestExtractProcessesPathsInOrder proves Extract preserves the given path order
// in the emitted slice, so the caller controls discovery order.
func TestExtractProcessesPathsInOrder(t *testing.T) {
	frs, err := Extract([]string{td(fixtureFQNExtends), td(fixtureStringRules)})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(frs) != 2 {
		t.Fatalf("want 2 FormRequests, got %d", len(frs))
	}
	if frs[0].Name != "StoreTagRequest" || frs[1].Name != "StoreArticleRequest" {
		t.Errorf("order = [%q, %q], want [StoreTagRequest, StoreArticleRequest]", frs[0].Name, frs[1].Name)
	}
}

// TestExtractEmptyPathsYieldsNoRequests proves the empty-input base case: no
// paths, no error, no nodes.
func TestExtractEmptyPathsYieldsNoRequests(t *testing.T) {
	frs, err := Extract(nil)
	if err != nil {
		t.Fatalf("Extract(nil): %v", err)
	}
	if len(frs) != 0 {
		t.Fatalf("Extract(nil) = %v, want no FormRequests", frs)
	}
}

// TestExtractMissingFileIsWrappedError proves an unreadable file aborts the whole
// extraction with an error wrapped in the package prefix and offending path (%w
// chain, ADR 0003): a silently dropped FormRequest would leave a route's request
// body undocumented.
func TestExtractMissingFileIsWrappedError(t *testing.T) {
	_, err := Extract([]string{td("does_not_exist.php")})
	if err == nil {
		t.Fatalf("Extract of a missing file must return an error")
	}
	if !strings.Contains(err.Error(), "formrequest: extract") {
		t.Fatalf("error %q missing the wrapped context prefix", err)
	}
	if !strings.Contains(err.Error(), "does_not_exist.php") {
		t.Fatalf("error %q should name the offending path", err)
	}
}
