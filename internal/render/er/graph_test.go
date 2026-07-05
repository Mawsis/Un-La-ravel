package er

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// graphGoldenPath is the on-disk location of the expected ERGraph JSON.
const graphGoldenPath = "testdata/golden.graph.json"

// marshalGraph renders g as deterministic indented JSON. The transform marshals
// no Go map (struct field order is fixed, slices preserve model order), so a
// fixed model always yields byte-identical JSON — the property the golden pins.
func marshalGraph(t *testing.T, g ERGraph) string {
	t.Helper()
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatalf("marshal ERGraph: %v", err)
	}
	return string(data) + "\n"
}

// TestRenderGraph_NodeCarriesOrderedColumnsWithKeyMarkers is the tracer bullet
// for the graph renderer (issue #21): a table becomes one node whose columns
// preserve source order and carry the right key marker. PK > FK > UK precedence
// and the single-column-unique → UK rule are the same domain rules the Mermaid
// renderer applies; here they surface as the ERColumn.Key field instead of a
// string token.
func TestRenderGraph_NodeCarriesOrderedColumnsWithKeyMarkers(t *testing.T) {
	users := model.NewTable("users")
	users.Columns = []model.Column{
		{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
		{Name: "email", Type: "string"},
	}
	users.Indexes = []model.Index{
		{Name: "users_email_unique", Columns: []string{"email"}, Unique: true},
	}
	pm := model.New("blog", "11.x").AddTable(users)

	graph := RenderGraph(pm)

	if len(graph.Nodes) != 1 {
		t.Fatalf("Nodes = %d, want 1", len(graph.Nodes))
	}
	node := graph.Nodes[0]
	if node.Table != "users" {
		t.Errorf("Node.Table = %q, want %q", node.Table, "users")
	}

	want := []ERColumn{
		{Name: "id", Type: "bigInteger", Key: "PK"},
		{Name: "email", Type: "string", Key: "UK"},
	}
	if len(node.Columns) != len(want) {
		t.Fatalf("Columns = %d, want %d", len(node.Columns), len(want))
	}
	for i, w := range want {
		if node.Columns[i] != w {
			t.Errorf("Columns[%d] = %+v, want %+v", i, node.Columns[i], w)
		}
	}
}

// TestRenderGraph_SchemaForeignKeyEdges asserts the schema (FK) view: a foreign
// key to a table present in the model produces one parent→child edge, and a
// foreign key to an absent table is silently dropped (no dangling edge). The
// edge direction mirrors the Mermaid renderer's PARENT ||--o{ CHILD: the
// referenced parent is From, the referencing child is To, cardinality one-to-many.
func TestRenderGraph_SchemaForeignKeyEdges(t *testing.T) {
	users := model.NewTable("users")
	users.Columns = []model.Column{{Name: "id", Type: "bigInteger", IsPrimaryKey: true}}

	posts := model.NewTable("posts")
	posts.Columns = []model.Column{
		{Name: "id", Type: "bigInteger", IsPrimaryKey: true},
		{Name: "author_id", Type: "bigInteger", IsForeignKey: true,
			References: &model.ForeignKeyRef{Table: "users", Column: "id"}},
		// References an absent table — the edge must be dropped.
		{Name: "legacy_id", Type: "bigInteger", IsForeignKey: true,
			References: &model.ForeignKeyRef{Table: "legacy_authors", Column: "id"}},
	}

	pm := model.New("blog", "11.x").AddTable(users).AddTable(posts)

	graph := RenderGraph(pm)

	want := []EREdge{
		{From: "users", To: "posts", Kind: "one-to-many",
			Label: "references (author_id)", Origin: "schema"},
	}
	if len(graph.Edges) != len(want) {
		t.Fatalf("Edges = %d (%+v), want %d", len(graph.Edges), graph.Edges, len(want))
	}
	if graph.Edges[0] != want[0] {
		t.Errorf("Edges[0] = %+v, want %+v", graph.Edges[0], want[0])
	}
}

// TestRenderGraph_EloquentCardinalityEdges asserts all four Eloquent-cardinality
// edges: hasMany → one-to-many, hasOne → one-to-one, belongsTo → many-to-one,
// belongsToMany → many-to-many. Each edge resolves the target Model to its table
// (From = declaring table, To = target table), carries a "method (kind)" label,
// and an association whose target Model was not extracted is silently dropped —
// mirroring how a foreign key to an absent table is dropped. Every table here
// exists in the schema, so the edges carry Origin "eloquent" and stay resolved
// (the never-orphan invariant of issue #36 requires endpoints to be schema
// nodes; reconciliation has its own test).
func TestRenderGraph_EloquentCardinalityEdges(t *testing.T) {
	user := model.NewModel("User")
	user.Table = "users"
	user.Relationships = []model.Relationship{
		{Kind: "hasMany", Method: "posts", Target: "Post"},
		{Kind: "hasOne", Method: "profile", Target: "Profile"},
		{Kind: "belongsToMany", Method: "roles", Target: "Role"},
		// Target Model not extracted below — this edge must be dropped.
		{Kind: "hasOne", Method: "pinned", Target: "PinnedComment"},
	}
	post := model.NewModel("Post")
	post.Table = "posts"
	post.Relationships = []model.Relationship{
		{Kind: "belongsTo", Method: "author", Target: "User"},
	}
	profile := model.NewModel("Profile")
	profile.Table = "profiles"
	role := model.NewModel("Role")
	role.Table = "roles"

	pm := model.New("blog", "11.x").
		AddTable(model.NewTable("users")).AddTable(model.NewTable("posts")).
		AddTable(model.NewTable("profiles")).AddTable(model.NewTable("roles")).
		AddModel(user).AddModel(post).AddModel(profile).AddModel(role)

	graph := RenderGraph(pm)

	want := []EREdge{
		{From: "users", To: "posts", Kind: "one-to-many", Label: "posts (hasMany)", Origin: "eloquent"},
		{From: "users", To: "profiles", Kind: "one-to-one", Label: "profile (hasOne)", Origin: "eloquent"},
		{From: "users", To: "roles", Kind: "many-to-many", Label: "roles (belongsToMany)", Origin: "eloquent"},
		{From: "posts", To: "users", Kind: "many-to-one", Label: "author (belongsTo)", Origin: "eloquent"},
	}
	if len(graph.Edges) != len(want) {
		t.Fatalf("Edges = %d (%+v), want %d", len(graph.Edges), graph.Edges, len(want))
	}
	for i, w := range want {
		if graph.Edges[i] != w {
			t.Errorf("Edges[%d] = %+v, want %+v", i, graph.Edges[i], w)
		}
	}
}

// TestRenderGraph_EloquentEdgeReconciliation asserts the never-orphan
// guarantee (issue #36): an Eloquent edge endpoint whose table is absent from
// the schema is either retargeted to its unambiguous singular/plural sibling
// node (marked unresolved so the renderer can style the repair) or dropped —
// the builder never hands the layout engine an edge to a non-existent node.
// The declaring model's own table is reconciled the same way as the target's:
// WaiterCalls' over-pluralized "waiter_callses" draws from the real
// "waiter_calls" node.
func TestRenderGraph_EloquentEdgeReconciliation(t *testing.T) {
	waiterCalls := model.NewModel("WaiterCalls")
	waiterCalls.Table = "waiter_callses" // over-pluralized inference, table absent
	waiterCalls.Relationships = []model.Relationship{
		{Kind: "belongsTo", Method: "user", Target: "User"},
		// Phantom's table has no schema sibling — this edge must be dropped,
		// not guessed.
		{Kind: "belongsTo", Method: "phantom", Target: "Phantom"},
	}
	user := model.NewModel("User")
	user.Table = "users"
	user.Relationships = []model.Relationship{
		// Both endpoints exist as-is: a clean, resolved edge.
		{Kind: "hasMany", Method: "posts", Target: "Post"},
	}
	post := model.NewModel("Post")
	post.Table = "posts"
	phantom := model.NewModel("Phantom")
	phantom.Table = "phantoms"

	users := model.NewTable("users")
	posts := model.NewTable("posts")
	calls := model.NewTable("waiter_calls")
	pm := model.New("cafe", "11.x").
		AddTable(users).AddTable(posts).AddTable(calls).
		AddModel(waiterCalls).AddModel(user).AddModel(post).AddModel(phantom)

	graph := RenderGraph(pm)

	want := []EREdge{
		{From: "waiter_calls", To: "users", Kind: "many-to-one",
			Label: "user (belongsTo)", Origin: "eloquent", Unresolved: true},
		{From: "users", To: "posts", Kind: "one-to-many",
			Label: "posts (hasMany)", Origin: "eloquent"},
	}
	if len(graph.Edges) != len(want) {
		t.Fatalf("Edges = %d (%+v), want %d", len(graph.Edges), graph.Edges, len(want))
	}
	for i, w := range want {
		if graph.Edges[i] != w {
			t.Errorf("Edges[%d] = %+v, want %+v", i, graph.Edges[i], w)
		}
	}
}

// TestRenderGraph_NilAndEmptyModel asserts the safety criterion: a nil model and
// an empty model both render an empty graph without panicking, and the Nodes and
// Edges slices are non-nil (so they serialize as [] not null, per the project's
// non-nil-empty-slice contract convention).
func TestRenderGraph_NilAndEmptyModel(t *testing.T) {
	cases := map[string]*model.ProjectModel{
		"nil model":   nil,
		"empty model": model.New("blog", "11.x"),
	}
	for name, pm := range cases {
		t.Run(name, func(t *testing.T) {
			graph := RenderGraph(pm)

			if len(graph.Nodes) != 0 {
				t.Errorf("Nodes = %d, want 0", len(graph.Nodes))
			}
			if len(graph.Edges) != 0 {
				t.Errorf("Edges = %d, want 0", len(graph.Edges))
			}
			if graph.Nodes == nil {
				t.Error("Nodes is nil, want non-nil empty slice (serializes as [] not null)")
			}
			if graph.Edges == nil {
				t.Error("Edges is nil, want non-nil empty slice (serializes as [] not null)")
			}
		})
	}
}

// TestRenderGraph_Golden pins the full ERGraph for the fixture model to exactly
// the bytes stored in the golden file, proving byte-stability for a fixed model
// (the determinism acceptance criterion) and guarding the whole nodes+edges
// shape at once. Regenerate deliberately with -update and review the diff.
func TestRenderGraph_Golden(t *testing.T) {
	got := marshalGraph(t, RenderGraph(buildFixtureModel()))

	if *update {
		if err := os.WriteFile(graphGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write graph golden %s: %v", graphGoldenPath, err)
		}
		t.Logf("updated graph golden file: %s", graphGoldenPath)
	}

	data, err := os.ReadFile(graphGoldenPath)
	if err != nil {
		t.Fatalf("read graph golden %s: %v (run with -update to generate it)", graphGoldenPath, err)
	}
	if want := string(data); got != want {
		t.Errorf("ERGraph JSON does not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
