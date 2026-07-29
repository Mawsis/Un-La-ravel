package phpast

import (
	"github.com/VKCOM/php-parser/pkg/ast"
)

// This file adds the class-property readers the middleware extractor needs to
// read an HTTP Kernel (issue #66): locating a named property's array-literal
// value on a named class, and reducing the three array shapes a Kernel declares
// — a keyed alias→Class::class map ($middlewareAliases / $routeMiddleware), a
// keyed group→[Class::class, ...] map ($middlewareGroups), and a flat list of
// Class::class ($middleware global stack, $middlewarePriority) — to plain Go
// values. Like every other helper here, these keep the extractor free of any
// direct php-parser import (ADR 0003): the extractor names its own visitor
// methods against concrete node types (the interface forces that), but reads all
// values through these functions.

// ClassPropertyArray returns the array-literal value expression assigned to the
// property named propName (WITHOUT its leading "$") on the class named className,
// found under the AST root. It descends into a top-level *ast.StmtNamespace so a
// conventionally-namespaced class (`namespace App\Http; class Kernel {...}`) is
// reached, and reads the property from the class's *ast.StmtPropertyList entries.
//
// It returns nil when root is not an *ast.Root, no class of that name is
// declared, the class has no such property, or the property's value is not an
// array literal (for example `protected $foo = 'x';`). The returned Vertex is an
// *ast.ExprArray suitable for ArrayClassConstPairs / ArrayClassConstGroups /
// ArrayClassConstItems (or the existing ArrayStringItems family). className and
// propName are matched by exact short name — the class's declared name, not an
// FQN — matching how a Kernel is read by its conventional class name.
func ClassPropertyArray(root Vertex, className, propName string) Vertex {
	class := findClass(root, className)
	if class == nil {
		return nil
	}
	for _, st := range class.Stmts {
		list, ok := st.(*ast.StmtPropertyList)
		if !ok {
			continue
		}
		for _, p := range list.Props {
			prop, ok := p.(*ast.StmtProperty)
			if !ok {
				continue
			}
			if VariableName(prop.Var) != propName {
				continue
			}
			if _, ok := prop.Expr.(*ast.ExprArray); ok {
				return prop.Expr
			}
			// The property exists but its value is not an array literal — reject it
			// (a Kernel property we cannot read as an array yields no data).
			return nil
		}
	}
	return nil
}

// findClass returns the first *ast.StmtClass named className declared directly
// under the root or one level inside a top-level *ast.StmtNamespace, or nil when
// none is found. It mirrors DeclaredClasses' one-level namespace descent: the
// single-namespace-per-file convention Laravel code follows.
func findClass(root Vertex, className string) *ast.StmtClass {
	r, ok := root.(*ast.Root)
	if !ok {
		return nil
	}
	if c := classIn(r.Stmts, className); c != nil {
		return c
	}
	return nil
}

// classIn scans stmts for an *ast.StmtClass named className, descending one level
// into any *ast.StmtNamespace, and returns the first match or nil.
func classIn(stmts []Vertex, className string) *ast.StmtClass {
	for _, st := range stmts {
		switch n := st.(type) {
		case *ast.StmtClass:
			if IdentifierName(n.Name) == className {
				return n
			}
		case *ast.StmtNamespace:
			if c := classIn(n.Stmts, className); c != nil {
				return c
			}
		}
	}
	return nil
}

// ClassConstPair is one key/value entry of a PHP array literal whose key is a
// string literal and whose value is a `Class::class` constant, both already
// reduced to plain strings (e.g. Key: "auth", Class: "App\Http\Middleware\
// Authenticate" for the $middlewareAliases entry 'auth' => Authenticate::class).
// The Class is the IdentifierName of the class-const fetch, so a leading-
// backslash reference (`\App\...::class`) is normalised to the codebase's no-
// leading-backslash FQN convention.
type ClassConstPair struct {
	Key   string
	Class string
}

// ArrayClassConstPairs reduces a keyed PHP array literal whose values are
// `Class::class` fetches to a []ClassConstPair, preserving source order (e.g.
// $middlewareAliases = ['auth' => Authenticate::class, ...] yields
// [{Key: "auth", Class: "...Authenticate"}]). Returns nil when v is not an
// *ast.ExprArray, mirroring ArrayPairs.
//
// A pair is skipped when its key is not a string literal or its value is not a
// class-const fetch (ClassConstClass(val) == ""), so a malformed or dynamic
// entry is dropped rather than guessed (precision over coverage, ADR 0002). A
// list-style entry (no key) is likewise skipped — an alias map is keyed by
// definition.
func ArrayClassConstPairs(v Vertex) []ClassConstPair {
	pairs := ArrayPairs(v)
	if pairs == nil {
		return nil
	}
	out := make([]ClassConstPair, 0, len(pairs))
	for _, p := range pairs {
		key := StringLiteral(p.Key)
		if key == "" {
			continue
		}
		class := ClassConstClass(p.Val)
		if class == "" {
			continue
		}
		out = append(out, ClassConstPair{Key: key, Class: class})
	}
	return out
}

// ClassConstGroup is one key/value entry of a PHP array literal whose key is a
// string literal and whose value is a list of `Class::class` constants, reduced
// to plain strings (e.g. Key: "web", Classes: ["...EncryptCookies",
// "...StartSession"] for the $middlewareGroups entry 'web' => [EncryptCookies::
// class, StartSession::class]). Classes is non-nil for a matched key: an empty
// group list yields an empty (never nil) slice.
type ClassConstGroup struct {
	Key     string
	Classes []string
}

// ArrayClassConstGroups reduces a keyed PHP array literal whose values are lists
// of `Class::class` fetches to a []ClassConstGroup, preserving source order (e.g.
// $middlewareGroups = ['web' => [A::class, B::class], ...]). Returns nil when v
// is not an *ast.ExprArray, mirroring ArrayPairs.
//
// A pair is skipped when its key is not a string literal or its value is not an
// array literal (a non-list group value is dropped, not guessed). Within a kept
// group the class list is ArrayClassConstItems of the value, so non-class-const
// items inside the list are individually skipped.
func ArrayClassConstGroups(v Vertex) []ClassConstGroup {
	pairs := ArrayPairs(v)
	if pairs == nil {
		return nil
	}
	out := make([]ClassConstGroup, 0, len(pairs))
	for _, p := range pairs {
		key := StringLiteral(p.Key)
		if key == "" {
			continue
		}
		classes := ArrayClassConstItems(p.Val)
		if classes == nil {
			// Value is not an array literal — skip this group rather than emit a
			// nil-classed entry.
			continue
		}
		out = append(out, ClassConstGroup{Key: key, Classes: classes})
	}
	return out
}

// ArrayClassConstItems reduces a PHP array literal's `Class::class` VALUES to a
// []string, built on ArrayItems + ClassConstClass (e.g. $middleware =
// [TrustProxies::class, ...] yields ["App\Http\Middleware\TrustProxies", ...]).
// Returns nil when v is not an *ast.ExprArray, mirroring ArrayItems; for a
// zero-item array literal it returns the same empty-but-non-nil slice ArrayItems
// returns for that case.
//
// An item is skipped when it is not a class-const fetch (ClassConstClass(item)
// == ""), so a plain string, a variable, or any dynamic reference in a global-
// or priority-stack list is dropped rather than guessed (ADR 0002).
func ArrayClassConstItems(v Vertex) []string {
	items := ArrayItems(v)
	if items == nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if class := ClassConstClass(item); class != "" {
			out = append(out, class)
		}
	}
	return out
}
