package schema

import (
	"github.com/VKCOM/php-parser/pkg/ast"

	"github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/phpast"
)

// This file contains the only AST-aware code in the package. Per ADR 0003 the
// parser and traverser themselves stay behind internal/phpast; the visitor must
// nonetheless name the concrete node-pointer types (*ast.ExprStaticCall,
// *ast.ExprMethodCall) because php-parser's Visitor interface has one method per
// node type and those signatures reference concrete structs. All reading of
// node values goes through the phpast.* helpers so the parser surface this file
// depends on stays minimal.

// schemaVisitor walks a single migration file's AST and accumulates the tables
// and columns it declares. It embeds phpast.NullVisitor (a no-op for every node
// type) and overrides only the two calls that carry schema information.
//
// State is mutated as the traverser walks; the visitor is single-use per file
// and the caller reads .tables afterwards. The mutation is confined to this
// internal type — the package's public Extract function exposes only pure,
// value-returning behaviour.
type schemaVisitor struct {
	phpast.NullVisitor

	// tables holds every table touched by this file, in first-seen order.
	tables []*tableBuilder
	// byName indexes tables for O(1) merge of Schema::table() alterations.
	byName map[string]*tableBuilder
	// current is the table whose closure is being walked; column method calls
	// attach to it. The most-recent Schema:: call wins, which is correct for
	// depth-first source-order traversal of Laravel's one-closure-per-call form.
	current *tableBuilder
	// seenBase dedupes chained column definitions: a chain's base call node is
	// recorded once, on the outermost (first-visited) link of the chain.
	seenBase map[ast.Vertex]bool
}

// tableBuilder accumulates the columns discovered for one table name, in source
// order. It is the mutable working form; the caller converts it to an immutable
// model.Table.
type tableBuilder struct {
	name    string
	columns []model.Column
}

// newSchemaVisitor returns a visitor ready to walk one file's AST.
func newSchemaVisitor() *schemaVisitor {
	return &schemaVisitor{
		byName:   make(map[string]*tableBuilder),
		seenBase: make(map[ast.Vertex]bool),
	}
}

// ExprStaticCall handles Schema::create('t', ...) and Schema::table('t', ...).
// create starts a brand-new table; table() opens an alteration, merging columns
// into a table of that name (creating a stub if the file alters a table it did
// not itself create). Any other static call is ignored.
func (v *schemaVisitor) ExprStaticCall(n *ast.ExprStaticCall) {
	if phpast.IdentifierName(n.Class) != "Schema" {
		return
	}
	call := phpast.CallName(n.Call)
	if call != "create" && call != "table" {
		return
	}
	name := phpast.FirstStringArg(n.Args)
	if name == "" {
		return
	}
	v.current = v.tableFor(name)
}

// ExprMethodCall handles fluent column definitions inside a Schema closure, e.g.
// $table->string('email')->unique()->nullable(). The traverser visits the
// OUTERMOST link first, so the first visit of a chain sees the complete modifier
// list. We collect the whole chain, record the column once keyed on its base
// node, and skip inner links (and re-visits) via seenBase.
func (v *schemaVisitor) ExprMethodCall(n *ast.ExprMethodCall) {
	if v.current == nil {
		return
	}

	base := chainBase(n)
	if base == nil {
		// Not a $table->...() column chain (e.g. some unrelated method call).
		return
	}
	if v.seenBase[base] {
		return
	}
	v.seenBase[base] = true

	links := chainLinks(n)
	if len(links) == 0 {
		return
	}

	// links is outermost..innermost; the innermost is the base builder.
	baseLink := links[len(links)-1]
	modifierLinks := links[:len(links)-1]

	cols := columnsForMethod(baseLink.Method, baseLink.StringArg)
	if len(cols) == 0 {
		return
	}
	cols = applyModifiers(cols, modifierLinks)

	v.current.columns = append(v.current.columns, cols...)
}

// tableFor returns the builder for name, creating and registering it on first
// use so Schema::table() alterations merge into the same table.
func (v *schemaVisitor) tableFor(name string) *tableBuilder {
	if tb, ok := v.byName[name]; ok {
		return tb
	}
	tb := &tableBuilder{name: name}
	v.byName[name] = tb
	v.tables = append(v.tables, tb)
	return tb
}

// chainBase walks inward through .Var links and returns the innermost
// ExprMethodCall whose receiver is the $table variable — the base column
// builder. It returns nil when the chain is not rooted in a plain variable
// receiver (so unrelated method calls are ignored).
func chainBase(n *ast.ExprMethodCall) ast.Vertex {
	cur := n
	for {
		inner, ok := cur.Var.(*ast.ExprMethodCall)
		if !ok {
			// cur is the base builder; its receiver is the variable ($table).
			if _, isVar := cur.Var.(*ast.ExprVariable); isVar {
				return cur
			}
			return nil
		}
		cur = inner
	}
}

// chainLinks reduces a method-call chain to outermost..innermost chainCall
// values (method name + first string arg), dropping all token/position detail so
// the modifier logic stays AST-free. n is the outermost link.
func chainLinks(n *ast.ExprMethodCall) []chainCall {
	var links []chainCall
	cur := n
	for {
		links = append(links, chainCall{
			Method:    phpast.CallName(cur.Method),
			StringArg: phpast.FirstStringArg(cur.Args),
		})
		inner, ok := cur.Var.(*ast.ExprMethodCall)
		if !ok {
			return links
		}
		cur = inner
	}
}
