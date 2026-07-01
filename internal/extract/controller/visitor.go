package controller

import (
	"github.com/VKCOM/php-parser/pkg/ast"

	"github.com/mawsis/unlaravel/internal/phpast"
)

// This file contains the only AST-aware code in the package. Per ADR 0003 the
// parser and traverser themselves stay behind internal/phpast; the visitor must
// nonetheless name the concrete node-pointer types (*ast.StmtClass,
// *ast.StmtClassMethod) because php-parser's Visitor interface has one method
// per node type and those signatures reference concrete structs. All reading of
// node values goes through the phpast.* helpers so the parser surface this file
// depends on stays minimal.
//
// Unlike the model extractor, this package needs no `domain` import alias: the
// visitor accumulates only short names, and controller.go assembles the
// domain.Controller (with its FQN from the file namespace) after the walk.

// constructorName is the reserved method name of a class constructor. It is the
// one public method that never serves as a routable Action, so it is excluded
// from a controller's action list. Named here so the exclusion reads by intent
// rather than as a bare string literal.
const constructorName = "__construct"

// controllerVisitor walks a single PHP file's AST and accumulates every class it
// declares along with each class's public, routable method names. It embeds
// phpast.NullVisitor (a no-op for every node type) and overrides only the two
// node kinds that carry controller information: the class declaration and its
// methods.
//
// Every class is collected, not just those extending a framework Controller
// base: Laravel controllers routinely extend the app's own base controller,
// invokable controllers may extend nothing, and phase-two resolution (ADR 0006)
// matches a Route's controller reference against these FQNs by name — so the
// extractor records the classes and lets resolution decide relevance. State is
// mutated as the traverser walks; the visitor is single-use per file and the
// caller reads .classes afterwards.
//
// The traverser visits a StmtClass before its members (verified — see the model
// extractor's MODEL_FACTS note), so the visitor tracks the "current" class and
// attaches each method to it purely from visit order.
type controllerVisitor struct {
	phpast.NullVisitor

	// classes holds every class declared in this file, in source order.
	classes []*classBuilder
	// current is the class whose body is being walked; methods attach to it.
	// Never nil once a StmtClass has opened, since every class is collected.
	current *classBuilder
}

// classBuilder accumulates the data discovered for one class, in source order.
// It is the mutable working form; controller.go converts it to an immutable
// domain.Controller once the file's namespace is known. name is the class short
// name; actions are its public, non-constructor method names in declaration
// order.
//
// actionParams records, per action, the type-hint names of its parameters in
// declaration order (untyped parameters are omitted — see
// phpast.ParamTypeNames). This is side data used to link a Route to a
// FormRequest during phase-two analysis; it is NOT part of the domain.Controller
// JSON contract, so controller.go exposes it separately from the Controller
// slice. An action with no typed parameters has no entry here.
type classBuilder struct {
	name         string
	actions      []string
	actionParams map[string][]string
}

// newControllerVisitor returns a visitor ready to walk one file's AST.
func newControllerVisitor() *controllerVisitor {
	return &controllerVisitor{}
}

// StmtClass opens a class declaration. A fresh class builder is started and
// recorded as current so subsequent methods attach to it. An anonymous or
// unnamed class (no name) is skipped by clearing current, so its methods are
// ignored — Laravel controllers are always named top-level classes.
func (v *controllerVisitor) StmtClass(n *ast.StmtClass) {
	name := phpast.IdentifierName(n.Name)
	if name == "" {
		v.current = nil
		return
	}

	cb := &classBuilder{name: name, actionParams: make(map[string][]string)}
	v.classes = append(v.classes, cb)
	v.current = cb
}

// StmtClassMethod records a method as an Action when it is public and routable.
// A method qualifies when its visibility is public — either an explicit `public`
// modifier or no visibility modifier at all, since PHP methods default to public
// — and it is not the constructor. The constructor, and any private or protected
// method, is skipped. __invoke is deliberately NOT excluded: it is the action of
// a single-action (invokable) controller. Methods outside a named class (current
// nil) are ignored.
func (v *controllerVisitor) StmtClassMethod(n *ast.StmtClassMethod) {
	if v.current == nil {
		return
	}
	name := phpast.IdentifierName(n.Name)
	if name == "" || name == constructorName {
		return
	}
	if !isPublicMethod(n.Modifiers) {
		return
	}
	v.current.actions = append(v.current.actions, name)
	if types := phpast.ParamTypeNames(n); len(types) > 0 {
		v.current.actionParams[name] = types
	}
}

// isPublicMethod reports whether a method's modifier list denotes public
// visibility. A method is public when it carries an explicit `public` modifier
// or carries no visibility modifier at all (PHP's default), and is NOT public
// when it carries `private` or `protected`. Non-visibility modifiers such as
// `static`, `final`, and `abstract` are irrelevant to routability and ignored.
//
// The modifiers are *ast.Identifier nodes; their names are read through
// phpast.IdentifierName so this package touches no php-parser value directly.
func isPublicMethod(modifiers []phpast.Vertex) bool {
	for _, m := range modifiers {
		switch phpast.IdentifierName(m) {
		case "private", "protected":
			return false
		}
	}
	return true
}
