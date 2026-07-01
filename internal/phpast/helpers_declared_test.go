package phpast_test

import (
	"reflect"
	"testing"

	"github.com/mawsis/unlaravel/internal/phpast"
)

// parseRoot parses PHP source and returns the AST root, failing on a
// catastrophic parse error.
func parseRoot(t *testing.T, src string) phpast.Vertex {
	t.Helper()
	res, err := phpast.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return res.Root
}

func TestDeclaredClassesNonBracketedNamespace(t *testing.T) {
	root := parseRoot(t, `<?php
namespace App\Http\Controllers;
class PostController {}
class UserController {}
`)
	got := phpast.DeclaredClasses(root)
	want := []string{"PostController", "UserController"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeclaredClasses = %v, want %v", got, want)
	}
}

func TestDeclaredClassesBracketedNamespace(t *testing.T) {
	root := parseRoot(t, `<?php
namespace App\Http\Controllers {
    class PostController {}
}
`)
	got := phpast.DeclaredClasses(root)
	want := []string{"PostController"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeclaredClasses (bracketed) = %v, want %v", got, want)
	}
}

func TestDeclaredClassesGlobalNamespace(t *testing.T) {
	root := parseRoot(t, `<?php
class LegacyController {}
`)
	got := phpast.DeclaredClasses(root)
	want := []string{"LegacyController"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeclaredClasses (global) = %v, want %v", got, want)
	}
}

func TestDeclaredClassesWrongNodeIsNil(t *testing.T) {
	if got := phpast.DeclaredClasses(nil); got != nil {
		t.Fatalf("DeclaredClasses(nil) = %v, want nil", got)
	}
}
