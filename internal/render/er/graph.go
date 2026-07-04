// graph.go is the structured ER-graph Renderer (issue #21): it turns a Project
// Model into an ERGraph of nodes (tables + their columns) and edges (schema
// foreign keys + Eloquent associations), the data contract the browser ER
// renderer draws instead of a Mermaid string.
//
// Like the Mermaid Render, RenderGraph is a pure model→struct transform: it
// reads ONLY the in-memory model, never source files or the parser, and emits
// in model-discovery order so a fixed model always produces a byte-identical
// graph (golden-file testable). The domain rules — PK > FK > UK marker
// precedence and single-column-unique → UK — are shared with the Mermaid
// renderer; here they surface as struct fields rather than string tokens.
package er

import "github.com/Mawsis/Un-La-ravel/internal/model"

// Cardinality kinds for graph edges — the structured counterparts of the
// Mermaid cardinality tokens. A schema foreign key is one-to-many (parent has
// one, FK-bearing child has many); the Eloquent kinds map hasMany → one-to-many,
// hasOne → one-to-one, belongsTo → many-to-one, belongsToMany → many-to-many.
const (
	cardOneToMany  = "one-to-many"
	cardOneToOne   = "one-to-one"
	cardManyToOne  = "many-to-one"
	cardManyToMany = "many-to-many"
)

// ERGraph is the structured entity-relationship graph: table nodes and the
// relationship edges between them, in deterministic discovery order.
type ERGraph struct {
	Nodes []ERNode `json:"nodes"`
	Edges []EREdge `json:"edges"`
}

// ERNode is one table: its name plus its columns in source-declaration order.
type ERNode struct {
	Table   string     `json:"table"`
	Columns []ERColumn `json:"columns"`
}

// ERColumn is one column: its name, declared type, and key marker. Key is one
// of "PK", "FK", "UK", or "" (no marker), following PK > FK > UK precedence.
type ERColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Key  string `json:"key"`
}

// EREdge is one relationship between two tables, with its cardinality kind and
// a human-readable label. From/To are table names present among the nodes.
type EREdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// RenderGraph turns a whole Project Model into an ERGraph. A nil model renders
// an empty graph (no nodes, no edges) rather than panicking.
func RenderGraph(m *model.ProjectModel) ERGraph {
	graph := ERGraph{Nodes: []ERNode{}, Edges: []EREdge{}}
	if m == nil {
		return graph
	}

	for _, t := range m.Schemas {
		graph.Nodes = append(graph.Nodes, tableNode(t))
	}
	graph.Edges = append(graph.Edges, schemaEdges(m.Schemas)...)
	graph.Edges = append(graph.Edges, eloquentEdges(m.Models)...)

	return graph
}

// eloquentEdges emits one edge per Eloquent association whose target Model
// resolves to a table: From is the declaring Model's table, To the target's,
// Kind the association's cardinality, Label "<method> (<kind>)". Associations of
// an unrecognized kind, or whose target Model was not extracted (unknown table),
// are dropped — mirroring how schemaEdges drops foreign keys to absent tables.
// Edges follow Model-discovery then declaration order, de-duplicated on the full
// edge value.
func eloquentEdges(models []model.Model) []EREdge {
	tableByModel := modelTableSet(models)
	seen := make(map[EREdge]struct{})
	edges := []EREdge{}

	for _, mdl := range models {
		if mdl.Table == "" {
			continue
		}
		for _, rel := range mdl.Relationships {
			kind, ok := edgeCardinality(rel.Kind)
			if !ok {
				continue
			}
			targetTable, known := tableByModel[rel.Target]
			if !known || targetTable == "" {
				continue
			}

			edge := EREdge{
				From:  mdl.Table,
				To:    targetTable,
				Kind:  kind,
				Label: rel.Method + " (" + rel.Kind + ")",
			}
			if _, dup := seen[edge]; dup {
				continue
			}
			seen[edge] = struct{}{}
			edges = append(edges, edge)
		}
	}
	return edges
}

// edgeCardinality maps an Eloquent relationship kind to its graph cardinality,
// reporting false for an unrecognized kind so the caller skips it. Mirrors the
// Mermaid renderer's relationshipCardinality, but yields the structured kind.
func edgeCardinality(kind string) (string, bool) {
	switch kind {
	case kindHasMany:
		return cardOneToMany, true
	case kindHasOne:
		return cardOneToOne, true
	case kindBelongsTo:
		return cardManyToOne, true
	case kindBelongsToMany:
		return cardManyToMany, true
	default:
		return "", false
	}
}

// schemaEdges emits one parent→child edge per foreign-key column whose
// referenced table is present among the tables. Foreign keys to unknown or
// absent tables are dropped so no dangling edge appears. Edges follow
// column-discovery order across the tables and are de-duplicated on the full
// edge value, mirroring the Mermaid renderer's writeRelationships.
func schemaEdges(tables []model.Table) []EREdge {
	known := tableNameSet(tables)
	seen := make(map[EREdge]struct{})
	edges := []EREdge{}

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

			edge := EREdge{
				From:  parent,
				To:    child.Name,
				Kind:  cardOneToMany,
				Label: fkLabel(col.Name),
			}
			if _, dup := seen[edge]; dup {
				continue
			}
			seen[edge] = struct{}{}
			edges = append(edges, edge)
		}
	}
	return edges
}

// fkLabel names the foreign-key column so multiple edges between the same table
// pair stay distinguishable, matching the Mermaid renderer's label text.
func fkLabel(fkColumn string) string {
	if fkColumn == "" {
		return "references"
	}
	return "references (" + fkColumn + ")"
}

// tableNode builds one ERNode from a table: its name plus one ERColumn per
// column, in source order, each carrying its key marker (PK > FK > UK).
func tableNode(t model.Table) ERNode {
	unique := singleColumnUniqueSet(t.Indexes)
	columns := make([]ERColumn, 0, len(t.Columns))
	for _, col := range t.Columns {
		columns = append(columns, ERColumn{
			Name: col.Name,
			Type: col.Type,
			Key:  columnKey(col, unique),
		})
	}
	return ERNode{Table: t.Name, Columns: columns}
}

// columnKey returns the key marker for a column following PK > FK > UK
// precedence: PK when it is a primary key; else FK when it is a foreign key;
// else UK when it carries a single-column unique index; else "" (no marker).
func columnKey(c model.Column, unique map[string]struct{}) string {
	switch {
	case c.IsPrimaryKey:
		return pkMarker
	case c.IsForeignKey:
		return fkMarker
	default:
		if _, ok := unique[c.Name]; ok {
			return ukMarker
		}
		return ""
	}
}
