package formrequest

import (
	"github.com/VKCOM/php-parser/pkg/ast"

	"github.com/mawsis/unlaravel/internal/phpast"
)

// This file contains the only AST-aware code in the package. Per ADR 0003 the
// parser and traverser stay behind internal/phpast; the visitor must nonetheless
// name the concrete node-pointer types (*ast.StmtClass, *ast.StmtClassMethod)
// because php-parser's Visitor interface has one method per node type and those
// signatures reference concrete structs. All reading of node values goes through
// the phpast.* helpers so the parser surface this file depends on stays minimal.
//
// Unlike the controller extractor, this visitor collects only the classes that
// are FormRequests (a base class whose short name is "FormRequest"): a
// non-FormRequest class in app/Http/Requests carries no rules() ruleset the
// OpenAPI renderer can use, so it is ignored rather than emitted empty.

// rulesMethod is the reserved method name whose returned array holds a
// FormRequest's validation ruleset. Named here so the match reads by intent
// rather than as a bare string literal.
const rulesMethod = "rules"

// formRequestVisitor walks a single PHP file's AST and accumulates every class
// that is a FormRequest, along with the parsed fields of its rules() method. It
// embeds phpast.NullVisitor (a no-op for every node type) and overrides only the
// two node kinds that carry the information it needs: the class declaration and
// its methods.
//
// The traverser visits a StmtClass before its members (verified — see the
// controller extractor), so the visitor tracks the "current" class and attaches
// its rules() fields purely from visit order. A class that is not a FormRequest
// clears current, so its methods are skipped.
type formRequestVisitor struct {
	phpast.NullVisitor

	// classes holds every FormRequest class declared in this file, in source
	// order.
	classes []*classBuilder
	// current is the FormRequest class whose body is being walked; its rules()
	// method attaches to it. Nil while walking a non-FormRequest class, so that
	// class's methods are ignored.
	current *classBuilder
}

// classBuilder accumulates the data discovered for one FormRequest class. It is
// the mutable working form; formrequest.go converts it to an immutable
// model.FormRequest once the file's namespace is known.
//
// name is the class short name. fields are the raw key/value pairs of the
// rules() array in source order — each pair's Key is the field-name scalar and
// its Val the rule string or rule array — left unparsed here so all rule-token
// parsing lives in one place (formrequest.go). It is nil for a FormRequest whose
// rules() method returns no array; empty for one that returns an empty array.
type classBuilder struct {
	name   string
	fields []phpast.ArrayPair
}

// newFormRequestVisitor returns a visitor ready to walk one file's AST.
func newFormRequestVisitor() *formRequestVisitor {
	return &formRequestVisitor{}
}

// StmtClass opens a class declaration. When the class is a FormRequest (see
// isFormRequest) a fresh builder is started and recorded as current so its
// rules() method attaches to it; otherwise current is cleared so the class's
// methods are ignored. An anonymous or unnamed class is likewise skipped.
func (v *formRequestVisitor) StmtClass(n *ast.StmtClass) {
	name := phpast.IdentifierName(n.Name)
	if name == "" || !isFormRequest(n) {
		v.current = nil
		return
	}

	cb := &classBuilder{name: name}
	v.classes = append(v.classes, cb)
	v.current = cb
}

// StmtClassMethod parses the rules() method of the current FormRequest class,
// recording its request-body fields. Any other method, and any method outside a
// FormRequest class (current nil), is ignored. A rules() method that does not
// return an array leaves the class with no fields.
func (v *formRequestVisitor) StmtClassMethod(n *ast.StmtClassMethod) {
	if v.current == nil {
		return
	}
	if phpast.IdentifierName(n.Name) != rulesMethod {
		return
	}
	v.current.fields = phpast.ArrayPairs(phpast.MethodReturnExpr(n))
}

// isFormRequest reports whether a class declaration extends a FormRequest base
// class. It matches on the LAST backslash segment of the parent class name being
// "FormRequest", which recognises both the unqualified `extends FormRequest`
// (the common Laravel form, imported via `use`) and the fully-qualified
// `extends Illuminate\Foundation\Http\FormRequest`. A class with no `extends`
// clause is not a FormRequest.
func isFormRequest(class *ast.StmtClass) bool {
	return shortName(phpast.ClassExtends(class)) == formRequestBase
}
