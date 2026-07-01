// Package symbol implements the two-phase symbol table of ADR 0006. It is the
// correctness core of cross-file reference resolution: Phase 1 collects, from
// every parsed file, the classes it declares (by fully-qualified name) and its
// `use`-import map; Phase 2 resolves a short class name — always in the context
// of the file that named it — to an FQN and reports whether that FQN is a class
// the project actually declares.
//
// The table is deliberately pure name→FQN bookkeeping. It carries no domain
// payload: a caller that needs the Controller (and its action list) behind a
// resolved FQN keeps its own FQN→Controller map alongside this table. Keeping
// the two concerns separate makes the table reusable for any cross-file edge
// (Route→Controller today; Model→Model, Route→FormRequest later) without
// coupling it to one node type.
//
// Anti-wrong-edge invariant (ADR 0006): resolution ALWAYS takes the naming
// file's path. A short name such as "PostController" resolves through that
// file's own `use` imports, so two classes sharing a short name in different
// namespaces never collapse into one wrong edge. There is no by-short-name-alone
// resolution path in this API, by design.
//
// This package imports no parser: it reads declarations and imports through the
// phpast helpers (DeclaredClasses, NamespaceName, UseImports) alone.
package symbol

import (
	"strings"
	"sync"

	"github.com/mawsis/unlaravel/internal/phpast"
)

// namespaceSep is PHP's namespace separator, joining a namespace to a short
// class name to form a fully-qualified name.
const namespaceSep = `\`

// Table is the ADR 0006 two-phase symbol table. The zero value is not ready for
// use; construct one with New. All methods are safe for concurrent use, so
// Phase-1 collection may run across goroutines (ADR 0006: Phase 1 can be
// concurrent, but must complete before Phase 2 resolves).
type Table struct {
	mu sync.RWMutex

	// declared is the set of fully-qualified names of every class declared
	// across all added files. Membership is the "is this a real class?" test
	// that turns an unresolved reference into a dangling (dead) edge.
	declared map[string]struct{}

	// importsByFile maps a file path to that file's `use` import map
	// (short name → FQN). Phase-2 resolution reads the naming file's entry
	// here, never a global short-name index — that is the anti-wrong-edge
	// invariant.
	importsByFile map[string]map[string]string
}

// New returns an empty Table ready for Phase-1 collection.
func New() *Table {
	return &Table{
		declared:      make(map[string]struct{}),
		importsByFile: make(map[string]map[string]string),
	}
}

// Add records one file's contribution to the table (Phase 1). It stores every
// class the file declares — each by its FQN, namespace joined to short name —
// into the project-wide declared set, and stores the file's `use` import map
// under filePath for later resolution.
//
// filePath is the key used by the Phase-2 resolvers to look up this file's
// imports; pass the same path the caller will pass to Resolve when resolving a
// reference that originates in this file. root is the file's AST root as
// returned by phpast.Parse / phpast.ParseFile.
//
// Add is idempotent per file: re-adding the same filePath replaces its import
// map, and declaring an already-declared FQN is a no-op. Passing a nil or
// non-Root vertex records an empty import map and no declarations (phpast's
// helpers tolerate it), so a file that failed to parse usefully simply
// contributes nothing rather than panicking.
//
// Safe for concurrent use.
func (t *Table) Add(filePath string, root phpast.Vertex) {
	namespace := phpast.NamespaceName(root)
	classes := phpast.DeclaredClasses(root)
	imports := phpast.UseImports(root)

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, short := range classes {
		t.declared[qualify(namespace, short)] = struct{}{}
	}
	t.importsByFile[filePath] = imports
}

// Resolve resolves a short class name to an FQN using the imports of fromFile —
// the file in which the reference appears — then reports whether that FQN names
// a class declared anywhere in the project (Phase 2).
//
// Resolution never guesses a namespace: if fromFile does not import shortName,
// the short name is returned unchanged as the FQN and found reflects whether
// that bare name happens to be a declared class. A caller that wants Laravel's
// default-namespace fallback (e.g. App\Http\Controllers\ for an unimported
// controller) uses ResolveWithDefault instead, keeping the default-namespace
// policy in the caller rather than baked into the table.
//
// A shortName that is already fully qualified (contains a backslash) is treated
// as an FQN directly and looked up as-is; the import map is only consulted for
// bare short names.
//
// found == false is the dangling-edge / dead-route signal of ADR 0006: the
// reference resolved to an FQN, but no such class exists in the project.
//
// Safe for concurrent use.
func (t *Table) Resolve(shortName, fromFile string) (fqn string, found bool) {
	fqn = t.resolveName(shortName, fromFile)
	return fqn, t.IsDeclared(fqn)
}

// ResolveWithDefault behaves like Resolve, but when fromFile does not import
// shortName (and shortName is not already fully qualified), it qualifies the
// short name with the caller-supplied defaultNamespace before looking it up —
// modelling a framework convention such as Laravel's default controller
// namespace App\Http\Controllers.
//
// The default is applied ONLY as a fallback: an explicit `use` import in
// fromFile always wins, and an already-fully-qualified name is used verbatim.
// An empty defaultNamespace makes this identical to Resolve. The default-vs-
// import precedence lives here so the table has no hard-coded knowledge of any
// framework's namespace layout.
//
// Safe for concurrent use.
func (t *Table) ResolveWithDefault(shortName, fromFile, defaultNamespace string) (fqn string, found bool) {
	fqn = t.resolveName(shortName, fromFile)
	// resolveName only leaves a bare (unqualified, unimported) name untouched;
	// apply the caller's default namespace to exactly that case.
	if defaultNamespace != "" && fqn == shortName && !isQualified(shortName) {
		fqn = qualify(defaultNamespace, shortName)
	}
	return fqn, t.IsDeclared(fqn)
}

// IsDeclared reports whether fqn names a class declared by some added file. It
// is the raw membership test underlying the resolvers, exposed for callers that
// already hold an FQN (e.g. to confirm a resolved edge's target still exists).
//
// Safe for concurrent use.
func (t *Table) IsDeclared(fqn string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.declared[fqn]
	return ok
}

// resolveName maps a short (or already-qualified) name to an FQN using
// fromFile's import map, applying no default namespace. An already-qualified
// name is returned unchanged; a bare name is looked up in the file's imports and
// returned unchanged if not imported. It reads shared state, so it is only
// called by the exported methods, which hold the read lock via IsDeclared or
// take it here.
func (t *Table) resolveName(shortName, fromFile string) string {
	if isQualified(shortName) {
		return shortName
	}

	t.mu.RLock()
	imports := t.importsByFile[fromFile]
	t.mu.RUnlock()

	if fqn, ok := imports[shortName]; ok {
		return fqn
	}
	return shortName
}

// qualify joins a namespace to a short class name with PHP's separator. An empty
// namespace (the global namespace) yields the short name unchanged, never a
// leading backslash.
func qualify(namespace, short string) string {
	if namespace == "" {
		return short
	}
	return namespace + namespaceSep + short
}

// isQualified reports whether a name already carries a namespace (contains a
// backslash), in which case it is an FQN and must not be resolved through an
// import map.
func isQualified(name string) bool {
	return strings.Contains(name, namespaceSep)
}
