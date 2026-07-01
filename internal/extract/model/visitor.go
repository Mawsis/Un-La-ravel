package model

import (
	"strings"

	"github.com/VKCOM/php-parser/pkg/ast"

	domain "github.com/mawsis/unlaravel/internal/model"
	"github.com/mawsis/unlaravel/internal/phpast"
)

// This file contains the only AST-aware code in the package. Per ADR 0003 the
// parser and traverser themselves stay behind internal/phpast; the visitor must
// nonetheless name the concrete node-pointer types (*ast.StmtClass,
// *ast.StmtClassMethod, *ast.StmtProperty, *ast.ExprMethodCall) because
// php-parser's Visitor interface has one method per node type and those
// signatures reference concrete structs. All reading of node values goes through
// the phpast.* helpers so the parser surface this file depends on stays minimal.
//
// The domain model package (internal/model) is imported under the alias `domain`
// because this extractor's own package is also named `model`; the alias keeps
// the two unambiguous (domain.Model vs. this package's TableName helper).

// eloquentBases is the set of base-class last segments that mark a class as an
// Eloquent model: the framework Model, the Authenticatable user base, and the
// Pivot intermediate-table base. Membership is tested on the LAST backslash
// segment of the extends clause, so both the bare form ("Model") and the
// fully-qualified form ("Illuminate\Database\Eloquent\Model") are recognised.
var eloquentBases = map[string]bool{
	"Model":           true,
	"Authenticatable": true,
	"Pivot":           true,
}

// relationshipKinds is the set of Eloquent relationship methods this slice
// extracts. A `$this-><kind>(...)` call whose kind is in this set, made inside a
// model method, declares a Relationship. Polymorphic and through relationships
// are deliberately excluded (out of scope).
var relationshipKinds = map[string]bool{
	"hasMany":       true,
	"hasOne":        true,
	"belongsTo":     true,
	"belongsToMany": true,
}

// modelVisitor walks a single PHP file's AST and accumulates every Eloquent
// model class it declares, along with each model's explicit table name and its
// relationships. It embeds phpast.NullVisitor (a no-op for every node type) and
// overrides only the four node kinds that carry model information.
//
// State is mutated as the traverser walks; the visitor is single-use per file
// and the caller reads .models afterwards. The mutation is confined to this
// internal type — the package's public Extract function exposes only pure,
// value-returning behaviour.
//
// The traverser visits a StmtClass before its members and a StmtClassMethod
// before its body (verified — see MODEL_FACTS), so the visitor can track the
// "current" class and method and attach properties and relationship calls to the
// right place purely from visit order.
type modelVisitor struct {
	phpast.NullVisitor

	// models holds every Eloquent model declared in this file, in source order.
	models []*modelBuilder
	// current is the model whose body is being walked; properties and
	// relationship calls attach to it. nil while walking a non-Eloquent class.
	current *modelBuilder
	// curMethod is the name of the StmtClassMethod currently being walked, used
	// to name the relationship a `$this-><kind>(...)` call declares.
	curMethod string
}

// modelBuilder accumulates the data discovered for one Eloquent class, in source
// order. It is the mutable working form; the caller converts it to an immutable
// domain.Model. className is the class name; explicitTable is the value of an
// explicit `protected $table` property when one is present (empty otherwise).
type modelBuilder struct {
	className     string
	explicitTable string
	relationships []domain.Relationship
}

// newModelVisitor returns a visitor ready to walk one file's AST.
func newModelVisitor() *modelVisitor {
	return &modelVisitor{}
}

// StmtClass opens a class declaration. When the class extends an Eloquent base
// (judged by the last segment of its extends clause), a fresh model builder is
// started and recorded as current so subsequent properties and methods attach to
// it. A non-Eloquent class clears current, so its members are ignored.
func (v *modelVisitor) StmtClass(n *ast.StmtClass) {
	className := phpast.IdentifierName(n.Name)
	base := phpast.IdentifierName(n.Extends)
	if className == "" || !isEloquentBase(base) {
		v.current = nil
		return
	}

	mb := &modelBuilder{className: className}
	v.models = append(v.models, mb)
	v.current = mb
	v.curMethod = ""
}

// StmtClassMethod records the name of the method now being walked, so a
// relationship call inside its body is attributed to it. Methods of a
// non-Eloquent class are still visited but harmless: current is nil, so no
// relationship is recorded.
func (v *modelVisitor) StmtClassMethod(n *ast.StmtClassMethod) {
	v.curMethod = phpast.IdentifierName(n.Name)
}

// StmtProperty captures an explicit table name from `protected $table = '...'`.
// Only a string-literal initialiser on the $table property is recognised; any
// other property is ignored. Properties outside an Eloquent class (current nil)
// are skipped.
func (v *modelVisitor) StmtProperty(n *ast.StmtProperty) {
	if v.current == nil {
		return
	}
	if phpast.VariableName(n.Var) != "table" {
		return
	}
	if s, ok := n.Expr.(*ast.ScalarString); ok {
		v.current.explicitTable = strings.Trim(string(s.Value), `'"`)
	}
}

// ExprMethodCall handles an Eloquent relationship declaration of the form
// $this-><kind>(Target::class, 'foreign_key'). It records a Relationship when
// the receiver is $this, the called method is a supported relationship kind, and
// a model class is currently open. The relationship's method name is the
// enclosing class method (curMethod). The target is resolved from the first
// argument — a Target::class constant or a 'Namespace\Target' string reduced to
// its last segment — and an explicit foreign key from a second string argument,
// if present.
func (v *modelVisitor) ExprMethodCall(n *ast.ExprMethodCall) {
	if v.current == nil {
		return
	}
	if phpast.VariableName(n.Var) != "this" {
		return
	}
	kind := phpast.CallName(n.Method)
	if !relationshipKinds[kind] {
		return
	}

	rel := domain.Relationship{
		Kind:       kind,
		Method:     v.curMethod,
		Target:     relationshipTarget(n.Args),
		ForeignKey: phpast.NthStringArg(n.Args, 1),
	}
	v.current.relationships = append(v.current.relationships, rel)
}

// isEloquentBase reports whether a base class (the extends clause, possibly
// namespaced) marks a class as an Eloquent model. The decision is made on the
// LAST backslash segment, so both "Model" and
// "Illuminate\Database\Eloquent\Model" qualify.
func isEloquentBase(base string) bool {
	if base == "" {
		return false
	}
	return eloquentBases[lastSegment(base)]
}

// relationshipTarget resolves the related model class name from a relationship
// call's arguments. The first argument is either a Target::class constant
// (resolved via NthArgClassConst) or a string class reference such as
// 'App\Models\Tag' (resolved via NthStringArg and reduced to its last segment).
// The class-const form is tried first because it is the idiomatic, refactor-safe
// way Laravel code names a related model.
func relationshipTarget(args []phpast.Vertex) string {
	if cls := phpast.NthArgClassConst(args, 0); cls != "" {
		return lastSegment(cls)
	}
	if str := phpast.NthStringArg(args, 0); str != "" {
		return lastSegment(str)
	}
	return ""
}

// lastSegment returns the final backslash-delimited segment of a PHP class
// reference, the bare class name (e.g. "App\Models\User" -> "User"). A name with
// no backslash is returned unchanged. This is a pure string operation, kept here
// rather than in phpast because it is meaningful only to model extraction.
func lastSegment(name string) string {
	if i := strings.LastIndex(name, `\`); i >= 0 {
		return name[i+1:]
	}
	return name
}
