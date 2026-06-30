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

	"github.com/mawsis/unlaravel/internal/model"
)

const (
	// diagramHeader opens every Mermaid entity-relationship diagram.
	diagramHeader = "erDiagram"

	// entityIndent indents entity blocks and relationship lines one level
	// under the erDiagram header. attrIndent indents attributes inside an
	// entity block one further level.
	entityIndent = "\t"
	attrIndent   = "\t\t"

	// pkMarker / fkMarker are Mermaid attribute key markers.
	pkMarker = "PK"
	fkMarker = "FK"

	// unknownType is the placeholder attribute type used when a Column has no
	// declared type, so the emitted attribute keeps a valid two-token shape.
	unknownType = "unknown"

	// relParentToChild is the Mermaid cardinality for a one-to-many
	// relationship: the referenced (parent) entity has exactly one row, the
	// referencing (child, FK-bearing) entity has zero-or-more.
	relParentToChild = "||--o{"
)

// Render turns a whole Project Model into a Mermaid erDiagram string. It is a
// thin wrapper over RenderSchema operating on m.Schemas. A nil model renders an
// empty diagram (just the header) rather than panicking.
func Render(m *model.ProjectModel) string {
	if m == nil {
		return diagramHeader + "\n"
	}
	return RenderSchema(m.Schemas)
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
func writeEntity(b *strings.Builder, t model.Table) {
	b.WriteString(entityIndent)
	b.WriteString(entityName(t.Name))
	b.WriteString(" {\n")

	for _, col := range t.Columns {
		b.WriteString(attrIndent)
		b.WriteString(attributeLine(col))
		b.WriteString("\n")
	}

	b.WriteString(entityIndent)
	b.WriteString("}\n")
}

// attributeLine renders one column as a Mermaid attribute: "<type> <name>"
// followed by an optional key marker. A column is marked PK when it is a
// primary key, otherwise FK when it is a foreign key (PK takes precedence so a
// single column never carries two markers).
func attributeLine(c model.Column) string {
	parts := []string{attributeType(c.Type), sanitizeToken(c.Name)}

	switch {
	case c.IsPrimaryKey:
		parts = append(parts, pkMarker)
	case c.IsForeignKey:
		parts = append(parts, fkMarker)
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
