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
// the shapes extractors encounter: a bare *ast.Identifier (e.g. a method name
// such as create); an unqualified or namespaced *ast.Name whose parts are joined
// with a backslash (e.g. Illuminate\Support\Facades\Schema); and a
// fully-qualified *ast.NameFullyQualified (a leading-backslash hint such as
// \App\Http\Requests\AdminRequest), whose parts are joined the same way with the
// leading separator dropped so the result matches the codebase's backslash-FQN
// convention (no leading backslash). Any other node type yields "".
func IdentifierName(v Vertex) string {
	switch n := v.(type) {
	case *ast.Name:
		return joinNameParts(n.Parts)
	case *ast.NameFullyQualified:
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

// VariableName returns the name of an *ast.ExprVariable WITHOUT its leading
// "$". Returns "" for any other node type.
//
// GOTCHA (verified against the parser, do not re-learn): ExprVariable.Name is
// an *ast.Identifier whose .Value INCLUDES the dollar sign — "$this" stays
// "$this", "$table" stays "$table". This helper strips that "$" so callers can
// compare against the bare PHP variable name ("this", "table") without ever
// touching the raw, dollar-prefixed form. ALWAYS go through this helper rather
// than reading ExprVariable.Name directly, or the "$" will silently break
// every name comparison.
func VariableName(v Vertex) string {
	ev, ok := v.(*ast.ExprVariable)
	if !ok {
		return ""
	}
	id, ok := ev.Name.(*ast.Identifier)
	if !ok {
		return ""
	}
	return strings.TrimPrefix(string(id.Value), "$")
}

// ClassConstClass returns the class name of an *ast.ExprClassConstFetch — the
// shape of a PHP "Class::const" expression such as User::class. The name is
// resolved from the node's .Class field via IdentifierName, so it handles both
// a bare *ast.Identifier (User) and a namespaced *ast.Name
// (App\Models\User → "App\Models\User"). Returns "" for any other node type.
//
// Note this returns the class part only and ignores which constant is fetched;
// for Eloquent relationship targets the constant is always "class".
func ClassConstClass(v Vertex) string {
	ccf, ok := v.(*ast.ExprClassConstFetch)
	if !ok {
		return ""
	}
	return IdentifierName(ccf.Class)
}

// NthStringArg returns the unquoted value of the argument at index n in an Args
// list when that argument is a string literal (e.g. the 'author_id' in
// $this->belongsTo(User::class, 'author_id') is at n == 1). Unlike
// FirstStringArg, which scans for the first string anywhere, this addresses a
// specific positional slot. Returns "" when n is out of range or the argument
// at n is not a string literal.
func NthStringArg(args []Vertex, n int) string {
	if n < 0 || n >= len(args) {
		return ""
	}
	if s, ok := stringArgValue(args[n]); ok {
		return s
	}
	return ""
}

// NthArgClassConst returns the class name of the argument at index n when that
// argument is a "Class::class" expression (*ast.ExprClassConstFetch), e.g. the
// User in $this->belongsTo(User::class) at n == 0. Returns "" when n is out of
// range, the argument is not an *ast.Argument, or its expression is not a
// class-const fetch. Pair with NthStringArg to resolve a relationship target
// that may be given either as User::class or as the string 'App\Models\User',
// without the caller ever type-switching on a parser node.
func NthArgClassConst(args []Vertex, n int) string {
	if n < 0 || n >= len(args) {
		return ""
	}
	a, ok := args[n].(*ast.Argument)
	if !ok {
		return ""
	}
	return ClassConstClass(a.Expr)
}

// NamespaceName returns the namespace declared by a file, resolved from the
// first *ast.StmtNamespace under the AST root (e.g. "App\Http\Controllers" for
// `namespace App\Http\Controllers;`). It returns "" when root is not an
// *ast.Root or the file declares no namespace (global namespace).
//
// This is Phase-1 input for the symbol table (ADR 0006): a declared class's FQN
// is this namespace joined to the class's short name. Only the first namespace
// statement is honoured, which matches the single-namespace-per-file convention
// Laravel application code follows; bracketed multi-namespace files are out of
// scope.
func NamespaceName(root Vertex) string {
	r, ok := root.(*ast.Root)
	if !ok {
		return ""
	}
	for _, st := range r.Stmts {
		if ns, ok := st.(*ast.StmtNamespace); ok {
			return IdentifierName(ns.Name)
		}
	}
	return ""
}

// UseImports returns a file's `use` import map as short name → fully-qualified
// name, built from every *ast.StmtUse under the top-level *ast.StmtUseList nodes
// (e.g. `use App\Http\Controllers\PostController;` yields
// "PostController" → "App\Http\Controllers\PostController"). When a use has an
// alias (`use App\...\Foo as Bar;`), the alias is the key and the full name the
// value ("Bar" → "App\...\Foo"). Returns an empty, non-nil map when root is not
// an *ast.Root or the file has no imports.
//
// This is the CRITICAL Phase-2 resolver for ADR 0006: a route's short
// controller name is resolved to an FQN through the ROUTE FILE's import map
// (never by short name alone), so two controllers sharing a short name in
// different namespaces never collapse into a wrong edge.
func UseImports(root Vertex) map[string]string {
	imports := make(map[string]string)
	r, ok := root.(*ast.Root)
	if !ok {
		return imports
	}
	for _, st := range r.Stmts {
		list, ok := st.(*ast.StmtUseList)
		if !ok {
			continue
		}
		for _, u := range list.Uses {
			use, ok := u.(*ast.StmtUse)
			if !ok {
				continue
			}
			fqn := IdentifierName(use.Use)
			if fqn == "" {
				continue
			}
			short := IdentifierName(use.Alias)
			if short == "" {
				short = lastNameSegment(fqn)
			}
			imports[short] = fqn
		}
	}
	return imports
}

// ArrayItems returns the value expressions of an *ast.ExprArray's items — the
// .Val of each *ast.ExprArrayItem, in source order (e.g. the two entries of
// [PostController::class, 'index'] as [class-const-fetch, string-literal]). Keys
// are ignored, so this yields values for both list-style and keyed arrays.
// Returns nil when v is not an *ast.ExprArray; an item with a nil value is
// skipped so callers can index the result safely.
//
// Lets the route extractor read an array-callable action without importing the
// parser: ArrayItems(arg)[0] → the controller via ClassConstClass, [1] → the
// method via a following string-arg helper.
func ArrayItems(v Vertex) []Vertex {
	arr, ok := v.(*ast.ExprArray)
	if !ok {
		return nil
	}
	items := make([]Vertex, 0, len(arr.Items))
	for _, it := range arr.Items {
		item, ok := it.(*ast.ExprArrayItem)
		if !ok || item.Val == nil {
			continue
		}
		items = append(items, item.Val)
	}
	return items
}

// ClosureStmts returns the body statements of an *ast.ExprClosure — the .Stmts
// of a `function () { ... }` expression, in source order. Returns nil when v is
// not a closure.
//
// The route extractor uses this to descend into a `->group(function () { ... })`
// body and re-walk the nested route statements with the group's inherited
// prefix and middleware (the group-flattening algorithm, ADR 0006 / ROUTE_FACTS).
func ClosureStmts(v Vertex) []Vertex {
	cl, ok := v.(*ast.ExprClosure)
	if !ok {
		return nil
	}
	return cl.Stmts
}

// StaticCallParts decomposes an *ast.ExprStaticCall into its three parts: the
// class expression (e.g. the "Route" of Route::get, readable via IdentifierName),
// the call target (the "get", readable via CallName), and the argument list
// (each entry an *ast.Argument, consumable by FirstStringArg / NthStringArg /
// NthArgClassConst / ArgExpr). The bool is false — with zero-value returns —
// when v is not an *ast.ExprStaticCall.
//
// This lets the route extractor identify and read facade calls
// (Route::get / Route::apiResource / Route::resource) without importing the
// parser; pair it with ArgExpr to reach an array-callable or closure argument.
func StaticCallParts(v Vertex) (class Vertex, call Vertex, args []Vertex, ok bool) {
	sc, ok := v.(*ast.ExprStaticCall)
	if !ok {
		return nil, nil, nil, false
	}
	return sc.Class, sc.Call, sc.Args, true
}

// MethodCallParts decomposes an *ast.ExprMethodCall into its three parts: the
// receiver expression (.Var — the inner call a chained modifier wraps, walked
// inward to unwind a route or group chain), the method target (readable via
// CallName, e.g. "middleware" / "prefix" / "name" / "group"), and the argument
// list (each an *ast.Argument). The bool is false — with zero-value returns —
// when v is not an *ast.ExprMethodCall.
//
// This drives the chain-walking half of the group-flattening algorithm: on
// method == "group", collect inherited prefix/middleware by recursing through
// .Var, then descend into the closure argument (ClosureStmts) with that context.
func MethodCallParts(v Vertex) (recv Vertex, method Vertex, args []Vertex, ok bool) {
	mc, ok := v.(*ast.ExprMethodCall)
	if !ok {
		return nil, nil, nil, false
	}
	return mc.Var, mc.Method, mc.Args, true
}

// ArgExpr returns the underlying expression of the argument at index n in an
// Args list — the .Expr of the *ast.Argument (e.g. the *ast.ExprArray behind
// the second argument of Route::get('/x', [C::class, 'm'])). Returns nil when n
// is out of range or the entry is not an *ast.Argument.
//
// The string- and class-const-typed accessors (NthStringArg, NthArgClassConst)
// cover scalar arguments; ArgExpr reaches the remaining composite argument
// shapes — array callables (feed to ArrayItems) and group closures (feed to
// ClosureStmts) — so extractors still never type-switch on a parser node.
func ArgExpr(args []Vertex, n int) Vertex {
	if n < 0 || n >= len(args) {
		return nil
	}
	a, ok := args[n].(*ast.Argument)
	if !ok {
		return nil
	}
	return a.Expr
}

// StringLiteral returns the unquoted value of a bare *ast.ScalarString
// expression (e.g. the "index" of the array callable [C::class, 'index'], or an
// element of a ['auth', 'throttle'] middleware list). Returns "" for any other
// node type.
//
// The Nth/First string-arg helpers read a string that is wrapped in an
// *ast.Argument; StringLiteral reaches the string values that appear UNWRAPPED —
// inside an array's items (via ArrayItems) — so the route extractor can read an
// array-callable action or a middleware-array element without a php-parser
// import. Like the argument helpers, it strips the surrounding quote characters
// the parser leaves on ScalarString.Value.
func StringLiteral(v Vertex) string {
	s, ok := v.(*ast.ScalarString)
	if !ok {
		return ""
	}
	return strings.Trim(string(s.Value), quoteChars)
}

// RootStmts returns the top-level statements of a parsed file — the .Stmts of
// the *ast.Root, in source order. Returns nil when root is not an *ast.Root.
//
// The route extractor walks these statements manually rather than with a
// tree-wide traverser: a route's enclosing group context (prefix, middleware)
// flows strictly top-down through the source, so recursion over statements in
// order — descending into each group closure with the inherited context — is the
// natural shape, and keeps that extractor free of any php-parser import.
func RootStmts(root Vertex) []Vertex {
	r, ok := root.(*ast.Root)
	if !ok {
		return nil
	}
	return r.Stmts
}

// ExpressionStmt returns the wrapped expression of an *ast.StmtExpression — the
// .Expr of an expression statement such as `Route::get(...);`. Returns nil when
// v is not an *ast.StmtExpression.
//
// Every top-level route declaration is an expression statement wrapping a
// Route:: static call (or a chained ->group()/->middleware()/->name() on one);
// this unwraps that statement so the route extractor can hand the inner call to
// StaticCallParts / MethodCallParts without naming the parser's statement type.
func ExpressionStmt(v Vertex) Vertex {
	stmt, ok := v.(*ast.StmtExpression)
	if !ok {
		return nil
	}
	return stmt.Expr
}

// DeclaredClasses returns the short names of every class declared in a file, in
// source order (e.g. ["PostController"] for a file declaring `class
// PostController extends Controller { ... }`). Classes nested inside a
// *ast.StmtNamespace are found as well as top-level ones, so the result is
// complete regardless of whether the file uses the `namespace X;` or the
// bracketed `namespace X { ... }` form. Anonymous classes (which have no name)
// and non-class declarations are excluded. Returns nil when root is not an
// *ast.Root.
//
// This is Phase-1 input for the symbol table (ADR 0006): joined to
// NamespaceName(root), each short name yields a declared class's FQN. Reading
// declarations through this helper keeps the symbol table — like every other
// consumer — free of any php-parser import.
func DeclaredClasses(root Vertex) []string {
	r, ok := root.(*ast.Root)
	if !ok {
		return nil
	}
	var names []string
	collectClassNames(r.Stmts, &names)
	return names
}

// collectClassNames appends the short name of each *ast.StmtClass found among
// stmts (descending one level into any *ast.StmtNamespace to reach classes
// declared inside a bracketed namespace block) to out. Unnamed classes are
// skipped.
func collectClassNames(stmts []Vertex, out *[]string) {
	for _, st := range stmts {
		switch n := st.(type) {
		case *ast.StmtClass:
			if name := IdentifierName(n.Name); name != "" {
				*out = append(*out, name)
			}
		case *ast.StmtNamespace:
			collectClassNames(n.Stmts, out)
		}
	}
}

// ParamTypeNames returns the type-hint names of a method's parameters, in
// declaration order, reading them from an *ast.StmtClassMethod's Params. Each
// name is the textual form of the parameter's type node (via IdentifierName):
// an unqualified hint yields its short name (e.g. "StorePostRequest") and a
// qualified hint yields its backslash-joined name (e.g.
// "App\Http\Requests\StorePostRequest"). Callers resolve short names to FQNs
// through the declaring file's `use` imports + the symbol table (ADR 0006).
//
// A parameter with NO type hint (e.g. `$id`) has a nil Type and is SKIPPED, so
// the returned slice contains only the typed parameters — this is precisely how
// a FormRequest-typed action parameter is told apart from a plain route-model or
// scalar parameter. The result is nil when method is not an *ast.StmtClassMethod
// or the method has no typed parameters, so callers can range over it safely.
//
// Nullable (`?Type`), union, and intersection hints are out of scope: only a
// name-shaped type node is read (unqualified/namespaced *ast.Name or
// fully-qualified *ast.NameFullyQualified, both via IdentifierName); any other
// type-node shape yields "" and is skipped, matching the FormRequest link's
// single-class-hint expectation (a controller action takes a FormRequest as a
// plain, non-nullable parameter).
func ParamTypeNames(method Vertex) []string {
	m, ok := method.(*ast.StmtClassMethod)
	if !ok {
		return nil
	}
	var names []string
	for _, p := range m.Params {
		param, ok := p.(*ast.Parameter)
		if !ok || param.Type == nil {
			continue
		}
		if name := IdentifierName(param.Type); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// ClassExtends returns the textual name of the base class an *ast.StmtClass
// extends — the .Extends node read via IdentifierName, so an unqualified parent
// yields its short name (e.g. "FormRequest") and a qualified parent yields its
// backslash-joined name (e.g. "Illuminate\Foundation\Http\FormRequest"). A class
// with no `extends` clause, or a node that is not an *ast.StmtClass, yields "".
//
// The FormRequest extractor uses this to recognise a request class: a class is a
// FormRequest when the last backslash segment of its parent name is "FormRequest"
// (whether written unqualified or fully qualified). Reading the parent through
// this helper keeps that extractor free of any php-parser import (ADR 0003).
func ClassExtends(class Vertex) string {
	c, ok := class.(*ast.StmtClass)
	if !ok || c.Extends == nil {
		return ""
	}
	return IdentifierName(c.Extends)
}

// ArrayPair is one key/value entry of an *ast.ExprArray, preserving BOTH sides
// of the `key => value` mapping. Key is nil for a list-style entry that has no
// explicit key.
type ArrayPair struct {
	Key Vertex
	Val Vertex
}

// ArrayPairs returns the key/value entries of an *ast.ExprArray in source order,
// keeping each *ast.ExprArrayItem's Key alongside its Val. It complements
// ArrayItems (which yields only values): the FormRequest rules() array is keyed
// by field name (`'title' => 'required|string'`), so its keys carry meaning and
// must be read, not discarded.
//
// An item with a nil Val is skipped so callers can read Val safely; a nil Key is
// preserved (a list-style entry). Returns nil when v is not an *ast.ExprArray.
func ArrayPairs(v Vertex) []ArrayPair {
	arr, ok := v.(*ast.ExprArray)
	if !ok {
		return nil
	}
	pairs := make([]ArrayPair, 0, len(arr.Items))
	for _, it := range arr.Items {
		item, ok := it.(*ast.ExprArrayItem)
		if !ok || item.Val == nil {
			continue
		}
		pairs = append(pairs, ArrayPair{Key: item.Key, Val: item.Val})
	}
	return pairs
}

// MethodReturnExpr returns the expression of the FIRST return statement in an
// *ast.StmtClassMethod's body — the .Expr of the first *ast.StmtReturn among the
// method body's top-level statements, in source order. Returns nil when method is
// not an *ast.StmtClassMethod, has no body, or its body has no return statement
// (e.g. an abstract or void method).
//
// The FormRequest extractor uses this to reach a rules() method's returned array
// (`return [ ... ];`) without naming the parser's statement types: the method
// body is an *ast.StmtStmtList whose Stmts are scanned for the return. Only the
// body's own top-level statements are examined — a return nested inside a
// conditional is out of scope, matching the single-`return [...]` shape a rules()
// method conventionally has.
func MethodReturnExpr(method Vertex) Vertex {
	m, ok := method.(*ast.StmtClassMethod)
	if !ok {
		return nil
	}
	body, ok := m.Stmt.(*ast.StmtStmtList)
	if !ok {
		return nil
	}
	for _, st := range body.Stmts {
		if ret, ok := st.(*ast.StmtReturn); ok {
			return ret.Expr
		}
	}
	return nil
}

// ArrayStringItems reduces a PHP array literal's VALUES to a []string, built
// on top of ArrayItems + StringLiteral (e.g. $fillable = ['title', 'body']
// yields ["title", "body"]). Returns nil when v is not an *ast.ExprArray,
// mirroring ArrayItems; for an array literal with zero items it returns the
// same empty-but-non-nil slice ArrayItems itself returns for that case.
//
// KNOWN SIMPLIFICATION: an item is skipped whenever StringLiteral(item) == "",
// which is true both when the item is not a plain string literal at all (a
// variable, a class-const, etc.) AND when it IS a string literal whose value
// is an empty string. These two cases are not distinguished. This is accepted
// because Laravel model code never declares an empty-string entry in
// $fillable or $guarded — it has no meaning there — so collapsing the two
// cases costs nothing in practice while keeping the implementation a direct
// composition of ArrayItems and StringLiteral.
//
// Lets the Eloquent model extractor read $fillable / $guarded without
// importing the parser.
func ArrayStringItems(v Vertex) []string {
	items := ArrayItems(v)
	if items == nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := StringLiteral(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// StringPair is one key/value entry of a PHP array literal whose value is a
// string literal, both sides already unquoted (e.g. Key: "email_verified_at",
// Value: "datetime" for the $casts entry 'email_verified_at' => 'datetime').
type StringPair struct {
	Key   string
	Value string
}

// ArrayStringPairs reduces a PHP array literal's key => string-value pairs to
// a []StringPair, built on top of ArrayPairs + StringLiteral (e.g. $casts =
// ['email_verified_at' => 'datetime'] yields
// [{Key: "email_verified_at", Value: "datetime"}]), preserving source order.
// Returns nil when v is not an *ast.ExprArray, mirroring ArrayPairs.
//
// KNOWN SIMPLIFICATION: a pair is skipped whenever StringLiteral(pair.Val) ==
// "", which — as with ArrayStringItems — conflates "value is not a string
// literal" (e.g. a variable or class-const cast target) with "value is an
// empty string". Accepted for the same reason: Laravel model code never
// declares a $casts entry with an empty-string cast type.
//
// Lets the Eloquent model extractor read $casts without importing the parser.
func ArrayStringPairs(v Vertex) []StringPair {
	pairs := ArrayPairs(v)
	if pairs == nil {
		return nil
	}
	out := make([]StringPair, 0, len(pairs))
	for _, p := range pairs {
		val := StringLiteral(p.Val)
		if val == "" {
			continue
		}
		out = append(out, StringPair{Key: StringLiteral(p.Key), Value: val})
	}
	return out
}

// lastNameSegment returns the final backslash-delimited segment of a
// fully-qualified name (e.g. "PostController" from
// "App\Http\Controllers\PostController"), i.e. its PHP short name. Names with no
// backslash are returned unchanged.
func lastNameSegment(fqn string) string {
	if i := strings.LastIndex(fqn, `\`); i >= 0 {
		return fqn[i+1:]
	}
	return fqn
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
