package phpast_test

import (
	"reflect"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
	"github.com/VKCOM/php-parser/pkg/ast"
)

// arrayStringSnippet exercises ArrayStringItems on a realistic $fillable
// declaration: a list-style array of plain string literals.
const arrayStringItemsSnippet = `<?php
$a = ['title', 'body'];
`

// arrayStringItemsMixedSnippet exercises the skip-non-string-literal path: a
// variable and a class-const fetch sit alongside a plain string literal.
const arrayStringItemsMixedSnippet = `<?php
$a = [$foo, Bar::class, 'kept'];
`

// arrayStringPairsSnippet exercises ArrayStringPairs on a realistic $casts
// declaration: a single key => string-value pair.
const arrayStringPairsSnippet = `<?php
$a = ['email_verified_at' => 'datetime'];
`

// arrayStringPairsMixedSnippet exercises the skip-non-string-value path: one
// pair's value is a variable (not a string literal) and must be skipped while
// a following valid pair is kept.
const arrayStringPairsMixedSnippet = `<?php
$a = ['bad' => $foo, 'good' => 'datetime'];
`

// emptyArraySnippet is a bare empty array literal, used to check both helpers'
// zero-item behaviour.
const emptyArraySnippet = `<?php
$a = [];
`

// TestArrayStringItemsHappyPath verifies ArrayStringItems reduces a list-style
// string array to a []string in source order.
func TestArrayStringItemsHappyPath(t *testing.T) {
	ac := &arrayCollector{}
	phpast.Walk(parseRoot(t, arrayStringItemsSnippet), ac)
	if len(ac.arrays) == 0 {
		t.Fatal("no ExprArray nodes found in arrayStringItemsSnippet")
	}

	got := phpast.ArrayStringItems(ac.arrays[0])
	want := []string{"title", "body"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArrayStringItems = %v, want %v", got, want)
	}
}

// TestArrayStringItemsSkipsNonStringLiterals verifies ArrayStringItems skips
// items that are not plain string literals (a variable, a class-const fetch)
// without panicking, keeping only the plain string literal.
func TestArrayStringItemsSkipsNonStringLiterals(t *testing.T) {
	ac := &arrayCollector{}
	phpast.Walk(parseRoot(t, arrayStringItemsMixedSnippet), ac)
	if len(ac.arrays) == 0 {
		t.Fatal("no ExprArray nodes found in arrayStringItemsMixedSnippet")
	}

	got := phpast.ArrayStringItems(ac.arrays[0])
	want := []string{"kept"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArrayStringItems (mixed) = %v, want %v", got, want)
	}
}

// TestArrayStringItemsEmptyArray verifies ArrayStringItems returns an empty,
// non-nil slice for an array literal with zero items, matching ArrayItems'
// own convention for the empty-array case.
func TestArrayStringItemsEmptyArray(t *testing.T) {
	ac := &arrayCollector{}
	phpast.Walk(parseRoot(t, emptyArraySnippet), ac)
	if len(ac.arrays) == 0 {
		t.Fatal("no ExprArray nodes found in emptyArraySnippet")
	}

	got := phpast.ArrayStringItems(ac.arrays[0])
	if got == nil {
		t.Fatal("ArrayStringItems(empty array) = nil, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("ArrayStringItems(empty array) = %v, want empty slice", got)
	}
}

// TestArrayStringItemsWrongNodeReturnsNil verifies ArrayStringItems returns
// nil when given a node that is not an ExprArray, and for nil input.
func TestArrayStringItemsWrongNodeReturnsNil(t *testing.T) {
	if got := phpast.ArrayStringItems(&ast.Identifier{Value: []byte("x")}); got != nil {
		t.Errorf("ArrayStringItems(non-array) = %v, want nil", got)
	}
	if got := phpast.ArrayStringItems(nil); got != nil {
		t.Errorf("ArrayStringItems(nil) = %v, want nil", got)
	}
}

// TestArrayStringPairsHappyPath verifies ArrayStringPairs reduces a keyed
// string-value array to a []StringPair.
func TestArrayStringPairsHappyPath(t *testing.T) {
	ac := &arrayCollector{}
	phpast.Walk(parseRoot(t, arrayStringPairsSnippet), ac)
	if len(ac.arrays) == 0 {
		t.Fatal("no ExprArray nodes found in arrayStringPairsSnippet")
	}

	got := phpast.ArrayStringPairs(ac.arrays[0])
	want := []phpast.StringPair{{Key: "email_verified_at", Value: "datetime"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArrayStringPairs = %v, want %v", got, want)
	}
}

// TestArrayStringPairsSkipsNonStringValues verifies ArrayStringPairs skips a
// pair whose value is not a string literal (e.g. a variable) without
// panicking, while preserving order among the pairs that are kept.
func TestArrayStringPairsSkipsNonStringValues(t *testing.T) {
	ac := &arrayCollector{}
	phpast.Walk(parseRoot(t, arrayStringPairsMixedSnippet), ac)
	if len(ac.arrays) == 0 {
		t.Fatal("no ExprArray nodes found in arrayStringPairsMixedSnippet")
	}

	got := phpast.ArrayStringPairs(ac.arrays[0])
	want := []phpast.StringPair{{Key: "good", Value: "datetime"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArrayStringPairs (mixed) = %v, want %v", got, want)
	}
}

// TestArrayStringPairsEmptyArray verifies ArrayStringPairs returns an empty,
// non-nil slice for an array literal with zero items, matching ArrayPairs'
// own convention for the empty-array case.
func TestArrayStringPairsEmptyArray(t *testing.T) {
	ac := &arrayCollector{}
	phpast.Walk(parseRoot(t, emptyArraySnippet), ac)
	if len(ac.arrays) == 0 {
		t.Fatal("no ExprArray nodes found in emptyArraySnippet")
	}

	got := phpast.ArrayStringPairs(ac.arrays[0])
	if got == nil {
		t.Fatal("ArrayStringPairs(empty array) = nil, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("ArrayStringPairs(empty array) = %v, want empty slice", got)
	}
}

// TestArrayStringPairsWrongNodeReturnsNil verifies ArrayStringPairs returns
// nil when given a node that is not an ExprArray, and for nil input.
func TestArrayStringPairsWrongNodeReturnsNil(t *testing.T) {
	if got := phpast.ArrayStringPairs(&ast.Identifier{Value: []byte("x")}); got != nil {
		t.Errorf("ArrayStringPairs(non-array) = %v, want nil", got)
	}
	if got := phpast.ArrayStringPairs(nil); got != nil {
		t.Errorf("ArrayStringPairs(nil) = %v, want nil", got)
	}
}
