package phpast_test

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/VKCOM/php-parser/pkg/ast"
)

// modelSnippet is a minimal Eloquent model: a class with an explicit $table
// property and a relationship method whose body holds a $this->belongsTo(...)
// call mixing a Class::class target and a string foreign-key argument. Parsing
// it exercises the node shapes the model-extractor helpers read: ExprVariable
// (the $this / $table), ExprClassConstFetch (User::class), and positional
// string args ('author_id').
const modelSnippet = `<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Post extends Model {
    protected $table = 'blog_posts';

    public function author()
    {
        return $this->belongsTo(User::class, 'author_id');
    }
}
`

// relCollector captures, through the phpast helpers only, what the model
// extractor needs from a relationship call: the receiver variable name (proving
// the leading "$" is stripped), the property variable name, the first argument
// resolved as a class-const target, and the second argument resolved as a
// positional string. It never type-switches on a parser node itself.
type relCollector struct {
	phpast.NullVisitor
	receiverNames []string // from ExprMethodCall .Var (e.g. "this")
	propertyNames []string // from StmtProperty .Var (e.g. "table")
	classTargets  []string // NthArgClassConst(args, 0) (e.g. "User")
	stringTargets []string // NthStringArg(args, 1) (e.g. "author_id")
}

func (c *relCollector) ExprMethodCall(n *ast.ExprMethodCall) {
	c.receiverNames = append(c.receiverNames, phpast.VariableName(n.Var))
	c.classTargets = append(c.classTargets, phpast.NthArgClassConst(n.Args, 0))
	c.stringTargets = append(c.stringTargets, phpast.NthStringArg(n.Args, 1))
}

func (c *relCollector) StmtProperty(n *ast.StmtProperty) {
	c.propertyNames = append(c.propertyNames, phpast.VariableName(n.Var))
}

// TestModelHelpersReadRelationshipShapes proves the model-extractor helpers
// read a real relationship call correctly: $this is stripped to "this",
// $table to "table", User::class resolves to "User", and the positional
// 'author_id' string is read from index 1.
func TestModelHelpersReadRelationshipShapes(t *testing.T) {
	res, err := phpast.Parse([]byte(modelSnippet))
	if err != nil {
		t.Fatalf("Parse() returned catastrophic error: %v", err)
	}

	c := &relCollector{}
	phpast.Walk(res.Root, c)

	if got := firstOr(c.receiverNames, ""); got != "this" {
		t.Errorf("VariableName(receiver) = %q, want %q (leading $ must be stripped)", got, "this")
	}
	if got := firstOr(c.propertyNames, ""); got != "table" {
		t.Errorf("VariableName($table) = %q, want %q", got, "table")
	}
	if got := firstOr(c.classTargets, ""); got != "User" {
		t.Errorf("NthArgClassConst(args, 0) = %q, want %q", got, "User")
	}
	if got := firstOr(c.stringTargets, ""); got != "author_id" {
		t.Errorf("NthStringArg(args, 1) = %q, want %q", got, "author_id")
	}
}

// TestVariableNameEdgeCases covers the non-variable and non-identifier paths
// that the parse-driven test cannot reach: VariableName returns "" for any node
// that is not an *ast.ExprVariable.
func TestVariableNameEdgeCases(t *testing.T) {
	if got := phpast.VariableName(&ast.Identifier{Value: []byte("notavar")}); got != "" {
		t.Errorf("VariableName(non-variable) = %q, want \"\"", got)
	}
	if got := phpast.VariableName(nil); got != "" {
		t.Errorf("VariableName(nil) = %q, want \"\"", got)
	}
}

// TestClassConstClassEdgeCases covers ClassConstClass's "" path for a node that
// is not a class-const fetch.
func TestClassConstClassEdgeCases(t *testing.T) {
	if got := phpast.ClassConstClass(&ast.Identifier{Value: []byte("x")}); got != "" {
		t.Errorf("ClassConstClass(non-classconst) = %q, want \"\"", got)
	}
}

// TestNthArgHelpersOutOfRange covers the boundary guards: a negative index, an
// index past the end, and an argument of the wrong shape all yield "".
func TestNthArgHelpersOutOfRange(t *testing.T) {
	empty := []phpast.Vertex{}
	if got := phpast.NthStringArg(empty, 0); got != "" {
		t.Errorf("NthStringArg(empty, 0) = %q, want \"\"", got)
	}
	if got := phpast.NthStringArg(empty, -1); got != "" {
		t.Errorf("NthStringArg(empty, -1) = %q, want \"\"", got)
	}
	if got := phpast.NthArgClassConst(empty, 0); got != "" {
		t.Errorf("NthArgClassConst(empty, 0) = %q, want \"\"", got)
	}
	if got := phpast.NthArgClassConst(empty, 5); got != "" {
		t.Errorf("NthArgClassConst(empty, 5) = %q, want \"\"", got)
	}
}

// firstOr returns the first element of s, or def when s is empty. Keeps the
// assertions above readable without index-out-of-range panics on a parse that
// unexpectedly found nothing.
func firstOr(s []string, def string) string {
	if len(s) == 0 {
		return def
	}
	return s[0]
}
