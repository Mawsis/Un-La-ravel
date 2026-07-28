package model

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// updateGolden, set by `-update`, rewrites internal/model/testdata/golden.json
// from the known model instead of asserting against it. Regenerate deliberately
// after an intentional contract change and review the diff — the golden IS the
// contract (ADR 0004).
var updateGolden = flag.Bool("update", false, "regenerate the model golden file instead of comparing")

// buildKnownModel constructs a fixed ProjectModel used by the golden-file test.
// It exercises every interesting JSON shape: a stamped schema version, a table
// with a primary-key column and a nullable foreign-key column carrying a
// References target, and a second table with no columns (to prove empty
// collections serialize as [] rather than null); a Model carrying relationships
// of every supported kind (one with an explicit foreign key, one with explicit
// keys, the rest implicit) plus a relationship-less Model (to prove
// "relationships": [] serializes as [] not null); a Disagreement of each kind
// (to lock the 1.1.0 contract fields models and disagreements); and — to lock
// the new 1.2.0 fields routes, controllers, and dead_routes — a resolved Route
// (FQN and Name populated), a Controller carrying its actions, and a DeadRoute
// of each kind.
func buildKnownModel() *ProjectModel {
	users := NewTable("users")
	users.Columns = append(users.Columns,
		Column{
			Name:         "id",
			Type:         "bigInteger",
			Nullable:     false,
			IsPrimaryKey: true,
			IsForeignKey: false,
		},
		Column{
			Name:         "team_id",
			Type:         "bigInteger",
			Nullable:     true,
			IsPrimaryKey: false,
			IsForeignKey: true,
			References:   &ForeignKeyRef{Table: "teams", Column: "id"},
		},
	)

	post := NewModel("Post")
	post.Table = "posts"
	post.Relationships = append(post.Relationships,
		Relationship{Kind: "belongsTo", Method: "author", Target: "User", ForeignKey: "author_id"},
		Relationship{Kind: "hasMany", Method: "comments", Target: "Comment"},
	)

	user := NewModel("User")
	user.Table = "users"
	user.Relationships = append(user.Relationships,
		Relationship{Kind: "hasMany", Method: "posts", Target: "Post"},
		Relationship{Kind: "belongsToMany", Method: "roles", Target: "Role", ForeignKey: "user_id", LocalKey: "id"},
	)

	pm := New("blog", "11.x")
	pm.AddTable(users)
	pm.AddTable(NewTable("posts"))
	pm.AddModel(post)
	pm.AddModel(user)
	pm.AddDisagreement(Disagreement{
		Model:        "Post",
		Relationship: "author",
		Reason:       `foreign key column "author_id" not found on table "posts"`,
		Kind:         DisagreementMissingFKColumn,
	})
	pm.AddDisagreement(Disagreement{
		Model:        "User",
		Relationship: "roles",
		Reason:       `target model "Role" is not among the extracted models, so its table cannot be resolved`,
		Kind:         DisagreementMissingTable,
	})

	postController := NewController("PostController", `App\Http\Controllers\PostController`)
	postController.Actions = append(postController.Actions, "index", "show")
	pm.AddController(postController)

	pm.AddRoute(Route{
		Method:     "GET",
		URI:        "/posts",
		Controller: "PostController",
		Action:     "index",
		Middleware: []string{"web", "auth"},
		Auth:       AuthAuthenticated,
		Name:       "posts.index",
		FQN:        `App\Http\Controllers\PostController`,
	})

	pm.AddDeadRoute(DeadRoute{
		Method:     "GET",
		URI:        "/ghost",
		Controller: "GhostController",
		Action:     "index",
		Reason:     `controller "App\Http\Controllers\GhostController" not found`,
		Kind:       DeadRouteMissingController,
	})
	pm.AddDeadRoute(DeadRoute{
		Method:     "POST",
		URI:        "/posts",
		Controller: "PostController",
		Action:     "store",
		Reason:     `action "store" not found on controller "App\Http\Controllers\PostController"`,
		Kind:       DeadRouteMissingAction,
	})

	// Two Findings — dead routes then disagreements — to lock the "findings"
	// array shape (kind/severity/count/label/view, severity added in 1.8.0) and
	// its fixed understand-then-judge order. The engine computes these from the
	// assembled model (internal/findings); here they are added explicitly because
	// the model package cannot import findings (it would be a cycle), and the
	// golden only needs to lock the serialized shape, not the computation.
	// Severity is stamped through SeverityFor — the same single table the
	// producer uses — so the golden can never disagree with the mapping.
	pm.AddFinding(Finding{Kind: FindingDeadRoutes, Severity: SeverityFor(FindingDeadRoutes), Count: 2, Label: "2 dead routes", View: "findings"})
	pm.AddFinding(Finding{Kind: FindingDisagreements, Severity: SeverityFor(FindingDisagreements), Count: 2, Label: "2 disagreements", View: "findings"})

	// Three Middlewares — one per origin tier, in the tiered emit order (app →
	// framework → unknown) — to lock the "middlewares" array shape. The first
	// (session) is a Kernel-declared app node (issue #66, contract 1.11.0): it
	// pins the POPULATED fields — a resolved "class", a non-empty "groups", a true
	// "global", and a non-zero "priority" — that earlier contracts always left at
	// their zero values. The second (auth) exercises the framework-origin tier and
	// the non-nil empty "groups": [] with an omitted "class". The third (tenant)
	// exercises the unknown-origin tier: origin "unknown", no guessed class,
	// "global": false, "priority": 0. Together they pin every field of the node
	// and the tiered emit order.
	session := NewMiddleware("session", OriginApp)
	session.Class = `Illuminate\Session\Middleware\StartSession`
	session.Groups = []string{"web"}
	session.Global = true
	session.Priority = 1
	pm.AddMiddleware(session)
	pm.AddMiddleware(NewMiddleware("auth", OriginFramework))
	pm.AddMiddleware(NewMiddleware("tenant", OriginUnknown))
	return pm
}

// TestCurrentSchemaVersionIsBumpedForHidden pins the contract version LITERALLY,
// which the other version assertions in this file deliberately cannot: they
// compare against CurrentSchemaVersion, so they hold at any value and would pass
// unchanged if a contract-shaping slice forgot its bump. Adding "hidden" to the
// Model node (issue #65) is exactly such a slice, so this test names the number
// out loud. It is meant to FAIL on the next shape change — that failure is the
// prompt to add the doc paragraph and regenerate the goldens, per CLAUDE.md's
// "a contract change is not complete until the version bump, the golden
// regeneration, and the affected renderer tests all land in the same PR".
func TestCurrentSchemaVersionIsBumpedForHidden(t *testing.T) {
	const want = "1.13.0"

	if CurrentSchemaVersion != want {
		t.Errorf("CurrentSchemaVersion = %q, want %q — if this change intentionally "+
			"reshapes the contract, bump the const, add its doc paragraph, regenerate "+
			"the goldens, and update this literal", CurrentSchemaVersion, want)
	}
}

// TestNewStampsSchemaVersion verifies the constructor sets the versioned
// contract field (ADR 0004) to CurrentSchemaVersion without the caller doing
// anything, and initializes Schemas, Models, Disagreements, Routes,
// Controllers, and DeadRoutes to non-nil empty slices so they serialize as []
// rather than null.
func TestNewStampsSchemaVersion(t *testing.T) {
	pm := New("blog", "11.x")

	if pm.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("New() SchemaVersion = %q, want %q", pm.SchemaVersion, CurrentSchemaVersion)
	}

	collections := []struct {
		name   string
		isNil  bool
		length int
	}{
		{"Schemas", pm.Schemas == nil, len(pm.Schemas)},
		{"Models", pm.Models == nil, len(pm.Models)},
		{"Disagreements", pm.Disagreements == nil, len(pm.Disagreements)},
		{"Routes", pm.Routes == nil, len(pm.Routes)},
		{"Controllers", pm.Controllers == nil, len(pm.Controllers)},
		{"DeadRoutes", pm.DeadRoutes == nil, len(pm.DeadRoutes)},
		{"Findings", pm.Findings == nil, len(pm.Findings)},
		{"Middlewares", pm.Middlewares == nil, len(pm.Middlewares)},
	}
	for _, c := range collections {
		if c.isNil {
			t.Errorf("New() %s is nil, want non-nil empty slice", c.name)
		}
		if c.length != 0 {
			t.Errorf("New() %s has %d entries, want 0", c.name, c.length)
		}
	}
}

// TestNewModelHasNonNilRelationships verifies NewModel initializes a non-nil
// empty Relationships slice so a relationship-less model serializes
// "relationships": [] not null.
func TestNewModelHasNonNilRelationships(t *testing.T) {
	m := NewModel("Post")

	if m.Name != "Post" {
		t.Errorf("NewModel() Name = %q, want %q", m.Name, "Post")
	}
	if m.Relationships == nil {
		t.Error("NewModel() Relationships is nil, want non-nil empty slice")
	}
	// Hidden joins Fillable and Guarded in the deliberate nil-vs-empty exception
	// (see Model's doc comment): NewModel must NOT coerce it to []string{}, or
	// "declared empty" becomes indistinguishable from "never declared".
	if m.Hidden != nil {
		t.Errorf("NewModel() Hidden = %v, want nil (not declared)", m.Hidden)
	}
}

// TestModelHiddenNilVsEmptySerialization pins the load-bearing distinction on
// Hidden through JSON, the boundary consumers actually read: an undeclared
// $hidden must serialize as null, while an explicit `protected $hidden = [];`
// must serialize as []. Coercing either direction would erase the difference
// between "hides nothing by omission" and "was explicitly declared to hide
// nothing" — the same reason Fillable and Guarded carry this exception.
func TestModelHiddenNilVsEmptySerialization(t *testing.T) {
	t.Run("not_declared_serializes_null", func(t *testing.T) {
		m := NewModel("Post")

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"hidden":null`) {
			t.Errorf("serialized Model = %s, want it to contain \"hidden\":null", data)
		}
	})

	t.Run("declared_empty_serializes_empty_array", func(t *testing.T) {
		m := NewModel("PublicProfile")
		m.Hidden = []string{}

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"hidden":[]`) {
			t.Errorf("serialized Model = %s, want it to contain \"hidden\":[]", data)
		}
	})

	t.Run("declared_values_round_trip", func(t *testing.T) {
		m := NewModel("Account")
		m.Hidden = []string{"password", "remember_token"}

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		var decoded Model
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if !reflect.DeepEqual(decoded.Hidden, m.Hidden) {
			t.Errorf("round-tripped Hidden = %v, want %v", decoded.Hidden, m.Hidden)
		}
	})
}

// TestAddModelPreservesDiscoveryOrder verifies AddModel appends in call order
// and returns the receiver for chaining.
func TestAddModelPreservesDiscoveryOrder(t *testing.T) {
	pm := New("blog", "11.x")

	got := pm.AddModel(NewModel("Post")).AddModel(NewModel("User"))

	if got != pm {
		t.Error("AddModel() did not return the receiver for chaining")
	}
	if len(pm.Models) != 2 {
		t.Fatalf("AddModel() produced %d models, want 2", len(pm.Models))
	}
	if pm.Models[0].Name != "Post" || pm.Models[1].Name != "User" {
		t.Errorf("AddModel() order = [%q, %q], want [Post, User]",
			pm.Models[0].Name, pm.Models[1].Name)
	}
}

// TestAddDisagreementPreservesDetectionOrder verifies AddDisagreement appends in
// call order and returns the receiver for chaining.
func TestAddDisagreementPreservesDetectionOrder(t *testing.T) {
	pm := New("blog", "11.x")

	first := Disagreement{Model: "Post", Relationship: "author", Kind: DisagreementMissingFKColumn}
	second := Disagreement{Model: "User", Relationship: "roles", Kind: DisagreementMissingTable}

	got := pm.AddDisagreement(first).AddDisagreement(second)

	if got != pm {
		t.Error("AddDisagreement() did not return the receiver for chaining")
	}
	if len(pm.Disagreements) != 2 {
		t.Fatalf("AddDisagreement() produced %d disagreements, want 2", len(pm.Disagreements))
	}
	if pm.Disagreements[0] != first || pm.Disagreements[1] != second {
		t.Errorf("AddDisagreement() did not preserve detection order: got %+v", pm.Disagreements)
	}
}

// TestAddFindingPreservesDetectionOrder verifies AddFinding appends in call
// order and returns the receiver for chaining, so the itemized verdict keeps its
// fixed understand-then-judge order in the serialized contract.
func TestAddFindingPreservesDetectionOrder(t *testing.T) {
	pm := New("blog", "11.x")

	first := Finding{Kind: FindingDeadRoutes, Count: 1, Label: "1 dead route", View: "findings"}
	second := Finding{Kind: FindingDisagreements, Count: 3, Label: "3 disagreements", View: "findings"}

	got := pm.AddFinding(first).AddFinding(second)

	if got != pm {
		t.Error("AddFinding() did not return the receiver for chaining")
	}
	if len(pm.Findings) != 2 {
		t.Fatalf("AddFinding() produced %d findings, want 2", len(pm.Findings))
	}
	if pm.Findings[0] != first || pm.Findings[1] != second {
		t.Errorf("AddFinding() did not preserve detection order: got %+v", pm.Findings)
	}
}

// TestFindingSerializesFieldsInOrder verifies a Finding marshals its five
// contract fields (kind, severity, count, label, view) in struct order — the
// shape the dashboard's verdict reader and any JSON consumer depend on. Severity
// was added in contract 1.8.0 (issue #47) between kind and count.
func TestFindingSerializesFieldsInOrder(t *testing.T) {
	pm := New("blog", "11.x").
		AddFinding(Finding{Kind: FindingUnguarded, Severity: SeverityBlocker, Count: 1, Label: "1 unguarded model", View: "findings"})

	out, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	want := `"findings": [
    {
      "kind": "unguarded",
      "severity": "blocker",
      "count": 1,
      "label": "1 unguarded model",
      "view": "findings"
    }
  ]`
	if !bytes.Contains(out, []byte(want)) {
		t.Errorf("ToJSON() findings block does not match expected shape.\n--- got ---\n%s\n--- want substring ---\n%s", out, want)
	}
}

// TestNewMiddlewareHasNonNilGroups verifies NewMiddleware initializes a non-nil
// empty Groups slice so a group-less middleware serializes "groups": [] not
// null, and stamps the given alias and origin.
func TestNewMiddlewareHasNonNilGroups(t *testing.T) {
	m := NewMiddleware("auth", OriginFramework)

	if m.Alias != "auth" {
		t.Errorf("NewMiddleware() Alias = %q, want %q", m.Alias, "auth")
	}
	if m.Origin != OriginFramework {
		t.Errorf("NewMiddleware() Origin = %q, want %q", m.Origin, OriginFramework)
	}
	if m.Groups == nil {
		t.Error("NewMiddleware() Groups is nil, want non-nil empty slice")
	}
}

// TestAddMiddlewarePreservesEmitOrder verifies AddMiddleware appends in call
// order and returns the receiver for chaining, so the tiered emit order (ADR
// 0012) survives into the serialized contract.
func TestAddMiddlewarePreservesEmitOrder(t *testing.T) {
	pm := New("blog", "11.x")

	got := pm.AddMiddleware(NewMiddleware("auth", OriginFramework)).
		AddMiddleware(NewMiddleware("tenant", OriginUnknown))

	if got != pm {
		t.Error("AddMiddleware() did not return the receiver for chaining")
	}
	if len(pm.Middlewares) != 2 {
		t.Fatalf("AddMiddleware() produced %d middlewares, want 2", len(pm.Middlewares))
	}
	if pm.Middlewares[0].Alias != "auth" || pm.Middlewares[1].Alias != "tenant" {
		t.Errorf("AddMiddleware() order = [%q, %q], want [auth, tenant]",
			pm.Middlewares[0].Alias, pm.Middlewares[1].Alias)
	}
}

// TestMiddlewareOmitsClassWhenAbsent verifies an unknown-origin middleware (no
// class to resolve) has no "class" key in the serialized output (omitempty on
// the empty Class), while its "groups" and "origin" keys are always present.
func TestMiddlewareOmitsClassWhenAbsent(t *testing.T) {
	pm := New("blog", "11.x").AddMiddleware(NewMiddleware("tenant", OriginUnknown))

	out, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	want := `"middlewares": [
    {
      "alias": "tenant",
      "groups": [],
      "origin": "unknown",
      "global": false,
      "priority": 0
    }
  ]`
	if !bytes.Contains(out, []byte(want)) {
		t.Errorf("ToJSON() middlewares block does not match expected shape.\n--- got ---\n%s\n--- want substring ---\n%s", out, want)
	}
}

// TestNewTableHasNonNilColumns verifies NewTable initializes a non-nil empty
// Columns slice so a column-less table serializes "columns": [] not null.
func TestNewTableHasNonNilColumns(t *testing.T) {
	tbl := NewTable("posts")

	if tbl.Name != "posts" {
		t.Errorf("NewTable() Name = %q, want %q", tbl.Name, "posts")
	}
	if tbl.Columns == nil {
		t.Error("NewTable() Columns is nil, want non-nil empty slice")
	}
}

// TestAddTablePreservesDiscoveryOrder verifies AddTable appends in call order
// and returns the receiver for chaining.
func TestAddTablePreservesDiscoveryOrder(t *testing.T) {
	pm := New("blog", "11.x")

	got := pm.AddTable(NewTable("first")).AddTable(NewTable("second"))

	if got != pm {
		t.Error("AddTable() did not return the receiver for chaining")
	}
	if len(pm.Schemas) != 2 {
		t.Fatalf("AddTable() produced %d schemas, want 2", len(pm.Schemas))
	}
	if pm.Schemas[0].Name != "first" || pm.Schemas[1].Name != "second" {
		t.Errorf("AddTable() order = [%q, %q], want [first, second]",
			pm.Schemas[0].Name, pm.Schemas[1].Name)
	}
}

// TestToJSONMatchesGolden serializes the known model and asserts it is
// byte-for-byte equal to internal/model/testdata/golden.json (trailing
// whitespace ignored). This locks the public output contract (ADR 0004),
// including snake_case keys, struct field ordering, and the omitempty behavior
// of references.
func TestToJSONMatchesGolden(t *testing.T) {
	pm := buildKnownModel()

	got, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() returned error: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden.json")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, append(got, '\n'), 0o644); err != nil {
			t.Fatalf("regenerating golden: %v", err)
		}
		t.Logf("regenerated %s", goldenPath)
		return
	}

	wantRaw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	want := bytes.TrimRight(wantRaw, "\n")
	got = bytes.TrimRight(got, "\n")

	if !bytes.Equal(got, want) {
		t.Errorf("ToJSON() did not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestToJSONIsDeterministic verifies repeated serialization of the same model
// yields byte-identical output, which golden-file consumers rely on (no maps,
// insertion order preserved).
func TestToJSONIsDeterministic(t *testing.T) {
	pm := buildKnownModel()

	first, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() first call error: %v", err)
	}
	second, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() second call error: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("ToJSON() is not deterministic: two calls produced different bytes")
	}
}

// TestToJSONOmitsReferencesWhenAbsent verifies a non-foreign-key column has no
// "references" key in the serialized output (omitempty on a nil pointer).
func TestToJSONOmitsReferencesWhenAbsent(t *testing.T) {
	tbl := NewTable("users")
	tbl.Columns = append(tbl.Columns, Column{Name: "id", Type: "bigInteger", IsPrimaryKey: true})
	pm := New("blog", "11.x").AddTable(tbl)

	out, err := pm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	if bytes.Contains(out, []byte("references")) {
		t.Errorf("ToJSON() emitted a references key for a non-FK column:\n%s", out)
	}

	// And the JSON must still be valid and round-trippable.
	var rt ProjectModel
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("serialized output is not valid JSON: %v", err)
	}
}

// TestSchemaVersionPresentInJSON verifies the schema_version key is present and
// carries CurrentSchemaVersion in the serialized contract.
func TestSchemaVersionPresentInJSON(t *testing.T) {
	out, err := New("blog", "11.x").ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	var decoded struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decoding serialized output: %v", err)
	}
	if decoded.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("serialized schema_version = %q, want %q", decoded.SchemaVersion, CurrentSchemaVersion)
	}
}
