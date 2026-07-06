package diff

import (
	"bytes"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// TestDiff_EmptyEmpty is the tracer bullet: diffing a model against an
// identical (here, empty) model yields a report with no added, removed, or
// changed entries in any section, and it carries both schema versions.
func TestDiff_EmptyEmpty(t *testing.T) {
	old := model.New("app", "11.0")
	new := model.New("app", "11.0")

	report := Diff(old, new)

	if report.SchemaVersionOld != old.SchemaVersion {
		t.Errorf("SchemaVersionOld = %q, want %q", report.SchemaVersionOld, old.SchemaVersion)
	}
	if report.SchemaVersionNew != new.SchemaVersion {
		t.Errorf("SchemaVersionNew = %q, want %q", report.SchemaVersionNew, new.SchemaVersion)
	}
	if !report.Empty() {
		t.Errorf("Empty() = false, want true for two identical empty models")
	}
}

// TestDiff_AddedRoute: a route present in new but not old appears in
// Routes.Added, keyed by method+URI, and nowhere in Removed or Changed.
func TestDiff_AddedRoute(t *testing.T) {
	old := model.New("app", "11.0")
	new := model.New("app", "11.0")
	new.AddRoute(model.Route{Method: "GET", URI: "/posts", Controller: "PostController", Action: "index"})

	report := Diff(old, new)

	if report.Empty() {
		t.Fatalf("Empty() = true, want false when a route was added")
	}
	if got := len(report.Routes.Added); got != 1 {
		t.Fatalf("Routes.Added len = %d, want 1", got)
	}
	added := report.Routes.Added[0]
	if added.Method != "GET" || added.URI != "/posts" {
		t.Errorf("added route = %s %s, want GET /posts", added.Method, added.URI)
	}
	if len(report.Routes.Removed) != 0 || len(report.Routes.Changed) != 0 {
		t.Errorf("Removed/Changed = %d/%d, want 0/0", len(report.Routes.Removed), len(report.Routes.Changed))
	}
}

// TestDiff_RemovedTable: a table present in old but not new appears in
// Tables.Removed, keyed by name.
func TestDiff_RemovedTable(t *testing.T) {
	old := model.New("app", "11.0")
	old.AddTable(model.NewTable("posts"))
	new := model.New("app", "11.0")

	report := Diff(old, new)

	if report.Empty() {
		t.Fatalf("Empty() = true, want false when a table was removed")
	}
	if got := len(report.Tables.Removed); got != 1 {
		t.Fatalf("Tables.Removed len = %d, want 1", got)
	}
	if report.Tables.Removed[0].Name != "posts" {
		t.Errorf("removed table = %q, want posts", report.Tables.Removed[0].Name)
	}
	if len(report.Tables.Added) != 0 || len(report.Tables.Changed) != 0 {
		t.Errorf("Added/Changed = %d/%d, want 0/0", len(report.Tables.Added), len(report.Tables.Changed))
	}
}

// TestDiff_ChangedColumn: a column whose type changed while its table+name stay
// the same appears in Columns.Changed, carrying both versions and its table, and
// nowhere in Added/Removed. The column is identified by table+name.
func TestDiff_ChangedColumn(t *testing.T) {
	oldTable := model.NewTable("posts")
	oldTable.Columns = append(oldTable.Columns, model.Column{Name: "votes", Type: "integer"})
	old := model.New("app", "11.0")
	old.AddTable(oldTable)

	newTable := model.NewTable("posts")
	newTable.Columns = append(newTable.Columns, model.Column{Name: "votes", Type: "bigInteger"})
	new := model.New("app", "11.0")
	new.AddTable(newTable)

	report := Diff(old, new)

	if got := len(report.Columns.Changed); got != 1 {
		t.Fatalf("Columns.Changed len = %d, want 1", got)
	}
	c := report.Columns.Changed[0]
	if c.Table != "posts" || c.Old.Type != "integer" || c.New.Type != "bigInteger" {
		t.Errorf("changed column = table %q %s→%s, want posts integer→bigInteger", c.Table, c.Old.Type, c.New.Type)
	}
	if len(report.Columns.Added) != 0 || len(report.Columns.Removed) != 0 {
		t.Errorf("Added/Removed = %d/%d, want 0/0", len(report.Columns.Added), len(report.Columns.Removed))
	}
}

// TestDiff_ChangedModel: a model whose relationships changed while its name
// stays the same appears in Models.Changed, keyed by name.
func TestDiff_ChangedModel(t *testing.T) {
	oldModel := model.NewModel("Post")
	old := model.New("app", "11.0")
	old.AddModel(oldModel)

	newModel := model.NewModel("Post")
	newModel.Relationships = append(newModel.Relationships, model.Relationship{Kind: "belongsTo", Method: "author", Target: "User"})
	new := model.New("app", "11.0")
	new.AddModel(newModel)

	report := Diff(old, new)

	if got := len(report.Models.Changed); got != 1 {
		t.Fatalf("Models.Changed len = %d, want 1", got)
	}
	c := report.Models.Changed[0]
	if c.New.Name != "Post" || len(c.Old.Relationships) != 0 || len(c.New.Relationships) != 1 {
		t.Errorf("changed model = %q %d→%d rels, want Post 0→1", c.New.Name, len(c.Old.Relationships), len(c.New.Relationships))
	}
	if len(report.Models.Added) != 0 || len(report.Models.Removed) != 0 {
		t.Errorf("Added/Removed = %d/%d, want 0/0", len(report.Models.Added), len(report.Models.Removed))
	}
}

// TestDiff_ChangedFindingSeverity: a finding of the same kind whose severity
// rose (warn → blocker) appears in Findings.Changed — a severity change is a
// change (issue #47), even when the count and label are unchanged.
func TestDiff_ChangedFindingSeverity(t *testing.T) {
	old := model.New("app", "11.0")
	old.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 1, Label: "1 dead route"})

	new := model.New("app", "11.0")
	new.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityBlocker, Count: 1, Label: "1 dead route"})

	report := Diff(old, new)

	if got := len(report.Findings.Changed); got != 1 {
		t.Fatalf("Findings.Changed len = %d, want 1", got)
	}
	c := report.Findings.Changed[0]
	if c.Old.Severity != model.SeverityWarn || c.New.Severity != model.SeverityBlocker {
		t.Errorf("severity change = %s→%s, want warn→blocker", c.Old.Severity, c.New.Severity)
	}
	if len(report.Findings.Added) != 0 || len(report.Findings.Removed) != 0 {
		t.Errorf("Added/Removed = %d/%d, want 0/0", len(report.Findings.Added), len(report.Findings.Removed))
	}
}

// TestIntroducesBlocker covers the CI gate: the diff introduces a new blocker
// when a blocker finding is newly ADDED, or when an existing finding's severity
// ROSE to blocker. A pre-existing, still-present blocker is NOT newly introduced,
// and a warning-severity add does not trip the gate.
func TestIntroducesBlocker(t *testing.T) {
	blocker := func() *model.ProjectModel {
		m := model.New("app", "11.0")
		m.AddFinding(model.Finding{Kind: model.FindingUnguarded, Severity: model.SeverityBlocker, Count: 1, Label: "1 unguarded model"})
		return m
	}
	warnDead := func() *model.ProjectModel {
		m := model.New("app", "11.0")
		m.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 1, Label: "1 dead route"})
		return m
	}
	blockerDead := func() *model.ProjectModel {
		m := model.New("app", "11.0")
		m.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityBlocker, Count: 1, Label: "1 dead route"})
		return m
	}

	tests := []struct {
		name     string
		old, new *model.ProjectModel
		want     bool
	}{
		{"clean to blocker added", model.New("app", "11.0"), blocker(), true},
		{"warn added, no blocker", model.New("app", "11.0"), warnDead(), false},
		{"severity rose to blocker", warnDead(), blockerDead(), true},
		{"pre-existing blocker unchanged", blocker(), blocker(), false},
		{"blocker removed", blocker(), model.New("app", "11.0"), false},
		{"clean to clean", model.New("app", "11.0"), model.New("app", "11.0"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Diff(tt.old, tt.new).IntroducesBlocker(); got != tt.want {
				t.Errorf("IntroducesBlocker() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mixedPair builds an old/new model pair exercising every section at once: a
// route added, a table removed, a column changed, a model added, and a finding
// whose severity rose. It is the fixture for both the mixed-coverage and
// determinism tests, so the two stay in sync.
func mixedPair() (*model.ProjectModel, *model.ProjectModel) {
	old := model.New("app", "11.0")
	postsOld := model.NewTable("posts")
	postsOld.Columns = append(postsOld.Columns, model.Column{Name: "votes", Type: "integer"})
	old.AddTable(postsOld)
	old.AddTable(model.NewTable("legacy"))
	old.AddModel(model.NewModel("Post"))
	old.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 1, Label: "1 dead route"})

	new := model.New("app", "11.0")
	postsNew := model.NewTable("posts")
	postsNew.Columns = append(postsNew.Columns, model.Column{Name: "votes", Type: "bigInteger"})
	new.AddTable(postsNew)
	new.AddModel(model.NewModel("Post"))
	new.AddModel(model.NewModel("Comment"))
	new.AddRoute(model.Route{Method: "POST", URI: "/posts", Controller: "PostController", Action: "store"})
	new.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityBlocker, Count: 1, Label: "1 dead route"})

	return old, new
}

// TestDiff_Mixed exercises all five sections in one diff: an added route, a
// removed table, a changed column, an added model, and a changed finding.
func TestDiff_Mixed(t *testing.T) {
	report := Diff(mixedPair())

	if report.Empty() {
		t.Fatal("Empty() = true, want false for a mixed diff")
	}
	if len(report.Routes.Added) != 1 {
		t.Errorf("Routes.Added = %d, want 1", len(report.Routes.Added))
	}
	if len(report.Tables.Removed) != 1 {
		t.Errorf("Tables.Removed = %d, want 1", len(report.Tables.Removed))
	}
	if len(report.Columns.Changed) != 1 {
		t.Errorf("Columns.Changed = %d, want 1", len(report.Columns.Changed))
	}
	if len(report.Models.Added) != 1 {
		t.Errorf("Models.Added = %d, want 1", len(report.Models.Added))
	}
	if len(report.Findings.Changed) != 1 {
		t.Errorf("Findings.Changed = %d, want 1", len(report.Findings.Changed))
	}
}

// TestDiff_Deterministic: the same input pair produces byte-identical serialized
// output across repeated runs — no map iteration leaks into the report, so
// golden-file comparison is stable (the determinism convention).
func TestDiff_Deterministic(t *testing.T) {
	old, new := mixedPair()

	first, err := Diff(old, new).ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}
	for i := 0; i < 50; i++ {
		next, err := Diff(old, new).ToJSON()
		if err != nil {
			t.Fatalf("ToJSON() error on run %d: %v", i, err)
		}
		if !bytes.Equal(first, next) {
			t.Fatalf("run %d serialized differently:\nfirst=%s\nnext =%s", i, first, next)
		}
	}
}

// TestVersionCompatible: two models within the same major schema version are
// compatible (fields align, the diff is meaningful); a major-version mismatch is
// not, so the CLI can refuse with a clear error rather than compare across a
// shape change. An empty version is never compatible.
func TestVersionCompatible(t *testing.T) {
	tests := []struct {
		name     string
		old, new string
		want     bool
	}{
		{"identical", "1.9.0", "1.9.0", true},
		{"same major, differing minor", "1.6.0", "1.9.0", true},
		{"same major, differing patch", "1.9.0", "1.9.1", true},
		{"differing major", "1.9.0", "2.0.0", false},
		{"old empty", "", "1.9.0", false},
		{"new empty", "1.9.0", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := model.New("app", "11.0")
			old.SchemaVersion = tt.old
			new := model.New("app", "11.0")
			new.SchemaVersion = tt.new
			if got := VersionCompatible(old, new); got != tt.want {
				t.Errorf("VersionCompatible(%q, %q) = %v, want %v", tt.old, tt.new, got, tt.want)
			}
		})
	}
}

// TestDiff_DuplicateKeyRemoval guards the multiset edge case: when the same
// identity key (here a route method+URI) appears more than once in the OLD
// model — a duplicate Laravel route registration the extractor emits verbatim —
// a genuine removal must not vanish. Old has two GET /x routes (A and B); new
// has only B. The report must show A removed and B unchanged, never fold A into
// a bogus "A→B changed" pair.
func TestDiff_DuplicateKeyRemoval(t *testing.T) {
	old := model.New("app", "11.0")
	old.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "A", Action: "one"})
	old.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "B", Action: "two"})
	new := model.New("app", "11.0")
	new.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "B", Action: "two"})

	report := Diff(old, new)

	if got := len(report.Routes.Removed); got != 1 {
		t.Fatalf("Routes.Removed len = %d, want 1 (route A was removed)", got)
	}
	if report.Routes.Removed[0].Controller != "A" {
		t.Errorf("removed route controller = %q, want A", report.Routes.Removed[0].Controller)
	}
	if got := len(report.Routes.Changed); got != 0 {
		t.Errorf("Routes.Changed len = %d, want 0 (B is unchanged, A is a removal not a change)", got)
	}
	if got := len(report.Routes.Added); got != 0 {
		t.Errorf("Routes.Added len = %d, want 0", got)
	}
}

// TestDiff_DuplicateKeyAddition is the symmetric case: two identical-key routes
// added in NEW that do not exist in OLD must both be reported as additions.
func TestDiff_DuplicateKeyAddition(t *testing.T) {
	old := model.New("app", "11.0")
	new := model.New("app", "11.0")
	new.AddRoute(model.Route{Method: "GET", URI: "/y", Controller: "A", Action: "one"})
	new.AddRoute(model.Route{Method: "GET", URI: "/y", Controller: "B", Action: "two"})

	report := Diff(old, new)

	if got := len(report.Routes.Added); got != 2 {
		t.Fatalf("Routes.Added len = %d, want 2 (both duplicate-key routes are new)", got)
	}
	if len(report.Routes.Removed) != 0 || len(report.Routes.Changed) != 0 {
		t.Errorf("Removed/Changed = %d/%d, want 0/0", len(report.Routes.Removed), len(report.Routes.Changed))
	}
}

// TestDiff_DuplicateKeyDeterministic: the duplicate-key pairing must be
// byte-stable across runs despite the internal per-key maps, so a diff over
// duplicate-registration routes still supports golden comparison.
func TestDiff_DuplicateKeyDeterministic(t *testing.T) {
	build := func() (*model.ProjectModel, *model.ProjectModel) {
		old := model.New("app", "11.0")
		old.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "A", Action: "one"})
		old.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "B", Action: "two"})
		old.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "C", Action: "three"})
		new := model.New("app", "11.0")
		new.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "C", Action: "three"})
		new.AddRoute(model.Route{Method: "GET", URI: "/x", Controller: "A", Action: "one"})
		return old, new
	}
	first, err := Diff(build()).ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	for i := 0; i < 50; i++ {
		next, err := Diff(build()).ToJSON()
		if err != nil {
			t.Fatalf("ToJSON run %d: %v", i, err)
		}
		if !bytes.Equal(first, next) {
			t.Fatalf("run %d differs:\n%s\n---\n%s", i, first, next)
		}
	}
}

// TestIntroducesBlocker_DuplicateFindingKind guards the gate against the
// duplicate-key masking the reviewer flagged: even if the OLD model carries two
// findings of the same kind (an upstream invariant violation), a NEW blocker
// whose severity rose must still trip IntroducesBlocker rather than being
// mispaired away. Old has two dead_routes findings, one warn one info; new has
// one dead_routes at blocker. The rise to blocker must be detected.
func TestIntroducesBlocker_DuplicateFindingKind(t *testing.T) {
	old := model.New("app", "11.0")
	old.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityInfo, Count: 1, Label: "x"})
	old.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityWarn, Count: 1, Label: "y"})
	new := model.New("app", "11.0")
	new.AddFinding(model.Finding{Kind: model.FindingDeadRoutes, Severity: model.SeverityBlocker, Count: 1, Label: "z"})

	if !Diff(old, new).IntroducesBlocker() {
		t.Error("IntroducesBlocker() = false with a duplicate-kind finding rising to blocker, want true")
	}
}
