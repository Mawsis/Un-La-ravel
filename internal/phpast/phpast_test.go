package phpast_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/VKCOM/php-parser/pkg/ast"
)

// migrationSnippet is a minimal modern-Laravel migration. It is intentionally
// small but real: a Schema::create('users', ...) static call wrapping a closure
// whose $table->...() method calls declare columns. Parsing it exercises the
// node shapes the helpers must read.
const migrationSnippet = `<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration {
    public function up(): void
    {
        Schema::create('users', function (Blueprint $table) {
            $table->id();
            $table->string('name')->nullable();
        });
    }
};
`

// TestParseSucceedsWithoutSyntaxErrors verifies Parse returns a usable Root and
// no recoverable diagnostics for valid PHP, and reports no catastrophic error.
func TestParseSucceedsWithoutSyntaxErrors(t *testing.T) {
	res, err := phpast.Parse([]byte(migrationSnippet))
	if err != nil {
		t.Fatalf("Parse() returned catastrophic error: %v", err)
	}
	if res.Root == nil {
		t.Fatal("Parse() returned a nil Root for valid PHP")
	}
	if len(res.SyntaxErrors) != 0 {
		t.Errorf("Parse() reported %d syntax errors for valid PHP, want 0: %v",
			len(res.SyntaxErrors), res.SyntaxErrors)
	}
}

// staticCallCollector is a partial Visitor built per the documented pattern:
// embed phpast.NullVisitor and override only the node methods of interest. It
// captures, for every Schema::create-style call, the class name, the call name,
// and the first string argument.
type staticCallCollector struct {
	phpast.NullVisitor
	classNames []string
	callNames  []string
	firstArgs  []string
}

func (c *staticCallCollector) ExprStaticCall(n *ast.ExprStaticCall) {
	c.classNames = append(c.classNames, phpast.IdentifierName(n.Class))
	c.callNames = append(c.callNames, phpast.CallName(n.Call))
	c.firstArgs = append(c.firstArgs, phpast.FirstStringArg(n.Args))
}

// methodCallCollector captures the method names of every chained $table->x()
// call so we can prove the helpers read method identifiers and the walker
// visits inner nodes of chained calls.
type methodCallCollector struct {
	phpast.NullVisitor
	methodNames []string
}

func (c *methodCallCollector) ExprMethodCall(n *ast.ExprMethodCall) {
	c.methodNames = append(c.methodNames, phpast.CallName(n.Method))
}

// TestWalkExtractsStaticCallHelpers parses the snippet, walks it with a visitor,
// and asserts the wrapper's helpers extract the expected values from the
// Schema::create('users', ...) call: class "Schema", method "create", first
// string argument "users" (quotes stripped).
func TestWalkExtractsStaticCallHelpers(t *testing.T) {
	res, err := phpast.Parse([]byte(migrationSnippet))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	c := &staticCallCollector{}
	phpast.Walk(res.Root, c)

	if len(c.callNames) != 1 {
		t.Fatalf("found %d static calls, want 1: %+v", len(c.callNames), c.callNames)
	}
	if c.classNames[0] != "Schema" {
		t.Errorf("IdentifierName(class) = %q, want %q", c.classNames[0], "Schema")
	}
	if c.callNames[0] != "create" {
		t.Errorf("CallName(call) = %q, want %q", c.callNames[0], "create")
	}
	if c.firstArgs[0] != "users" {
		t.Errorf("FirstStringArg = %q, want %q (quotes must be stripped)", c.firstArgs[0], "users")
	}
}

// TestWalkVisitsChainedMethodCalls verifies Walk reaches the inner column
// definition of a chained call ($table->string('name')->nullable()): both the
// outer "nullable" and inner "string" method calls are visited, and CallName
// reads each method identifier.
func TestWalkVisitsChainedMethodCalls(t *testing.T) {
	res, err := phpast.Parse([]byte(migrationSnippet))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	c := &methodCallCollector{}
	phpast.Walk(res.Root, c)

	got := map[string]bool{}
	for _, m := range c.methodNames {
		got[m] = true
	}
	for _, want := range []string{"id", "string", "nullable"} {
		if !got[want] {
			t.Errorf("method call %q was not visited; got methods %v", want, c.methodNames)
		}
	}
}

// TestFirstStringArgIgnoresNonStringArgs verifies FirstStringArg skips
// non-string arguments and strips quotes from the first string literal, even
// when it is not the first argument.
func TestFirstStringArgIgnoresNonStringArgs(t *testing.T) {
	src := `<?php Schema::foo(123, true, 'wanted', 'second');`

	res, err := phpast.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	c := &staticCallCollector{}
	phpast.Walk(res.Root, c)

	if len(c.firstArgs) != 1 {
		t.Fatalf("found %d static calls, want 1", len(c.firstArgs))
	}
	if c.firstArgs[0] != "wanted" {
		t.Errorf("FirstStringArg = %q, want %q", c.firstArgs[0], "wanted")
	}
}

// TestNamePartsReturnsNamespaceSegments verifies NameParts returns the
// individual segments of a namespaced *ast.Name, and IdentifierName joins them
// with a backslash.
func TestNamePartsReturnsNamespaceSegments(t *testing.T) {
	src := `<?php Illuminate\Support\Facades\Schema::create('users');`

	res, err := phpast.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	var (
		gotParts  []string
		gotJoined string
	)
	c := &namedClassCollector{onClass: func(class phpast.Vertex) {
		gotParts = phpast.NameParts(class)
		gotJoined = phpast.IdentifierName(class)
	}}
	phpast.Walk(res.Root, c)

	wantParts := []string{"Illuminate", "Support", "Facades", "Schema"}
	if len(gotParts) != len(wantParts) {
		t.Fatalf("NameParts = %v, want %v", gotParts, wantParts)
	}
	for i := range wantParts {
		if gotParts[i] != wantParts[i] {
			t.Errorf("NameParts[%d] = %q, want %q", i, gotParts[i], wantParts[i])
		}
	}
	if gotJoined != `Illuminate\Support\Facades\Schema` {
		t.Errorf("IdentifierName = %q, want %q", gotJoined, `Illuminate\Support\Facades\Schema`)
	}
}

type namedClassCollector struct {
	phpast.NullVisitor
	onClass func(class phpast.Vertex)
}

func (c *namedClassCollector) ExprStaticCall(n *ast.ExprStaticCall) {
	c.onClass(n.Class)
}

// TestHelpersReturnEmptyForWrongNodeTypes verifies the reader helpers degrade
// gracefully: given a node that is not the expected shape, they return the
// documented zero value ("" or nil) rather than panicking.
func TestHelpersReturnEmptyForWrongNodeTypes(t *testing.T) {
	res, err := phpast.Parse([]byte(`<?php $x = 1;`))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// The Root node is not an identifier, name, or call target.
	if got := phpast.IdentifierName(res.Root); got != "" {
		t.Errorf("IdentifierName(Root) = %q, want empty string", got)
	}
	if got := phpast.CallName(res.Root); got != "" {
		t.Errorf("CallName(Root) = %q, want empty string", got)
	}
	if got := phpast.NameParts(res.Root); got != nil {
		t.Errorf("NameParts(Root) = %v, want nil", got)
	}
	if got := phpast.FirstStringArg(nil); got != "" {
		t.Errorf("FirstStringArg(nil) = %q, want empty string", got)
	}
}

// TestParseFileReadsAndParses verifies ParseFile reads a real file from disk and
// parses it through the same path as Parse.
func TestParseFileReadsAndParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "migration.php")
	if err := os.WriteFile(path, []byte(migrationSnippet), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	res, err := phpast.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}
	if res.Root == nil {
		t.Fatal("ParseFile() returned a nil Root")
	}

	c := &staticCallCollector{}
	phpast.Walk(res.Root, c)
	if len(c.firstArgs) != 1 || c.firstArgs[0] != "users" {
		t.Errorf("ParseFile() + Walk extracted %v, want first arg \"users\"", c.firstArgs)
	}
}

// TestParseFileMissingFileErrors verifies ParseFile returns a wrapped error for
// a nonexistent path (the read failure is surfaced as an error, not swallowed).
func TestParseFileMissingFileErrors(t *testing.T) {
	_, err := phpast.ParseFile(filepath.Join(t.TempDir(), "does-not-exist.php"))
	if err == nil {
		t.Fatal("ParseFile() on a missing file returned nil error, want an error")
	}
}

// classMethodCollector captures every StmtClassMethod so a test can feed each
// method node to phpast.ParamTypeNames.
type classMethodCollector struct {
	phpast.NullVisitor
	methods []*ast.StmtClassMethod
}

func (c *classMethodCollector) StmtClassMethod(n *ast.StmtClassMethod) {
	c.methods = append(c.methods, n)
}

// paramTypesSnippet declares one method per ParamTypeNames case: an unqualified
// FormRequest hint, a qualified hint joined with backslashes, a typed hint
// followed by an untyped route-model param (the untyped one omitted), and a
// parameter-less method (nil result).
const paramTypesSnippet = `<?php

namespace App\Http\Controllers;

use App\Http\Requests\StorePostRequest;

class PostController
{
    public function store(StorePostRequest $request) {}
    public function admin(\App\Http\Requests\AdminRequest $request) {}
    public function update(StorePostRequest $request, $id) {}
    public function index() {}
}
`

// TestParamTypeNames proves ParamTypeNames reads a method's typed parameters in
// order, joins a qualified hint with backslashes, skips untyped parameters, and
// yields nil for a parameter-less method and for a non-method node.
func TestParamTypeNames(t *testing.T) {
	res, err := phpast.Parse([]byte(paramTypesSnippet))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	c := &classMethodCollector{}
	phpast.Walk(res.Root, c)

	if len(c.methods) != 4 {
		t.Fatalf("found %d methods, want 4", len(c.methods))
	}

	cases := []struct {
		method string
		want   []string
	}{
		{"store", []string{"StorePostRequest"}},
		{"admin", []string{`App\Http\Requests\AdminRequest`}},
		{"update", []string{"StorePostRequest"}}, // untyped $id omitted
		{"index", nil},                           // no parameters
	}
	for i, tc := range cases {
		got := phpast.ParamTypeNames(c.methods[i])
		if !equalStrings(got, tc.want) {
			t.Errorf("ParamTypeNames(%s) = %v, want %v", tc.method, got, tc.want)
		}
	}

	// A non-StmtClassMethod node yields nil rather than panicking.
	if got := phpast.ParamTypeNames(res.Root); got != nil {
		t.Errorf("ParamTypeNames(non-method) = %v, want nil", got)
	}
}

// equalStrings reports whether two string slices are element-wise equal,
// treating nil and empty as equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
