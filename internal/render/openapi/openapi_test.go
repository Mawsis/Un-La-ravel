package openapi

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mawsis/unlaravel/internal/model"
)

// update regenerates the golden document when the test is run with -update:
//
//	go test ./internal/render/openapi -run TestRenderGolden -update
//
// The committed golden is the source of truth; -update only exists to
// regenerate it intentionally after a deliberate change to the Renderer.
var update = flag.Bool("update", false, "regenerate golden files")

// goldenPath is the on-disk location of the expected OpenAPI JSON document.
const goldenPath = "testdata/openapi.json"

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// rule is a terse constructor for a model.Rule, keeping the table-driven cases
// compact and readable.
func rule(name string, args ...string) model.Rule {
	if len(args) == 0 {
		return model.Rule{Name: name}
	}
	return model.Rule{Name: name, Args: args}
}

// renderDoc renders a model and unmarshals the result into a generic map, so
// tests can assert on the JSON structure without depending on the private
// document structs. It fails the test on any render or parse error.
func renderDoc(t *testing.T, m *model.ProjectModel) map[string]any {
	t.Helper()
	out, err := Render(m)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("Render output is not valid JSON: %v\n%s", err, out)
	}
	return doc
}

// ---------------------------------------------------------------------------
// fieldSchema: the rules → JSON-Schema mapping (the decision-dense core)
// ---------------------------------------------------------------------------

// intPtr / floatPtr build pointers for the expected numeric-bound fields, since
// the schema struct uses pointers so a zero bound is still emitted.
func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestFieldSchema(t *testing.T) {
	tests := []struct {
		name         string
		rules        []model.Rule
		wantSchema   schema
		wantRequired bool
	}{
		{
			name:         "no rules defaults to string",
			rules:        nil,
			wantSchema:   schema{Type: schemaTypeString},
			wantRequired: false,
		},
		{
			name:         "required is a flag not a property",
			rules:        []model.Rule{rule("required"), rule("string")},
			wantSchema:   schema{Type: schemaTypeString},
			wantRequired: true,
		},
		{
			name:         "nullable sets nullable true",
			rules:        []model.Rule{rule("nullable"), rule("string")},
			wantSchema:   schema{Type: schemaTypeString, Nullable: true},
			wantRequired: false,
		},
		{
			name:         "sometimes also sets nullable true",
			rules:        []model.Rule{rule("sometimes"), rule("integer")},
			wantSchema:   schema{Type: schemaTypeInteger, Nullable: true},
			wantRequired: false,
		},
		{
			name:         "integer type",
			rules:        []model.Rule{rule("integer")},
			wantSchema:   schema{Type: schemaTypeInteger},
			wantRequired: false,
		},
		{
			name:         "numeric maps to number",
			rules:        []model.Rule{rule("numeric")},
			wantSchema:   schema{Type: schemaTypeNumber},
			wantRequired: false,
		},
		{
			name:         "boolean type",
			rules:        []model.Rule{rule("boolean")},
			wantSchema:   schema{Type: schemaTypeBoolean},
			wantRequired: false,
		},
		{
			name:         "array type",
			rules:        []model.Rule{rule("array")},
			wantSchema:   schema{Type: schemaTypeArray},
			wantRequired: false,
		},
		{
			name:         "email format on string",
			rules:        []model.Rule{rule("email")},
			wantSchema:   schema{Type: schemaTypeString, Format: formatEmail},
			wantRequired: false,
		},
		{
			name:         "url maps to uri format",
			rules:        []model.Rule{rule("url")},
			wantSchema:   schema{Type: schemaTypeString, Format: formatURI},
			wantRequired: false,
		},
		{
			name:         "uuid format",
			rules:        []model.Rule{rule("uuid")},
			wantSchema:   schema{Type: schemaTypeString, Format: formatUUID},
			wantRequired: false,
		},
		{
			name:         "date format",
			rules:        []model.Rule{rule("date")},
			wantSchema:   schema{Type: schemaTypeString, Format: formatDate},
			wantRequired: false,
		},
		{
			name:         "in becomes enum",
			rules:        []model.Rule{rule("in", "draft", "published")},
			wantSchema:   schema{Type: schemaTypeString, Enum: []string{"draft", "published"}},
			wantRequired: false,
		},
		{
			// BUG 1: max on a string must emit maxLength as a NUMBER, not "255".
			name:         "max on string is numeric maxLength",
			rules:        []model.Rule{rule("string"), rule("max", "255")},
			wantSchema:   schema{Type: schemaTypeString, MaxLength: intPtr(255)},
			wantRequired: false,
		},
		{
			name:         "min on string is numeric minLength",
			rules:        []model.Rule{rule("string"), rule("min", "3")},
			wantSchema:   schema{Type: schemaTypeString, MinLength: intPtr(3)},
			wantRequired: false,
		},
		{
			// BUG 1 + numeric dispatch: max on an integer must be maximum (number).
			name:         "max on integer is numeric maximum",
			rules:        []model.Rule{rule("integer"), rule("max", "100")},
			wantSchema:   schema{Type: schemaTypeInteger, Maximum: floatPtr(100)},
			wantRequired: false,
		},
		{
			name:         "min on number is numeric minimum",
			rules:        []model.Rule{rule("numeric"), rule("min", "0")},
			wantSchema:   schema{Type: schemaTypeNumber, Minimum: floatPtr(0)},
			wantRequired: false,
		},
		{
			// BUG 2: rules in REVERSE order (max before the type) must still
			// dispatch to maximum, not maxLength — proving the two-pass design.
			name:         "max before type still dispatches to maximum",
			rules:        []model.Rule{rule("max", "100"), rule("integer")},
			wantSchema:   schema{Type: schemaTypeInteger, Maximum: floatPtr(100)},
			wantRequired: false,
		},
		{
			// BUG 2: with no type declared, max defaults to string-length bound.
			name:         "max with default string type is maxLength",
			rules:        []model.Rule{rule("max", "50")},
			wantSchema:   schema{Type: schemaTypeString, MaxLength: intPtr(50)},
			wantRequired: false,
		},
		{
			name:         "unknown rule preserved in description",
			rules:        []model.Rule{rule("string"), rule("exists", "posts", "id")},
			wantSchema:   schema{Type: schemaTypeString, Description: "rule: exists"},
			wantRequired: false,
		},
		{
			name: "multiple unknown rules joined in description",
			rules: []model.Rule{
				rule("string"),
				rule("confirmed"),
				rule("regex", "/foo/"),
			},
			wantSchema:   schema{Type: schemaTypeString, Description: "rule: confirmed; rule: regex"},
			wantRequired: false,
		},
		{
			name: "kitchen sink required string with format and bounds",
			rules: []model.Rule{
				rule("required"),
				rule("string"),
				rule("email"),
				rule("max", "255"),
				rule("min", "5"),
			},
			wantSchema: schema{
				Type:      schemaTypeString,
				Format:    formatEmail,
				MaxLength: intPtr(255),
				MinLength: intPtr(5),
			},
			wantRequired: true,
		},
		{
			name:         "malformed max with no arg is skipped not crashed",
			rules:        []model.Rule{rule("string"), rule("max")},
			wantSchema:   schema{Type: schemaTypeString},
			wantRequired: false,
		},
		{
			name:         "non-numeric max arg is skipped",
			rules:        []model.Rule{rule("integer"), rule("max", "lots")},
			wantSchema:   schema{Type: schemaTypeInteger},
			wantRequired: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSchema, gotRequired := fieldSchema(tc.rules)
			if !reflect.DeepEqual(gotSchema, tc.wantSchema) {
				t.Errorf("schema mismatch\n got: %+v\nwant: %+v", gotSchema, tc.wantSchema)
			}
			if gotRequired != tc.wantRequired {
				t.Errorf("required = %v, want %v", gotRequired, tc.wantRequired)
			}
		})
	}
}

// TestFieldSchema_NumericBoundsAreJSONNumbers is the direct regression guard for
// BUG 1: it renders a schema with a maxLength and asserts the serialized JSON
// carries a bare number, never a quoted string.
func TestFieldSchema_NumericBoundsAreJSONNumbers(t *testing.T) {
	prop, _ := fieldSchema([]model.Rule{rule("string"), rule("max", "255")})
	out, err := json.Marshal(prop)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	if !bytes.Contains(out, []byte(`"maxLength":255`)) {
		t.Errorf("expected numeric maxLength; got %s", out)
	}
	if bytes.Contains(out, []byte(`"255"`)) {
		t.Errorf("maxLength must not be a string; got %s", out)
	}
}

// ---------------------------------------------------------------------------
// pathParameterNames
// ---------------------------------------------------------------------------

func TestPathParameterNames(t *testing.T) {
	tests := []struct {
		uri  string
		want []string
	}{
		{"/posts", nil},
		{"/posts/{post}", []string{"post"}},
		{"/posts/{post}/comments/{comment}", []string{"post", "comment"}},
		{"/users/{id?}", []string{"id"}},
		{"/a/{x}/b/{x}", []string{"x"}},
		{"/broken/{unclosed", nil},
	}
	for _, tc := range tests {
		t.Run(tc.uri, func(t *testing.T) {
			got := pathParameterNames(tc.uri)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("pathParameterNames(%q) = %v, want %v", tc.uri, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Render: whole-document shape
// ---------------------------------------------------------------------------

// TestRender_NilModel confirms a nil model yields a valid, empty document.
func TestRender_NilModel(t *testing.T) {
	doc := renderDoc(t, nil)
	if doc["openapi"] != openAPIVersion {
		t.Errorf("openapi = %v, want %v", doc["openapi"], openAPIVersion)
	}
	info, _ := doc["info"].(map[string]any)
	if info["title"] != defaultInfoTitle || info["version"] != defaultInfoVersion {
		t.Errorf("info defaults wrong: %+v", info)
	}
	paths, _ := doc["paths"].(map[string]any)
	if len(paths) != 0 {
		t.Errorf("expected no paths, got %v", paths)
	}
}

// TestRender_InfoFromModel confirms the project name and Laravel version flow
// into the info block.
func TestRender_InfoFromModel(t *testing.T) {
	m := model.New("Blog", "11.9.0")
	doc := renderDoc(t, m)
	info, _ := doc["info"].(map[string]any)
	if info["title"] != "Blog" {
		t.Errorf("title = %v, want Blog", info["title"])
	}
	if info["version"] != "11.9.0" {
		t.Errorf("version = %v, want 11.9.0", info["version"])
	}
}

// TestRender_PostWithFormRequestBody is the showpiece assertion: a POST /posts
// route linked to a StorePostRequest yields a requestBody whose schema is the
// object built from the form request's fields.
func TestRender_PostWithFormRequestBody(t *testing.T) {
	fr := model.NewFormRequest("StorePostRequest", "App\\Http\\Requests\\StorePostRequest")
	fr.Fields = []model.Field{
		{Name: "title", Rules: []model.Rule{rule("required"), rule("string"), rule("max", "255")}},
		{Name: "status", Rules: []model.Rule{rule("required"), rule("in", "draft", "published")}},
		{Name: "views", Rules: []model.Rule{rule("nullable"), rule("integer"), rule("min", "0")}},
	}

	m := model.New("Blog", "11.9.0")
	m.AddFormRequest(fr)
	m.AddRoute(model.Route{
		Method:      "POST",
		URI:         "/posts",
		Controller:  "PostController",
		Action:      "store",
		FormRequest: "App\\Http\\Requests\\StorePostRequest",
	})

	doc := renderDoc(t, m)
	paths, _ := doc["paths"].(map[string]any)
	post, _ := paths["/posts"].(map[string]any)
	op, _ := post["post"].(map[string]any)
	if op == nil {
		t.Fatalf("no POST operation under /posts: %+v", paths)
	}
	if op["operationId"] != "PostController_store" {
		t.Errorf("operationId = %v", op["operationId"])
	}

	body, _ := op["requestBody"].(map[string]any)
	if body == nil {
		t.Fatalf("expected requestBody, got %+v", op)
	}
	if body["required"] != true {
		t.Errorf("requestBody.required = %v, want true", body["required"])
	}

	content, _ := body["content"].(map[string]any)
	appJSON, _ := content[jsonMediaType].(map[string]any)
	sch, _ := appJSON["schema"].(map[string]any)
	if sch["type"] != schemaTypeObject {
		t.Errorf("schema.type = %v, want object", sch["type"])
	}

	required, _ := sch["required"].([]any)
	if len(required) != 2 || required[0] != "title" || required[1] != "status" {
		t.Errorf("required = %v, want [title status]", required)
	}

	props, _ := sch["properties"].(map[string]any)
	title, _ := props["title"].(map[string]any)
	if title["type"] != schemaTypeString {
		t.Errorf("title.type = %v", title["type"])
	}
	// BUG 1 guard at the document level: maxLength is a JSON number.
	if ml, ok := title["maxLength"].(float64); !ok || ml != 255 {
		t.Errorf("title.maxLength = %v (%T), want number 255", title["maxLength"], title["maxLength"])
	}
}

// TestRender_RouteWithoutFormRequestHasNoBody confirms a route with no linked
// FormRequest carries no requestBody.
func TestRender_RouteWithoutFormRequestHasNoBody(t *testing.T) {
	m := model.New("Blog", "11.9.0")
	m.AddRoute(model.Route{Method: "GET", URI: "/posts/{post}", Controller: "PostController", Action: "show"})

	doc := renderDoc(t, m)
	paths, _ := doc["paths"].(map[string]any)
	item, _ := paths["/posts/{post}"].(map[string]any)
	op, _ := item["get"].(map[string]any)
	if _, has := op["requestBody"]; has {
		t.Errorf("GET route should have no requestBody, got %+v", op)
	}

	params, _ := op["parameters"].([]any)
	if len(params) != 1 {
		t.Fatalf("expected 1 path parameter, got %v", params)
	}
	p0, _ := params[0].(map[string]any)
	if p0["name"] != "post" || p0["in"] != "path" || p0["required"] != true {
		t.Errorf("path parameter wrong: %+v", p0)
	}
}

// TestRender_MultipleMethodsSamePath confirms two verbs on the same URI collapse
// into one path item with two method keys.
func TestRender_MultipleMethodsSamePath(t *testing.T) {
	m := model.New("Blog", "11.9.0")
	m.AddRoute(model.Route{Method: "GET", URI: "/posts", Controller: "PostController", Action: "index"})
	m.AddRoute(model.Route{Method: "POST", URI: "/posts", Controller: "PostController", Action: "store"})

	doc := renderDoc(t, m)
	paths, _ := doc["paths"].(map[string]any)
	item, _ := paths["/posts"].(map[string]any)
	if _, has := item["get"]; !has {
		t.Errorf("missing get: %+v", item)
	}
	if _, has := item["post"]; !has {
		t.Errorf("missing post: %+v", item)
	}
}

// TestRender_Deterministic confirms two renders of the same model are
// byte-identical, which golden-file tests rely on.
func TestRender_Deterministic(t *testing.T) {
	build := func() []byte {
		m := model.New("Blog", "11.9.0")
		for _, uri := range []string{"/z", "/a", "/m"} {
			m.AddRoute(model.Route{Method: "GET", URI: uri, Controller: "C", Action: "index"})
			m.AddRoute(model.Route{Method: "POST", URI: uri, Controller: "C", Action: "store"})
		}
		out, err := Render(m)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return out
	}
	if !bytes.Equal(build(), build()) {
		t.Error("Render output is not deterministic across runs")
	}
}

// TestRender_OperationIDFromRouteName confirms an explicit route name wins over
// the controller_action synthesis.
func TestRender_OperationIDFromRouteName(t *testing.T) {
	m := model.New("Blog", "11.9.0")
	m.AddRoute(model.Route{Method: "GET", URI: "/posts", Controller: "PostController", Action: "index", Name: "posts.index"})
	doc := renderDoc(t, m)
	paths, _ := doc["paths"].(map[string]any)
	item, _ := paths["/posts"].(map[string]any)
	op, _ := item["get"].(map[string]any)
	if op["operationId"] != "posts.index" {
		t.Errorf("operationId = %v, want posts.index", op["operationId"])
	}
}

// TestRender_FormRequestByShortName confirms the link resolves when the analyzer
// set Route.FormRequest to the short class name rather than the FQN.
func TestRender_FormRequestByShortName(t *testing.T) {
	fr := model.NewFormRequest("StorePostRequest", "App\\Http\\Requests\\StorePostRequest")
	fr.Fields = []model.Field{{Name: "title", Rules: []model.Rule{rule("required"), rule("string")}}}
	m := model.New("Blog", "11.9.0")
	m.AddFormRequest(fr)
	m.AddRoute(model.Route{Method: "POST", URI: "/posts", Controller: "PostController", Action: "store", FormRequest: "StorePostRequest"})

	doc := renderDoc(t, m)
	paths, _ := doc["paths"].(map[string]any)
	item, _ := paths["/posts"].(map[string]any)
	op, _ := item["post"].(map[string]any)
	if _, has := op["requestBody"]; !has {
		t.Errorf("expected requestBody resolved by short name, got %+v", op)
	}
}

// ---------------------------------------------------------------------------
// Golden document + structural validity
// ---------------------------------------------------------------------------

// buildFixtureModel constructs a small but representative Project Model that
// exercises the Renderer's externally observable behavior across every branch of
// the OpenAPI contract:
//
//   - GET /posts (PostController@index): a collection route with no path
//     parameter and no request body.
//   - POST /posts (PostController@store, StorePostRequest): a route whose action
//     links a FormRequest, so the operation carries a JSON requestBody whose
//     schema is derived from the request's rules — covering required flags, a
//     string with a numeric maxLength bound (BUG 1), an enum from in:, a nullable
//     integer with a numeric minimum (BUG 2 — the bound must dispatch by the
//     resolved integer type), and an unknown rule preserved in a description.
//   - GET /posts/{post} (PostController@show): a route with a required string
//     path parameter and no body.
//   - PUT /posts/{post} (PostController@update, StorePostRequest): the same URI
//     as show but a different verb — proving two methods collapse under one path
//     item — that ALSO carries a path parameter AND a request body at once.
//   - GET /users/{id?} (UserController@show, name "users.show"): an optional path
//     parameter (the "?" is stripped) and an explicit route name that drives the
//     operationId.
//
// The FormRequest is registered once and linked by FQN from POST and by short
// name from PUT, so the golden pins both resolution spellings. It is a pure
// in-memory fixture: no files, no parser, no I/O.
func buildFixtureModel() *model.ProjectModel {
	store := model.NewFormRequest("StorePostRequest", "App\\Http\\Requests\\StorePostRequest")
	store.Fields = []model.Field{
		{Name: "title", Rules: []model.Rule{rule("required"), rule("string"), rule("max", "255")}},
		{Name: "status", Rules: []model.Rule{rule("required"), rule("in", "draft", "published")}},
		{Name: "views", Rules: []model.Rule{rule("nullable"), rule("integer"), rule("min", "0")}},
		{Name: "slug", Rules: []model.Rule{rule("string"), rule("unique", "posts")}},
	}

	m := model.New("Blog", "11.9.0")
	m.AddFormRequest(store)
	m.AddRoute(model.Route{Method: "GET", URI: "/posts", Controller: "PostController", Action: "index"})
	m.AddRoute(model.Route{
		Method: "POST", URI: "/posts", Controller: "PostController", Action: "store",
		FormRequest: "App\\Http\\Requests\\StorePostRequest",
	})
	m.AddRoute(model.Route{Method: "GET", URI: "/posts/{post}", Controller: "PostController", Action: "show"})
	m.AddRoute(model.Route{
		Method: "PUT", URI: "/posts/{post}", Controller: "PostController", Action: "update",
		FormRequest: "StorePostRequest",
	})
	m.AddRoute(model.Route{
		Method: "GET", URI: "/users/{id?}", Controller: "UserController", Action: "show",
		Name: "users.show",
	})
	return m
}

// TestRenderGolden asserts external behavior: a fixed Project Model renders to
// exactly the bytes stored in the golden file. This pins the OpenAPI JSON output
// contract so any unintended change to the Renderer is caught as a diff.
func TestRenderGolden(t *testing.T) {
	got, err := Render(buildFixtureModel())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	if *update {
		writeGolden(t, got)
		t.Logf("updated golden file: %s", goldenPath)
	}

	want := readGolden(t)
	if !bytes.Equal(got, want) {
		t.Errorf("rendered OpenAPI does not match golden file.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestRenderGoldenIsValidOpenAPI asserts the golden fixture renders a
// structurally valid OpenAPI 3.0.3 document, independently of the byte-for-byte
// golden comparison. It checks the required top-level keys are present, the
// version and info block are well-formed, and — the structural invariant the
// task names — that every path item declares at least one HTTP-method operation
// and that every such operation carries a non-empty responses object. This guards
// the document's validity even if the golden bytes are regenerated.
func TestRenderGoldenIsValidOpenAPI(t *testing.T) {
	doc := renderDoc(t, buildFixtureModel())

	// Required top-level keys.
	for _, key := range []string{"openapi", "info", "paths"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("document missing required top-level key %q", key)
		}
	}
	if doc["openapi"] != openAPIVersion {
		t.Errorf("openapi = %v, want %v", doc["openapi"], openAPIVersion)
	}

	info, ok := doc["info"].(map[string]any)
	if !ok {
		t.Fatalf("info is not an object: %T", doc["info"])
	}
	if s, _ := info["title"].(string); s == "" {
		t.Error("info.title must be a non-empty string")
	}
	if s, _ := info["version"].(string); s == "" {
		t.Error("info.version must be a non-empty string")
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths is not an object: %T", doc["paths"])
	}
	if len(paths) == 0 {
		t.Fatal("fixture must render at least one path")
	}

	// The structural invariant: every path has >=1 method operation, and every
	// operation carries a non-empty responses object.
	httpMethods := map[string]struct{}{
		"get": {}, "post": {}, "put": {}, "patch": {}, "delete": {},
		"head": {}, "options": {}, "trace": {},
	}
	for uri, raw := range paths {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Errorf("path %q is not an object: %T", uri, raw)
			continue
		}
		operations := 0
		for method, opRaw := range item {
			if _, isMethod := httpMethods[method]; !isMethod {
				t.Errorf("path %q has unexpected method key %q", uri, method)
				continue
			}
			operations++
			op, ok := opRaw.(map[string]any)
			if !ok {
				t.Errorf("path %q method %q is not an object: %T", uri, method, opRaw)
				continue
			}
			responses, ok := op["responses"].(map[string]any)
			if !ok || len(responses) == 0 {
				t.Errorf("path %q method %q must carry a non-empty responses object, got %v", uri, method, op["responses"])
			}
		}
		if operations == 0 {
			t.Errorf("path %q declares no HTTP-method operation", uri)
		}
	}
}

// readGolden loads the expected document, failing the test with a clear,
// actionable message if it is missing (it must be regenerated with -update).
func readGolden(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with -update to generate it)", goldenPath, err)
	}
	return data
}

// writeGolden persists the rendered document as the new golden, creating the
// testdata directory if needed.
func writeGolden(t *testing.T, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatalf("create golden dir: %v", err)
	}
	if err := os.WriteFile(goldenPath, content, 0o644); err != nil {
		t.Fatalf("write golden file %s: %v", goldenPath, err)
	}
}
