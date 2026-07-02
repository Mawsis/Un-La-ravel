// Package er is the ER-diagram Renderer (ADR 0001, ADR 0004): it turns a
// Project Model into a Mermaid erDiagram string.
//
// It is a pure model→string transform. It reads ONLY the in-memory model
// (model.ProjectModel) and never touches source files, the parser, or any
// other I/O. Output order follows model order exactly (Schemas in discovery
// order, Columns in source-declaration order), so identical models always
// produce byte-identical strings — which golden-file tests rely on.
package er

import (
	"strings"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

const (
	// diagramHeader opens every Mermaid entity-relationship diagram.
	diagramHeader = "erDiagram"

	// entityIndent indents entity blocks and relationship lines one level
	// under the erDiagram header. attrIndent indents attributes inside an
	// entity block one further level.
	entityIndent = "\t"
	attrIndent   = "\t\t"

	// pkMarker / fkMarker / ukMarker are Mermaid attribute key markers.
	pkMarker = "PK"
	fkMarker = "FK"
	ukMarker = "UK"

	// unknownType is the placeholder attribute type used when a Column has no
	// declared type, so the emitted attribute keeps a valid two-token shape.
	unknownType = "unknown"

	// relParentToChild is the Mermaid cardinality for a one-to-many
	// relationship: the referenced (parent) entity has exactly one row, the
	// referencing (child, FK-bearing) entity has zero-or-more. It is also the
	// cardinality drawn for an Eloquent hasMany (the current model is the "one").
	relParentToChild = "||--o{"

	// relOneToOne is the Mermaid cardinality for a one-to-one relationship,
	// drawn for an Eloquent hasOne (the current model is the "one", the target
	// is the "one" too).
	relOneToOne = "||--||"

	// relChildToParent is the Mermaid cardinality drawn for an Eloquent
	// belongsTo: the current (child) model is the "many" and points at the
	// target (parent) "one". Reads left-to-right as "many belong to one".
	relChildToParent = "}o--||"

	// relManyToMany is the Mermaid cardinality drawn for an Eloquent
	// belongsToMany: both endpoints are "zero-or-more".
	relManyToMany = "}o--o{"
)

// Eloquent relationship kinds recognized by the renderer. These mirror the
// model.Relationship.Kind values; defined here so the renderer never hardcodes
// the literals when mapping a kind to a Mermaid cardinality.
const (
	kindHasMany       = "hasMany"
	kindHasOne        = "hasOne"
	kindBelongsTo     = "belongsTo"
	kindBelongsToMany = "belongsToMany"
)

// Render turns a whole Project Model into a Mermaid erDiagram string.
//
// It renders the Schema exactly as RenderSchema does — foreign-key relationship
// lines, then one entity block per table — and then appends one relationship
// line per Eloquent association declared on the model's Models. The FK lines and
// the Eloquent lines coexist: FK lines describe what the database schema
// declares, Eloquent lines describe what the application code declares, and a
// reader can see both views at once. A nil model renders an empty diagram (just
// the header) rather than panicking.
//
// Unlike RenderSchema, Render reads m.Models, so only Render draws Eloquent
// edges. The renderer reads ONLY the in-memory model (ADR 0004); it never loads
// source files or the parser.
func Render(m *model.ProjectModel) string {
	if m == nil {
		return diagramHeader + "\n"
	}

	var b strings.Builder
	b.WriteString(diagramHeader)
	b.WriteString("\n")

	writeRelationships(&b, m.Schemas)
	writeEloquentRelationships(&b, m.Models)
	for _, t := range m.Schemas {
		writeEntity(&b, t)
	}

	return b.String()
}

// RenderSchema renders a slice of Tables into a Mermaid erDiagram string.
//
// Layout (matches the vault Diagrams Index target output):
//   - "erDiagram" header.
//   - One relationship line per foreign key whose referenced table is known
//     and present among the tables, using PARENT ||--o{ CHILD cardinality.
//   - One entity block per table, with one attribute line per column:
//     "<type> <name>" plus an optional "PK"/"FK" marker.
//
// Output is deterministic: entities and attributes follow input order, and
// relationship lines follow first-seen order across the input. The returned
// string always ends in a trailing newline.
func RenderSchema(tables []model.Table) string {
	var b strings.Builder
	b.WriteString(diagramHeader)
	b.WriteString("\n")

	writeRelationships(&b, tables)
	for _, t := range tables {
		writeEntity(&b, t)
	}

	return b.String()
}

// writeRelationships emits the relationship lines for every foreign-key column
// whose referenced table is known and present in the diagram. Foreign keys
// pointing at unknown or absent tables are skipped so no dangling line breaks
// the Mermaid output. Lines are emitted in column-discovery order across the
// tables (deterministic), de-duplicated on the (parent, child, label) triple.
func writeRelationships(b *strings.Builder, tables []model.Table) {
	known := tableNameSet(tables)
	seen := make(map[string]struct{})

	for _, child := range tables {
		for _, col := range child.Columns {
			if !col.IsForeignKey || col.References == nil {
				continue
			}
			parent := col.References.Table
			if parent == "" {
				continue
			}
			if _, ok := known[parent]; !ok {
				continue
			}

			line := relationshipLine(parent, child.Name, col.Name)
			if _, dup := seen[line]; dup {
				continue
			}
			seen[line] = struct{}{}

			b.WriteString(entityIndent)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
}

// writeEloquentRelationships emits one Mermaid relationship line per Eloquent
// association declared on the model's Models. Each line connects the declaring
// Model's table to the target Model's table, using the Mermaid cardinality that
// matches the relationship kind (hasMany → ||--o{, hasOne → ||--||, belongsTo →
// }o--||, belongsToMany → }o--o{).
//
// The target Model's table is resolved from the same Models slice: a Model name
// is mapped to its Table. A relationship whose target Model was not extracted
// (so its table is unknown) is skipped, mirroring how writeRelationships drops
// foreign keys pointing at tables the diagram does not contain — this keeps the
// renderer free of any table-naming convention and free of a dependency on the
// extractor. A relationship of an unrecognized kind is likewise skipped.
//
// Lines are emitted in Model-discovery order, then relationship-declaration
// order within each Model (the slices' natural order), and de-duplicated on the
// full line so repeated edges never appear twice. The output is deterministic.
func writeEloquentRelationships(b *strings.Builder, models []model.Model) {
	tableByModel := modelTableSet(models)
	seen := make(map[string]struct{})

	for _, m := range models {
		for _, rel := range m.Relationships {
			cardinality, ok := relationshipCardinality(rel.Kind)
			if !ok {
				continue
			}
			targetTable, known := tableByModel[rel.Target]
			if !known || targetTable == "" || m.Table == "" {
				continue
			}

			line := eloquentRelationshipLine(rel, m.Table, targetTable, cardinality)
			if _, dup := seen[line]; dup {
				continue
			}
			seen[line] = struct{}{}

			b.WriteString(entityIndent)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
}

// relationshipCardinality maps an Eloquent relationship kind to its Mermaid
// cardinality token, with the boolean reporting whether the kind is recognized.
// An unrecognized kind yields ("", false) so the caller can skip it rather than
// emit a malformed line.
func relationshipCardinality(kind string) (string, bool) {
	switch kind {
	case kindHasMany:
		return relParentToChild, true
	case kindHasOne:
		return relOneToOne, true
	case kindBelongsTo:
		return relChildToParent, true
	case kindBelongsToMany:
		return relManyToMany, true
	default:
		return "", false
	}
}

// eloquentRelationshipLine builds a single Mermaid relationship statement for an
// Eloquent association of the form
//
//	SOURCE <cardinality> TARGET : "<method> (<kind>)"
//
// The label names the relationship method and kind so a reader can tell apart
// multiple Eloquent edges between the same pair of tables and distinguish them
// from the schema's foreign-key edges.
func eloquentRelationshipLine(rel model.Relationship, sourceTable, targetTable, cardinality string) string {
	label := rel.Method + " (" + rel.Kind + ")"
	return entityName(sourceTable) + " " + cardinality + " " +
		entityName(targetTable) + " : " + quoteLabel(label)
}

// modelTableSet maps each Model name to the table it declares, so a
// relationship's target Model name can be resolved to a table entity in the
// diagram. A target name absent from this map is a Model the diagram cannot
// place, and its relationship lines are skipped.
func modelTableSet(models []model.Model) map[string]string {
	set := make(map[string]string, len(models))
	for _, m := range models {
		set[m.Name] = m.Table
	}
	return set
}

// relationshipLine builds a single Mermaid relationship statement of the form
//
//	PARENT ||--o{ CHILD : "references (fk_column)"
//
// The label names the FK column so a reader can tell apart multiple edges
// between the same pair of entities.
func relationshipLine(parentTable, childTable, fkColumn string) string {
	label := "references"
	if fkColumn != "" {
		label = "references (" + fkColumn + ")"
	}
	return entityName(parentTable) + " " + relParentToChild + " " +
		entityName(childTable) + " : " + quoteLabel(label)
}

// writeEntity emits one entity block: the entity header, an attribute line per
// column, and a closing brace. An entity with no columns still renders a
// well-formed empty block.
//
// Before emitting attributes it builds the set of column names carrying a
// single-column unique index (t.Indexes where Unique is true and Columns has
// exactly one entry), so attributeLine can mark those columns UK. A composite
// unique index (2+ columns) contributes no column to this set: Mermaid's
// erDiagram syntax has no first-class representation for a multi-column
// index, so composite, named, and non-unique indexes are not represented in
// the output at all — only single-column unique indexes surface, via the UK
// marker.
func writeEntity(b *strings.Builder, t model.Table) {
	b.WriteString(entityIndent)
	b.WriteString(entityName(t.Name))
	b.WriteString(" {\n")

	unique := singleColumnUniqueSet(t.Indexes)
	for _, col := range t.Columns {
		b.WriteString(attrIndent)
		b.WriteString(attributeLine(col, unique))
		b.WriteString("\n")
	}

	b.WriteString(entityIndent)
	b.WriteString("}\n")
}

// singleColumnUniqueSet returns the set of column names that carry a
// single-column unique index: for each idx in indexes, idx.Columns[0] is
// added when idx.Unique is true and len(idx.Columns) == 1. Composite unique
// indexes and non-unique indexes contribute nothing to the set.
func singleColumnUniqueSet(indexes []model.Index) map[string]struct{} {
	set := make(map[string]struct{})
	for _, idx := range indexes {
		if idx.Unique && len(idx.Columns) == 1 {
			set[idx.Columns[0]] = struct{}{}
		}
	}
	return set
}

// attributeLine renders one column as a Mermaid attribute: "<type> <name>"
// followed by an optional key marker. Precedence is PK > FK > UK: a column is
// marked PK when it is a primary key; otherwise FK when it is a foreign key;
// otherwise UK when its name is in unique (it carries a single-column unique
// index); otherwise it carries no marker. A single column never carries more
// than one marker.
//
// unique is the set built by singleColumnUniqueSet: only single-column unique
// indexes ever produce a UK marker. A composite unique index or a non-unique
// index is not represented in Mermaid output at all — Mermaid's erDiagram
// syntax has no first-class syntax for either, so those indexes are simply
// invisible to this renderer. The UK marker itself is a real, recognized
// Mermaid attribute-key token: the vendored parser (mermaid.min.js, 10.9.6,
// under internal/web/assets/vendor) lexes attribute keys with
// /^(?:\b((?:PK)|(?:FK)|(?:UK))\b)/i, so "UK" round-trips through Mermaid
// exactly like "PK" and "FK" do.
func attributeLine(c model.Column, unique map[string]struct{}) string {
	parts := []string{attributeType(c.Type), sanitizeToken(c.Name)}

	switch {
	case c.IsPrimaryKey:
		parts = append(parts, pkMarker)
	case c.IsForeignKey:
		parts = append(parts, fkMarker)
	default:
		if _, ok := unique[c.Name]; ok {
			parts = append(parts, ukMarker)
		}
	}

	return strings.Join(parts, " ")
}

// attributeType returns a single safe token for a column's declared type,
// falling back to unknownType when the type is empty.
func attributeType(t string) string {
	tok := sanitizeToken(t)
	if tok == "" {
		return unknownType
	}
	return tok
}

// entityName normalizes a table name into a Mermaid entity identifier:
// uppercased and reduced to a single safe token. Mermaid entity names cannot
// contain whitespace, so any internal whitespace is collapsed to underscores.
func entityName(table string) string {
	return strings.ToUpper(sanitizeToken(table))
}

// sanitizeToken collapses any run of whitespace in s into single underscores
// and trims surrounding whitespace, yielding a single token with no spaces.
// It never introduces characters that were not already present (besides the
// underscore separators), keeping the transform predictable.
func sanitizeToken(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, "_")
}

// quoteLabel wraps a relationship label in double quotes so labels containing
// spaces or parentheses are always valid Mermaid. Any embedded double quotes
// are stripped to avoid producing malformed Mermaid.
func quoteLabel(label string) string {
	clean := strings.ReplaceAll(label, "\"", "")
	return "\"" + clean + "\""
}

// tableNameSet returns the set of table names present in tables, used to decide
// whether a foreign key references a table the diagram actually contains.
func tableNameSet(tables []model.Table) map[string]struct{} {
	set := make(map[string]struct{}, len(tables))
	for _, t := range tables {
		set[t.Name] = struct{}{}
	}
	return set
}
