package phpast_test

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/VKCOM/php-parser/pkg/ast"
)

// ---------------------------------------------------------------------------
// PHP snippets exercised by the route-helper tests
// ---------------------------------------------------------------------------

// routeSnippet is a minimal but realistic route file that exercises every
// shape the route extractor helpers need to read:
//   - a namespace declaration
//   - several use imports, including one with an alias
//   - a Route::get static call (StaticCallParts)
//   - a Route::get call whose second argument is an array callable ([C::class,'m'])
//   - a chained ->middleware(...) method call (MethodCallParts)
//   - a ->group(function () { ... }) closure (ClosureStmts)
//   - a string literal inside an array (StringLiteral)
const routeSnippet = `<?php

namespace App\Http\Controllers;

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\PostController;
use App\Http\Controllers\UserController as UserCtrl;

Route::get('/posts', [PostController::class, 'index']);

Route::middleware('auth')->group(function () {
    Route::get('/users', [UserCtrl::class, 'list']);
});
`

// closureSnippet is a minimal file whose single expression is a closure so
// ClosureStmts can be tested directly via a Walk.
const closureSnippet = `<?php
$fn = function () {
    echo 'hello';
    return 1;
};
`

// arraySnippet is a minimal file containing an array literal with two items so
// ArrayItems can be tested via a Walk.
const arraySnippet = `<?php
$a = [PostController::class, 'index'];
`

// stringLiteralSnippet exercises StringLiteral via a bare scalar string inside
// an array (the unwrapped form that StringLiteral is designed to handle).
const stringLiteralSnippet = `<?php
$a = ['hello', "world"];
`

// ---------------------------------------------------------------------------
// Helpers shared by route tests
// ---------------------------------------------------------------------------

// stmtExprCollector gathers the expression inside every top-level
// *ast.StmtExpression so tests can hand them to ExpressionStmt, StaticCallParts,
// and MethodCallParts without importing the parser.
type stmtExprCollector struct {
	phpast.NullVisitor
	// We only want top-level exprs; the Walk depth does not matter here because
	// we verify through the helpers only.
	stmtExprs []phpast.Vertex
}

func (c *stmtExprCollector) StmtExpression(n *ast.StmtExpression) {
	c.stmtExprs = append(c.stmtExprs, n)
}

// closureCollector captures every *ast.ExprClosure visited.
type closureCollector struct {
	phpast.NullVisitor
	closures []phpast.Vertex
}

func (c *closureCollector) ExprClosure(n *ast.ExprClosure) {
	c.closures = append(c.closures, n)
}

// arrayCollector captures every *ast.ExprArray visited.
type arrayCollector struct {
	phpast.NullVisitor
	arrays []phpast.Vertex
}

func (c *arrayCollector) ExprArray(n *ast.ExprArray) {
	c.arrays = append(c.arrays, n)
}

// scalarStringCollector captures every *ast.ScalarString visited.
type scalarStringCollector struct {
	phpast.NullVisitor
	scalars []phpast.Vertex
}

func (c *scalarStringCollector) ScalarString(n *ast.ScalarString) {
	c.scalars = append(c.scalars, n)
}

// methodCallCollectorRoute collects ExprMethodCall nodes so MethodCallParts
// can be tested. (Renamed to avoid a redeclaration with phpast_test.go's
// methodCallCollector which is in the same test package.)
type methodCallCollectorRoute struct {
	phpast.NullVisitor
	calls []phpast.Vertex
}

func (c *methodCallCollectorRoute) ExprMethodCall(n *ast.ExprMethodCall) {
	c.calls = append(c.calls, n)
}

// staticCallCollectorRoute collects ExprStaticCall nodes for MethodCallParts /
// StaticCallParts. Named differently from the one in phpast_test.go to avoid
// redeclaration.
type staticCallCollectorRoute struct {
	phpast.NullVisitor
	calls []phpast.Vertex
}

func (c *staticCallCollectorRoute) ExprStaticCall(n *ast.ExprStaticCall) {
	c.calls = append(c.calls, n)
}

// ---------------------------------------------------------------------------
// NamespaceName
// ---------------------------------------------------------------------------

// TestNamespaceNameReturnsNamespace verifies NamespaceName extracts the
// declared namespace from a file that has one.
func TestNamespaceNameReturnsNamespace(t *testing.T) {
	root := parseRoot(t, routeSnippet)
	got := phpast.NamespaceName(root)
	const want = `App\Http\Controllers`
	if got != want {
		t.Errorf("NamespaceName = %q, want %q", got, want)
	}
}

// TestNamespaceNameNoNamespaceReturnsEmpty verifies NamespaceName returns ""
// for a file that has no namespace declaration.
func TestNamespaceNameNoNamespaceReturnsEmpty(t *testing.T) {
	root := parseRoot(t, `<?php
class Foo {}
`)
	got := phpast.NamespaceName(root)
	if got != "" {
		t.Errorf("NamespaceName (no namespace) = %q, want \"\"", got)
	}
}

// TestNamespaceNameWrongNodeReturnsEmpty verifies NamespaceName returns ""
// when given a non-Root node.
func TestNamespaceNameWrongNodeReturnsEmpty(t *testing.T) {
	got := phpast.NamespaceName(&ast.Identifier{Value: []byte("x")})
	if got != "" {
		t.Errorf("NamespaceName(non-root) = %q, want \"\"", got)
	}
}

// TestNamespaceNameNilReturnsEmpty verifies NamespaceName returns "" for nil.
func TestNamespaceNameNilReturnsEmpty(t *testing.T) {
	got := phpast.NamespaceName(nil)
	if got != "" {
		t.Errorf("NamespaceName(nil) = %q, want \"\"", got)
	}
}

// ---------------------------------------------------------------------------
// UseImports
// ---------------------------------------------------------------------------

// TestUseImportsBuildsMap verifies UseImports returns a complete short→FQN map
// that includes both plain imports and aliased ones (the `as` form).
func TestUseImportsBuildsMap(t *testing.T) {
	root := parseRoot(t, routeSnippet)
	got := phpast.UseImports(root)

	cases := []struct{ short, fqn string }{
		{"Route", `Illuminate\Support\Facades\Route`},
		{"PostController", `App\Http\Controllers\PostController`},
		// aliased import: `use App\Http\Controllers\UserController as UserCtrl;`
		{"UserCtrl", `App\Http\Controllers\UserController`},
	}
	for _, tc := range cases {
		if v, ok := got[tc.short]; !ok {
			t.Errorf("UseImports: key %q missing; map = %v", tc.short, got)
		} else if v != tc.fqn {
			t.Errorf("UseImports[%q] = %q, want %q", tc.short, v, tc.fqn)
		}
	}
	// The original long name "UserController" must NOT appear as a key (alias wins).
	if _, ok := got["UserController"]; ok {
		t.Errorf("UseImports: alias key overridden; \"UserController\" should not be present when aliased to \"UserCtrl\"")
	}
}

// TestUseImportsNoImportsReturnsEmptyMap verifies UseImports returns a non-nil
// empty map when the file has no use imports.
func TestUseImportsNoImportsReturnsEmptyMap(t *testing.T) {
	root := parseRoot(t, `<?php
namespace Foo;
class Bar {}
`)
	got := phpast.UseImports(root)
	if got == nil {
		t.Fatal("UseImports(no imports) = nil, want empty non-nil map")
	}
	if len(got) != 0 {
		t.Errorf("UseImports(no imports) = %v, want empty map", got)
	}
}

// TestUseImportsWrongNodeReturnsEmptyMap verifies UseImports returns a non-nil
// empty map when given a non-Root node.
func TestUseImportsWrongNodeReturnsEmptyMap(t *testing.T) {
	got := phpast.UseImports(&ast.Identifier{Value: []byte("x")})
	if got == nil {
		t.Fatal("UseImports(non-root) = nil, want non-nil empty map")
	}
	if len(got) != 0 {
		t.Errorf("UseImports(non-root) = %v, want empty map", got)
	}
}

// TestUseImportsNilReturnsEmptyMap verifies UseImports returns a non-nil empty
// map for nil input.
func TestUseImportsNilReturnsEmptyMap(t *testing.T) {
	got := phpast.UseImports(nil)
	if got == nil {
		t.Fatal("UseImports(nil) = nil, want non-nil empty map")
	}
}

// TestUseImportsSingleSegmentName verifies UseImports handles a single-segment
// import (no backslash) — the short name equals the full name.
func TestUseImportsSingleSegmentName(t *testing.T) {
	root := parseRoot(t, `<?php
use Closure;
`)
	got := phpast.UseImports(root)
	if v, ok := got["Closure"]; !ok || v != "Closure" {
		t.Errorf("UseImports single-segment: got %v, want {\"Closure\":\"Closure\"}", got)
	}
}

// ---------------------------------------------------------------------------
// ArrayItems
// ---------------------------------------------------------------------------

// TestArrayItemsReturnsTwoItems verifies ArrayItems returns one Vertex per
// array element for a two-item PHP array literal.
func TestArrayItemsReturnsTwoItems(t *testing.T) {
	res, err := phpast.Parse([]byte(arraySnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	ac := &arrayCollector{}
	phpast.Walk(res.Root, ac)

	if len(ac.arrays) == 0 {
		t.Fatal("no ExprArray nodes found in arraySnippet")
	}
	items := phpast.ArrayItems(ac.arrays[0])
	if len(items) != 2 {
		t.Errorf("ArrayItems = %d items, want 2; items = %v", len(items), items)
	}
	// The first item is a class-const fetch (PostController::class).
	if got := phpast.ClassConstClass(items[0]); got != "PostController" {
		t.Errorf("ArrayItems[0] class = %q, want %q", got, "PostController")
	}
	// The second item is a string literal ('index').
	if got := phpast.StringLiteral(items[1]); got != "index" {
		t.Errorf("ArrayItems[1] string = %q, want %q", got, "index")
	}
}

// TestArrayItemsWrongNodeReturnsNil verifies ArrayItems returns nil when given
// a node that is not an ExprArray.
func TestArrayItemsWrongNodeReturnsNil(t *testing.T) {
	got := phpast.ArrayItems(&ast.Identifier{Value: []byte("x")})
	if got != nil {
		t.Errorf("ArrayItems(non-array) = %v, want nil", got)
	}
}

// TestArrayItemsNilReturnsNil verifies ArrayItems returns nil for nil input.
func TestArrayItemsNilReturnsNil(t *testing.T) {
	got := phpast.ArrayItems(nil)
	if got != nil {
		t.Errorf("ArrayItems(nil) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// ClosureStmts
// ---------------------------------------------------------------------------

// TestClosureStmtsReturnsTwoStatements verifies ClosureStmts returns the
// body statements of a closure literal that contains two statements.
func TestClosureStmtsReturnsTwoStatements(t *testing.T) {
	res, err := phpast.Parse([]byte(closureSnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	cc := &closureCollector{}
	phpast.Walk(res.Root, cc)

	if len(cc.closures) == 0 {
		t.Fatal("no ExprClosure found in closureSnippet")
	}
	stmts := phpast.ClosureStmts(cc.closures[0])
	if len(stmts) != 2 {
		t.Errorf("ClosureStmts = %d stmts, want 2", len(stmts))
	}
}

// TestClosureStmtsWrongNodeReturnsNil verifies ClosureStmts returns nil when
// given a node that is not an ExprClosure.
func TestClosureStmtsWrongNodeReturnsNil(t *testing.T) {
	got := phpast.ClosureStmts(&ast.Identifier{Value: []byte("x")})
	if got != nil {
		t.Errorf("ClosureStmts(non-closure) = %v, want nil", got)
	}
}

// TestClosureStmtsNilReturnsNil verifies ClosureStmts returns nil for nil.
func TestClosureStmtsNilReturnsNil(t *testing.T) {
	got := phpast.ClosureStmts(nil)
	if got != nil {
		t.Errorf("ClosureStmts(nil) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// StaticCallParts
// ---------------------------------------------------------------------------

// TestStaticCallPartsDecomposesRouteGet verifies StaticCallParts returns the
// correct class, call, and args for a Route::get(...) node.
func TestStaticCallPartsDecomposesRouteGet(t *testing.T) {
	res, err := phpast.Parse([]byte(routeSnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	sc := &staticCallCollectorRoute{}
	phpast.Walk(res.Root, sc)

	if len(sc.calls) == 0 {
		t.Fatal("no ExprStaticCall nodes found in routeSnippet")
	}

	// The first static call in the snippet is Route::get('/posts', [...]).
	class, call, args, ok := phpast.StaticCallParts(sc.calls[0])
	if !ok {
		t.Fatal("StaticCallParts returned ok=false for a valid ExprStaticCall")
	}
	if got := phpast.IdentifierName(class); got != "Route" {
		t.Errorf("StaticCallParts class = %q, want %q", got, "Route")
	}
	if got := phpast.CallName(call); got != "get" {
		t.Errorf("StaticCallParts call = %q, want %q", got, "get")
	}
	if len(args) == 0 {
		t.Error("StaticCallParts: args is empty, want at least 2 args")
	}
	// First string arg is the route path.
	if got := phpast.NthStringArg(args, 0); got != "/posts" {
		t.Errorf("StaticCallParts NthStringArg(0) = %q, want %q", got, "/posts")
	}
}

// TestStaticCallPartsWrongNodeReturnsFalse verifies StaticCallParts returns
// ok=false and zero values for a non-ExprStaticCall node.
func TestStaticCallPartsWrongNodeReturnsFalse(t *testing.T) {
	class, call, args, ok := phpast.StaticCallParts(&ast.Identifier{Value: []byte("x")})
	if ok {
		t.Error("StaticCallParts(non-static-call) returned ok=true, want false")
	}
	if class != nil || call != nil || args != nil {
		t.Errorf("StaticCallParts(non-static-call) = (%v, %v, %v), want all nil", class, call, args)
	}
}

// TestStaticCallPartsNilReturnsFalse verifies StaticCallParts returns ok=false
// for nil.
func TestStaticCallPartsNilReturnsFalse(t *testing.T) {
	_, _, _, ok := phpast.StaticCallParts(nil)
	if ok {
		t.Error("StaticCallParts(nil) returned ok=true, want false")
	}
}

// ---------------------------------------------------------------------------
// MethodCallParts
// ---------------------------------------------------------------------------

// methodChainSnippet exercises MethodCallParts on a call where the receiver is
// a variable, producing a true ExprMethodCall chain: $router->get('/x', ...).
const methodChainSnippet = `<?php
$router->middleware('auth')->get('/dashboard', 'DashboardController@index');
`

// TestMethodCallPartsDecomposesChainedCall verifies MethodCallParts correctly
// decomposes an ExprMethodCall: the outer node is "get" on $router->middleware,
// and the receiver of "get" is itself a method call ("middleware" on $router).
//
// Note: Route::middleware('auth')->group(...) in routeSnippet produces a
// *static* call (ExprStaticCall) as the receiver of "group", not a method
// call. methodChainSnippet uses a variable receiver ($router) to guarantee a
// true ExprMethodCall chain for testing MethodCallParts.
func TestMethodCallPartsDecomposesChainedCall(t *testing.T) {
	res, err := phpast.Parse([]byte(methodChainSnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	mc := &methodCallCollectorRoute{}
	phpast.Walk(res.Root, mc)

	if len(mc.calls) == 0 {
		t.Fatal("no ExprMethodCall nodes found in methodChainSnippet")
	}

	// The traverser visits only the outermost call ("get"). "middleware" is its
	// receiver. Decompose the outer node then walk inward.
	outerNode := mc.calls[0]
	recv, method, args, ok := phpast.MethodCallParts(outerNode)
	if !ok {
		t.Fatal("MethodCallParts returned ok=false for a valid ExprMethodCall")
	}
	if phpast.CallName(method) != "get" {
		t.Errorf("outer method = %q, want %q", phpast.CallName(method), "get")
	}
	if len(args) == 0 {
		t.Error("outer args is empty")
	}

	// recv should itself be a method call: $router->middleware('auth')
	innerRecv, innerMethod, innerArgs, innerOK := phpast.MethodCallParts(recv)
	if !innerOK {
		t.Fatal("MethodCallParts: recv of outer call is not a method call — expected middleware")
	}
	if phpast.CallName(innerMethod) != "middleware" {
		t.Errorf("inner method = %q, want %q", phpast.CallName(innerMethod), "middleware")
	}
	if got := phpast.NthStringArg(innerArgs, 0); got != "auth" {
		t.Errorf("middleware arg = %q, want %q", got, "auth")
	}
	// innerRecv should be the $router variable.
	if phpast.VariableName(innerRecv) != "router" {
		t.Errorf("innerRecv variable = %q, want %q", phpast.VariableName(innerRecv), "router")
	}
}

// TestMethodCallPartsOnRouteGroupDecomposesGroupCall verifies MethodCallParts
// correctly decomposes the ->group(...) call in routeSnippet, where the
// receiver is a static call (Route::middleware(...)) — not a method call.
// This proves MethodCallParts extracts group/args correctly regardless of recv type.
func TestMethodCallPartsOnRouteGroupDecomposesGroupCall(t *testing.T) {
	res, err := phpast.Parse([]byte(routeSnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	mc := &methodCallCollectorRoute{}
	phpast.Walk(res.Root, mc)

	if len(mc.calls) == 0 {
		t.Fatal("no ExprMethodCall nodes found in routeSnippet")
	}

	foundGroup := false
	for _, node := range mc.calls {
		recv, method, args, ok := phpast.MethodCallParts(node)
		if !ok {
			t.Error("MethodCallParts returned ok=false for a valid ExprMethodCall")
			continue
		}
		if phpast.CallName(method) == "group" {
			foundGroup = true
			// recv is Route::middleware(...) — a static call, not a method call.
			class, scall, scArgs, scOK := phpast.StaticCallParts(recv)
			if !scOK {
				t.Error("group recv is not a static call — expected Route::middleware")
			} else {
				if phpast.IdentifierName(class) != "Route" {
					t.Errorf("group recv class = %q, want Route", phpast.IdentifierName(class))
				}
				if phpast.CallName(scall) != "middleware" {
					t.Errorf("group recv method = %q, want middleware", phpast.CallName(scall))
				}
				if got := phpast.NthStringArg(scArgs, 0); got != "auth" {
					t.Errorf("middleware arg = %q, want auth", got)
				}
			}
			// The group closure is arg 0.
			closureExpr := phpast.ArgExpr(args, 0)
			if stmts := phpast.ClosureStmts(closureExpr); len(stmts) == 0 {
				t.Error("group closure body is empty")
			}
		}
	}
	if !foundGroup {
		t.Error("MethodCallParts: did not find a 'group' method call in routeSnippet")
	}
}

// TestMethodCallPartsWrongNodeReturnsFalse verifies MethodCallParts returns
// ok=false and zero values for a non-ExprMethodCall node.
func TestMethodCallPartsWrongNodeReturnsFalse(t *testing.T) {
	recv, method, args, ok := phpast.MethodCallParts(&ast.Identifier{Value: []byte("x")})
	if ok {
		t.Error("MethodCallParts(non-method-call) returned ok=true, want false")
	}
	if recv != nil || method != nil || args != nil {
		t.Errorf("MethodCallParts(non-method-call) = (%v, %v, %v), want all nil", recv, method, args)
	}
}

// TestMethodCallPartsNilReturnsFalse verifies MethodCallParts returns ok=false
// for nil.
func TestMethodCallPartsNilReturnsFalse(t *testing.T) {
	_, _, _, ok := phpast.MethodCallParts(nil)
	if ok {
		t.Error("MethodCallParts(nil) returned ok=true, want false")
	}
}

// ---------------------------------------------------------------------------
// ArgExpr
// ---------------------------------------------------------------------------

// TestArgExprReturnsExpressionAtIndex verifies ArgExpr returns the inner
// expression of an *ast.Argument at the given index.
func TestArgExprReturnsExpressionAtIndex(t *testing.T) {
	res, err := phpast.Parse([]byte(routeSnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	sc := &staticCallCollectorRoute{}
	phpast.Walk(res.Root, sc)

	if len(sc.calls) == 0 {
		t.Fatal("no ExprStaticCall found")
	}
	_, _, args, _ := phpast.StaticCallParts(sc.calls[0])
	// The second argument of Route::get('/posts', [...]) is an array callable.
	expr := phpast.ArgExpr(args, 1)
	if expr == nil {
		t.Fatal("ArgExpr(args, 1) = nil, want array expression")
	}
	// The array should have 2 items.
	items := phpast.ArrayItems(expr)
	if len(items) != 2 {
		t.Errorf("ArgExpr(args,1) array items = %d, want 2", len(items))
	}
}

// TestArgExprOutOfRangeReturnsNil verifies ArgExpr returns nil for out-of-range
// and negative indices.
func TestArgExprOutOfRangeReturnsNil(t *testing.T) {
	empty := []phpast.Vertex{}
	if got := phpast.ArgExpr(empty, 0); got != nil {
		t.Errorf("ArgExpr(empty, 0) = %v, want nil", got)
	}
	if got := phpast.ArgExpr(empty, -1); got != nil {
		t.Errorf("ArgExpr(empty, -1) = %v, want nil", got)
	}
}

// TestArgExprWrongNodeReturnsNil verifies ArgExpr returns nil when the entry
// at index n is not an *ast.Argument.
func TestArgExprWrongNodeReturnsNil(t *testing.T) {
	nonArg := []phpast.Vertex{&ast.Identifier{Value: []byte("x")}}
	if got := phpast.ArgExpr(nonArg, 0); got != nil {
		t.Errorf("ArgExpr(non-argument, 0) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// StringLiteral
// ---------------------------------------------------------------------------

// TestStringLiteralStripsQuotes verifies StringLiteral returns the unquoted
// value of a bare ScalarString node (both single and double quoted forms).
func TestStringLiteralStripsQuotes(t *testing.T) {
	res, err := phpast.Parse([]byte(stringLiteralSnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	ss := &scalarStringCollector{}
	phpast.Walk(res.Root, ss)

	if len(ss.scalars) < 2 {
		t.Fatalf("expected at least 2 ScalarString nodes, got %d", len(ss.scalars))
	}
	// 'hello' → hello
	if got := phpast.StringLiteral(ss.scalars[0]); got != "hello" {
		t.Errorf("StringLiteral('hello') = %q, want %q", got, "hello")
	}
	// "world" → world
	if got := phpast.StringLiteral(ss.scalars[1]); got != "world" {
		t.Errorf("StringLiteral(\"world\") = %q, want %q", got, "world")
	}
}

// TestStringLiteralWrongNodeReturnsEmpty verifies StringLiteral returns ""
// for a non-ScalarString node.
func TestStringLiteralWrongNodeReturnsEmpty(t *testing.T) {
	if got := phpast.StringLiteral(&ast.Identifier{Value: []byte("x")}); got != "" {
		t.Errorf("StringLiteral(non-scalar) = %q, want \"\"", got)
	}
}

// TestStringLiteralNilReturnsEmpty verifies StringLiteral returns "" for nil.
func TestStringLiteralNilReturnsEmpty(t *testing.T) {
	if got := phpast.StringLiteral(nil); got != "" {
		t.Errorf("StringLiteral(nil) = %q, want \"\"", got)
	}
}

// ---------------------------------------------------------------------------
// RootStmts
// ---------------------------------------------------------------------------

// TestRootStmtsReturnsTopLevelStatements verifies RootStmts returns the
// top-level statements of a parsed file (at minimum: namespace, use imports,
// and expression statements).
func TestRootStmtsReturnsTopLevelStatements(t *testing.T) {
	root := parseRoot(t, routeSnippet)
	stmts := phpast.RootStmts(root)
	// routeSnippet has: 1 namespace + 3 use stmts + 1 plain route call + 1 chained call = 6
	if len(stmts) == 0 {
		t.Error("RootStmts returned empty slice for non-empty file")
	}
}

// TestRootStmtsWrongNodeReturnsNil verifies RootStmts returns nil for a
// non-Root node.
func TestRootStmtsWrongNodeReturnsNil(t *testing.T) {
	if got := phpast.RootStmts(&ast.Identifier{Value: []byte("x")}); got != nil {
		t.Errorf("RootStmts(non-root) = %v, want nil", got)
	}
}

// TestRootStmtsNilReturnsNil verifies RootStmts returns nil for nil.
func TestRootStmtsNilReturnsNil(t *testing.T) {
	if got := phpast.RootStmts(nil); got != nil {
		t.Errorf("RootStmts(nil) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// ExpressionStmt
// ---------------------------------------------------------------------------

// TestExpressionStmtUnwrapsStaticCall verifies ExpressionStmt returns the
// inner expression of a StmtExpression wrapping a static call, and that the
// result can be fed to StaticCallParts.
func TestExpressionStmtUnwrapsStaticCall(t *testing.T) {
	res, err := phpast.Parse([]byte(routeSnippet))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	sec := &stmtExprCollector{}
	phpast.Walk(res.Root, sec)

	if len(sec.stmtExprs) == 0 {
		t.Fatal("no StmtExpression nodes found in routeSnippet")
	}

	// The first StmtExpression in routeSnippet wraps Route::get('/posts', ...).
	expr := phpast.ExpressionStmt(sec.stmtExprs[0])
	if expr == nil {
		t.Fatal("ExpressionStmt returned nil for a valid StmtExpression")
	}
	// The unwrapped node should be decomposable as a static call.
	_, _, _, ok := phpast.StaticCallParts(expr)
	if !ok {
		t.Error("ExpressionStmt result is not a StaticCall — unwrapping failed")
	}
}

// TestExpressionStmtWrongNodeReturnsNil verifies ExpressionStmt returns nil
// for a non-StmtExpression node.
func TestExpressionStmtWrongNodeReturnsNil(t *testing.T) {
	if got := phpast.ExpressionStmt(&ast.Identifier{Value: []byte("x")}); got != nil {
		t.Errorf("ExpressionStmt(non-stmt) = %v, want nil", got)
	}
}

// TestExpressionStmtNilReturnsNil verifies ExpressionStmt returns nil for nil.
func TestExpressionStmtNilReturnsNil(t *testing.T) {
	if got := phpast.ExpressionStmt(nil); got != nil {
		t.Errorf("ExpressionStmt(nil) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// Integration: RootStmts + ExpressionStmt + StaticCallParts / MethodCallParts
// ---------------------------------------------------------------------------

// TestManualWalkExtractsRouteGetWithArrayCallable verifies that a caller can
// manually iterate RootStmts, unwrap each via ExpressionStmt, decompose via
// StaticCallParts or MethodCallParts, and read the array-callable action —
// exactly the pattern the route extractor uses.
func TestManualWalkExtractsRouteGetWithArrayCallable(t *testing.T) {
	root := parseRoot(t, routeSnippet)
	stmts := phpast.RootStmts(root)

	type routeEntry struct {
		method     string
		path       string
		controller string
		action     string
	}
	var routes []routeEntry

	var extractRoutes func(stmts []phpast.Vertex)
	extractRoutes = func(stmts []phpast.Vertex) {
		for _, st := range stmts {
			expr := phpast.ExpressionStmt(st)
			if expr == nil {
				continue
			}
			// Try static call: Route::get(path, action)
			if class, call, args, ok := phpast.StaticCallParts(expr); ok {
				if phpast.IdentifierName(class) == "Route" {
					path := phpast.NthStringArg(args, 0)
					actionExpr := phpast.ArgExpr(args, 1)
					items := phpast.ArrayItems(actionExpr)
					if len(items) == 2 {
						routes = append(routes, routeEntry{
							method:     phpast.CallName(call),
							path:       path,
							controller: phpast.ClassConstClass(items[0]),
							action:     phpast.StringLiteral(items[1]),
						})
					}
				}
				continue
			}
			// Try method call chain: walk inward to find the static call base.
			node := expr
			for {
				recv, method, args, ok := phpast.MethodCallParts(node)
				if !ok {
					break
				}
				if phpast.CallName(method) == "group" {
					closureExpr := phpast.ArgExpr(args, 0)
					extractRoutes(phpast.ClosureStmts(closureExpr))
				}
				node = recv
			}
		}
	}

	extractRoutes(stmts)

	if len(routes) != 2 {
		t.Fatalf("manual walk extracted %d routes, want 2; got %+v", len(routes), routes)
	}
	// First route: GET /posts → PostController::index
	r0 := routes[0]
	if r0.method != "get" || r0.path != "/posts" || r0.controller != "PostController" || r0.action != "index" {
		t.Errorf("routes[0] = %+v, want {get /posts PostController index}", r0)
	}
	// Second route: GET /users → UserCtrl::list (inside the group closure)
	r1 := routes[1]
	if r1.method != "get" || r1.path != "/users" || r1.controller != "UserCtrl" || r1.action != "list" {
		t.Errorf("routes[1] = %+v, want {get /users UserCtrl list}", r1)
	}
}
