// Package diff computes the difference between two Project Models — the same
// versioned unlaravel.json contract, analyzed at two points in time — and
// enumerates what a change added, removed, or altered per section (issue #53,
// parent #46).
//
// It is a pure package: one function, two ProjectModels in, one DiffReport out.
// No I/O, no color, no analysis — the CLI reads the two files and prints the
// report; this package only compares. Like internal/model it never marshals a
// Go map into its output: every section is diffed by walking the OLD slice in
// source order (removed + changed), then the NEW slice in source order (added),
// so the report is deterministic and two identical input pairs serialize to
// byte-identical JSON (the golden-file / determinism convention).
package diff

import (
	"encoding/json"
	"fmt"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// jsonIndent is the indentation used for the serialized DiffReport, matching the
// model package's two-space contract so a diff and a model read the same in a
// terminal or a golden file.
const jsonIndent = "  "

// DiffReport is the difference between two Project Models: the two schema
// versions compared, then one section diff per contract section (routes,
// tables, columns, models, findings). Field order is emitted-JSON order; every
// slice within is non-nil and in source order, so identical input pairs
// serialize identically.
type DiffReport struct {
	// SchemaVersionOld and SchemaVersionNew record the schema_version of each
	// input, so a report over two different contract versions names both rather
	// than silently comparing across a shape change.
	SchemaVersionOld string `json:"schema_version_old"`
	SchemaVersionNew string `json:"schema_version_new"`
	// Routes is the added/removed/changed diff of the Routes section, keyed by
	// method+URI.
	Routes RouteDiff `json:"routes"`
	// Tables is the added/removed/changed diff of the Schemas (tables) section,
	// keyed by table name.
	Tables TableDiff `json:"tables"`
	// Columns is the added/removed/changed diff of every column across all
	// tables, keyed by table+name. It is a section in its own right (issue #53)
	// so a column-level change (a type change, a new/dropped column) is itemized
	// even though its table also appears in Tables.Changed.
	Columns ColumnDiff `json:"columns"`
	// Models is the added/removed/changed diff of the Eloquent Models section,
	// keyed by model name.
	Models ModelDiff `json:"models"`
	// Findings is the added/removed/changed diff of the health-verdict Findings
	// section, keyed by kind. A finding whose severity changed (issue #47) — even
	// with an unchanged count and label — is a Change, so a diff can gate on a
	// severity that rose to blocker.
	Findings FindingDiff `json:"findings"`
}

// FindingChange is one finding of the same kind that differs between the two
// models in some field — most importantly its severity (issue #47), but also its
// count or label.
type FindingChange struct {
	Old model.Finding `json:"old"`
	New model.Finding `json:"new"`
}

// FindingDiff is the added/removed/changed diff for the Findings section,
// findings identified by kind (a project has at most one finding per kind — the
// rolled-up category). Every slice is non-nil and in source order.
type FindingDiff struct {
	Added   []model.Finding `json:"added"`
	Removed []model.Finding `json:"removed"`
	Changed []FindingChange `json:"changed"`
}

// empty reports whether this section has no additions, removals, or changes.
func (d FindingDiff) empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// ModelChange is one Eloquent model that exists in both models under the same
// name but differs in some field (table, relationships, fillable/guarded, casts).
type ModelChange struct {
	Old model.Model `json:"old"`
	New model.Model `json:"new"`
}

// ModelDiff is the added/removed/changed diff for the Eloquent Models section,
// models identified by name. Every slice is non-nil and in source order.
type ModelDiff struct {
	Added   []model.Model `json:"added"`
	Removed []model.Model `json:"removed"`
	Changed []ModelChange `json:"changed"`
}

// empty reports whether this section has no additions, removals, or changes.
func (d ModelDiff) empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// ColumnRef is one column together with the table it belongs to, so a column in
// the flattened cross-table diff is unambiguous (two tables may each have an
// "id" column).
type ColumnRef struct {
	Table  string       `json:"table"`
	Column model.Column `json:"column"`
}

// ColumnChange is one column that exists in both models under the same
// table+name but differs in some field (type, nullability, key flags, FK target).
type ColumnChange struct {
	Table string       `json:"table"`
	Old   model.Column `json:"old"`
	New   model.Column `json:"new"`
}

// ColumnDiff is the added/removed/changed diff for columns across all tables,
// each column identified by table+name. Every slice is non-nil and in source
// order (columns walked table-by-table in table order, then column-declaration
// order within each table). Columns of a table that was wholly added or removed
// are NOT repeated here — that table's appearance in Tables.Added/Removed
// already carries them; this section reports only columns of tables present in
// both models.
type ColumnDiff struct {
	Added   []ColumnRef    `json:"added"`
	Removed []ColumnRef    `json:"removed"`
	Changed []ColumnChange `json:"changed"`
}

// empty reports whether this section has no additions, removals, or changes.
func (d ColumnDiff) empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// TableChange is one table that exists in both models under the same name but
// differs in some field (its columns or indexes changed).
type TableChange struct {
	Old model.Table `json:"old"`
	New model.Table `json:"new"`
}

// TableDiff is the added/removed/changed diff for the Schemas (tables) section,
// tables identified by name. Every slice is non-nil and in source order.
type TableDiff struct {
	Added   []model.Table `json:"added"`
	Removed []model.Table `json:"removed"`
	Changed []TableChange `json:"changed"`
}

// empty reports whether this section has no additions, removals, or changes.
func (d TableDiff) empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// RouteChange is one route that exists in both models under the same method+URI
// but differs in some other field (controller, action, auth, middleware, ...).
type RouteChange struct {
	// Old and New are the two versions of the route, so a consumer can render
	// exactly what changed without re-deriving it.
	Old model.Route `json:"old"`
	New model.Route `json:"new"`
}

// RouteDiff is the added/removed/changed diff for the Routes section. Routes are
// identified by method+URI: same key in both → candidate for Changed; only in
// new → Added; only in old → Removed. Every slice is non-nil and in source
// order (Added in new order, Removed and Changed in old order).
type RouteDiff struct {
	Added   []model.Route `json:"added"`
	Removed []model.Route `json:"removed"`
	Changed []RouteChange `json:"changed"`
}

// empty reports whether this section has no additions, removals, or changes.
func (d RouteDiff) empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// Empty reports whether the two models are identical across every diffed
// section — no additions, removals, or changes anywhere. A clean diff.
func (r DiffReport) Empty() bool {
	return r.Routes.empty() && r.Tables.empty() && r.Columns.empty() &&
		r.Models.empty() && r.Findings.empty()
}

// ToJSON serializes the DiffReport to stable, 2-space-indented JSON. The output
// is deterministic: every section slice is built by walking a source-ordered
// input (never by iterating a map into the result), and no map is marshaled, so
// identical input pairs always produce byte-identical JSON — what the
// determinism test and any golden-file comparison rely on.
func (r DiffReport) ToJSON() ([]byte, error) {
	out, err := json.MarshalIndent(r, "", jsonIndent)
	if err != nil {
		return nil, fmt.Errorf("diff: serialize report to JSON: %w", err)
	}
	return out, nil
}

// IntroducesBlocker reports whether this diff introduces a NEW blocker finding:
// a blocker that the old model did not have. That is the CI gate the `unlaravel
// diff` subcommand exits non-zero on — so a PR that adds a dead route or an
// unauthenticated write route (either newly blocker-severity findings) fails its
// check, while a blocker that already existed and merely persists does not
// re-fail the gate.
//
// "Newly present" is exactly the two shapes the finding diff produces: the
// finding appears in Findings.Added at blocker severity, or an existing finding
// appears in Findings.Changed whose NEW severity is blocker and whose OLD
// severity was not (its severity rose to blocker).
func (r DiffReport) IntroducesBlocker() bool {
	for _, f := range r.Findings.Added {
		if f.Severity == model.SeverityBlocker {
			return true
		}
	}
	for _, c := range r.Findings.Changed {
		if c.New.Severity == model.SeverityBlocker && c.Old.Severity != model.SeverityBlocker {
			return true
		}
	}
	return false
}

// Diff compares an old and a new Project Model and returns the DiffReport
// enumerating what changed per section, in source order. It is pure: it reads
// only the two models and allocates a fresh report.
func Diff(old, new *model.ProjectModel) DiffReport {
	return DiffReport{
		SchemaVersionOld: old.SchemaVersion,
		SchemaVersionNew: new.SchemaVersion,
		Routes:           diffRoutes(old.Routes, new.Routes),
		Tables:           diffTables(old.Schemas, new.Schemas),
		Columns:          diffColumns(old.Schemas, new.Schemas),
		Models:           diffModels(old.Models, new.Models),
		Findings:         diffFindings(old.Findings, new.Findings),
	}
}

// findingKind is a finding's identity for diffing: its kind (a project carries
// at most one rolled-up finding per kind).
func findingKind(f model.Finding) string { return f.Kind }

// diffFindings computes the added/removed/changed diff for the Findings section
// over the shared diffSection algorithm, keyed by kind. A finding present under
// the same kind in both models but differing in any field — severity (issue
// #47), count, or label — is a Change.
func diffFindings(old, new []model.Finding) FindingDiff {
	added, removed, changed := diffSection(old, new, findingKind)
	out := make([]FindingChange, 0, len(changed))
	for _, p := range changed {
		out = append(out, FindingChange{Old: p.Old, New: p.New})
	}
	return FindingDiff{Added: added, Removed: removed, Changed: out}
}

// modelName is an Eloquent model's identity for diffing: its class name.
func modelName(m model.Model) string { return m.Name }

// diffModels computes the added/removed/changed diff for the Eloquent Models
// section over the shared diffSection algorithm, keyed by model name.
func diffModels(old, new []model.Model) ModelDiff {
	added, removed, changed := diffSection(old, new, modelName)
	out := make([]ModelChange, 0, len(changed))
	for _, p := range changed {
		out = append(out, ModelChange{Old: p.Old, New: p.New})
	}
	return ModelDiff{Added: added, Removed: removed, Changed: out}
}

// columnKeySep separates a column's table from its name in its cross-table
// identity key. It is the ASCII Unit Separator — a byte that cannot appear in a
// table or column name — so no combination of names can collide by straddling
// it (matching the separator rationale in internal/baseline).
const columnKeySep = "\x1f"

// columnRefKey identifies a column across the two models: its table plus its
// name. Two ColumnRefs with the same table+name are "the same column" for
// diffing, so a type change is a Change, not a remove+add.
func columnRefKey(c ColumnRef) string {
	return c.Table + columnKeySep + c.Column.Name
}

// diffColumns computes the added/removed/changed diff for columns across all
// tables, each keyed by table+name, over the shared diffSection algorithm. It
// flattens the columns of tables present in BOTH models into per-side ColumnRef
// slices (in table order, then column-declaration order within each table) and
// diffs those. Columns of a wholly added or removed table are deliberately
// excluded: that table already appears in Tables.Added/Removed with its columns,
// so counting them here too would double-report them.
func diffColumns(old, new []model.Table) ColumnDiff {
	sharedOld, sharedNew := sharedTableColumns(old, new)

	added, removed, changed := diffSection(sharedOld, sharedNew, columnRefKey)
	out := make([]ColumnChange, 0, len(changed))
	for _, p := range changed {
		out = append(out, ColumnChange{Table: p.New.Table, Old: p.Old.Column, New: p.New.Column})
	}
	return ColumnDiff{Added: added, Removed: removed, Changed: out}
}

// sharedTableColumns flattens, for each side, the columns of tables present in
// BOTH models into ColumnRef slices in table-then-column source order. Building
// the old side by walking old tables (and the new side by walking new tables),
// each gated on membership in the other model, keeps both slices source-ordered
// and excludes columns of tables unique to one side.
func sharedTableColumns(old, new []model.Table) (oldRefs, newRefs []ColumnRef) {
	oldNames := tableNameSet(old)
	newNames := tableNameSet(new)

	for _, ot := range old {
		if !newNames[ot.Name] {
			continue
		}
		for _, c := range ot.Columns {
			oldRefs = append(oldRefs, ColumnRef{Table: ot.Name, Column: c})
		}
	}
	for _, nt := range new {
		if !oldNames[nt.Name] {
			continue
		}
		for _, c := range nt.Columns {
			newRefs = append(newRefs, ColumnRef{Table: nt.Name, Column: c})
		}
	}
	return oldRefs, newRefs
}

// tableNameSet is the set of table names in a slice, a membership lookup used to
// gate which tables contribute columns. Lookup only — never iterated into a
// result — so determinism holds.
func tableNameSet(tables []model.Table) map[string]bool {
	m := make(map[string]bool, len(tables))
	for _, t := range tables {
		m[t.Name] = true
	}
	return m
}

// tableName is a table's identity for diffing: its name.
func tableName(t model.Table) string { return t.Name }

// diffTables computes the added/removed/changed diff for the Schemas section
// over the shared diffSection algorithm, keyed by table name.
func diffTables(old, new []model.Table) TableDiff {
	added, removed, changed := diffSection(old, new, tableName)
	out := make([]TableChange, 0, len(changed))
	for _, p := range changed {
		out = append(out, TableChange{Old: p.Old, New: p.New})
	}
	return TableDiff{Added: added, Removed: removed, Changed: out}
}

// routeKey identifies a route across the two models: its HTTP method and URI.
// Two routes with the same method+URI are "the same route" for diffing, so a
// change to the controller or auth of an existing route is a Change, not a
// remove+add.
func routeKey(r model.Route) string {
	return r.Method + " " + r.URI
}

// diffRoutes computes the added/removed/changed diff for the Routes section over
// the shared diffSection algorithm, keyed by method+URI.
func diffRoutes(old, new []model.Route) RouteDiff {
	added, removed, changed := diffSection(old, new, routeKey)
	return RouteDiff{Added: added, Removed: removed, Changed: routeChanges(changed)}
}

// routeChanges adapts the generic changed-pairs to the section's Change type.
func routeChanges(pairs []pair[model.Route]) []RouteChange {
	out := make([]RouteChange, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, RouteChange{Old: p.Old, New: p.New})
	}
	return out
}
