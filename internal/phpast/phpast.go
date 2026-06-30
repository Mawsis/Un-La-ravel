// Package phpast is the single, isolated wrapper around the VKCOM/php-parser
// library. Per ADR 0003, every Extractor depends on PHP AST parsing, and
// swapping the underlying parser later is a meaningful cost. To contain that
// cost, this is the ONLY package in the codebase permitted to import
// github.com/VKCOM/php-parser. Extractors (e.g. internal/extract/schema) parse
// and walk source exclusively through the types and helpers exported here.
package phpast

import (
	"fmt"
	"os"

	"github.com/VKCOM/php-parser/pkg/ast"
	"github.com/VKCOM/php-parser/pkg/conf"
	"github.com/VKCOM/php-parser/pkg/errors"
	"github.com/VKCOM/php-parser/pkg/parser"
	"github.com/VKCOM/php-parser/pkg/version"
	"github.com/VKCOM/php-parser/pkg/visitor/traverser"
)

// maxPHPVersion is the highest PHP language version the parser supports
// (php8RangeEnd). Passing 8.2+ is rejected by the parser; 8.1 is the ceiling.
var maxPHPVersion = version.Version{Major: 8, Minor: 1}

// Vertex aliases the parser's node interface so callers never name the
// underlying library type directly. Re-exporting it here keeps the parser
// import confined to this package.
type Vertex = ast.Vertex

// Visitor aliases the parser's per-node-type visitor interface. Extractors
// build visitors by embedding NullVisitor and overriding the methods they care
// about (see the package's helper docs).
type Visitor = ast.Visitor

// SyntaxError aliases a single recoverable parse diagnostic. The parser is
// fault-tolerant: it reports recoverable issues here rather than failing the
// whole parse, so callers can choose to analyse partially-valid files.
type SyntaxError = errors.Error

// ParseResult is the outcome of parsing a single PHP source buffer. Root is the
// AST root suitable for Walk; SyntaxErrors holds any recoverable diagnostics
// gathered during parsing (it may be non-empty even on a usable Root).
type ParseResult struct {
	Root         Vertex
	SyntaxErrors []*SyntaxError
}

// Parse parses a PHP source buffer into an AST. It returns an error only on
// catastrophic, unrecoverable failure. Recoverable syntax issues are collected
// into ParseResult.SyntaxErrors and do NOT produce a non-nil error, because the
// parser tolerates syntax-invalid files and still yields a usable tree.
//
// The PHP language version is pinned to the parser's maximum (8.1).
func Parse(src []byte) (ParseResult, error) {
	var syntaxErrs []*SyntaxError
	ver := maxPHPVersion // copy: never hand the parser a pointer to package state
	root, err := parser.Parse(src, conf.Config{
		Version: &ver,
		ErrorHandlerFunc: func(e *errors.Error) {
			syntaxErrs = append(syntaxErrs, e)
		},
	})
	if err != nil {
		return ParseResult{}, fmt.Errorf("phpast: parse failed: %w", err)
	}
	return ParseResult{Root: root, SyntaxErrors: syntaxErrs}, nil
}

// ParseFile reads the file at path and parses it via Parse. Read errors and any
// catastrophic parse error are wrapped with the path for context. Recoverable
// syntax issues are surfaced in ParseResult.SyntaxErrors, not as an error.
func ParseFile(path string) (ParseResult, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return ParseResult{}, fmt.Errorf("phpast: read %q: %w", path, err)
	}
	res, err := Parse(src)
	if err != nil {
		return ParseResult{}, fmt.Errorf("phpast: parse %q: %w", path, err)
	}
	return res, nil
}

// Walk traverses the AST rooted at root, invoking the per-node-type methods of
// visitor on every node in source order. This is the single point in the
// codebase where the traverser is used, so the parser's traversal API stays
// confined here.
func Walk(root Vertex, visitor Visitor) {
	traverser.NewTraverser(visitor).Traverse(root)
}
