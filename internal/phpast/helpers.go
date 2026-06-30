package phpast

import (
	"strings"

	"github.com/VKCOM/php-parser/pkg/ast"
	"github.com/VKCOM/php-parser/pkg/visitor"
)

// NullVisitor is an embeddable no-op visitor with a method for every node type.
// Build a partial visitor by embedding it and overriding only the per-node
// methods you need (e.g. ExprStaticCall, ExprMethodCall). Re-exported here so
// extractors never import the parser's visitor package directly.
//
//	type schemaVisitor struct {
//	    phpast.NullVisitor
//	    // ... state
//	}
//	func (v *schemaVisitor) ExprStaticCall(n *ast.ExprStaticCall) { ... }
type NullVisitor = visitor.Null

// quoteChars are the surrounding delimiters on a PHP string literal's raw
// value. ScalarString.Value INCLUDES these quotes, so they must be stripped.
const quoteChars = `'"`

// IdentifierName returns the textual name of an identifier-like node. It handles
// the two shapes extractors encounter: a bare *ast.Identifier (e.g. a method
// name such as create), and a (possibly namespaced) *ast.Name whose parts are
// joined with a backslash (e.g. Illuminate\Support\Facades\Schema). Any other
// node type yields "".
func IdentifierName(v Vertex) string {
	switch n := v.(type) {
	case *ast.Name:
		return joinNameParts(n.Parts)
	case *ast.Identifier:
		return string(n.Value)
	}
	return ""
}

// CallName returns the name of a call target when it is a bare *ast.Identifier
// — the common case for the .Call of an ExprStaticCall and the .Method of an
// ExprMethodCall (e.g. create, string, nullable). Returns "" for any other
// node type. Use IdentifierName when the target may be a namespaced *ast.Name.
func CallName(v Vertex) string {
	if id, ok := v.(*ast.Identifier); ok {
		return string(id.Value)
	}
	return ""
}

// NameParts returns the dotted/namespaced parts of an *ast.Name as a slice of
// strings (e.g. ["Illuminate", "Support", "Facades", "Schema"]). Returns nil
// for any other node type. Useful when a caller needs the segments rather than
// the joined backslash form returned by IdentifierName.
func NameParts(v Vertex) []string {
	name, ok := v.(*ast.Name)
	if !ok {
		return nil
	}
	parts := make([]string, 0, len(name.Parts))
	for _, p := range name.Parts {
		if np, ok := p.(*ast.NamePart); ok {
			parts = append(parts, string(np.Value))
		}
	}
	return parts
}

// FirstStringArg returns the unquoted value of the first string-literal argument
// in an Args list (e.g. the "users" in Schema::create('users', ...)). Each entry
// in args is expected to be an *ast.Argument wrapping the actual expression.
// Non-string arguments are skipped. Returns "" when no string argument is found.
func FirstStringArg(args []Vertex) string {
	for _, a := range args {
		if s, ok := stringArgValue(a); ok {
			return s
		}
	}
	return ""
}

// joinNameParts joins the segments of an *ast.Name's parts with a backslash,
// matching PHP's namespace separator.
func joinNameParts(parts []ast.Vertex) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if np, ok := p.(*ast.NamePart); ok {
			out = append(out, string(np.Value))
		}
	}
	return strings.Join(out, `\`)
}

// stringArgValue extracts the unquoted value of an *ast.Argument whose
// expression is a string literal. The bool reports whether extraction succeeded.
func stringArgValue(arg Vertex) (string, bool) {
	a, ok := arg.(*ast.Argument)
	if !ok {
		return "", false
	}
	s, ok := a.Expr.(*ast.ScalarString)
	if !ok {
		return "", false
	}
	return strings.Trim(string(s.Value), quoteChars), true
}
