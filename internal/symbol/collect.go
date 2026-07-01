package symbol

import (
	"fmt"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// AddFile parses the PHP file at path and adds its declarations and imports to
// the table (Phase 1). It is the file-oriented counterpart to Add: the caller
// supplies a path rather than a pre-parsed root, and the file is parsed via
// phpast.ParseFile using path as the table key.
//
// A read or catastrophic parse failure is returned wrapped, because a file that
// cannot be parsed would silently omit its declared classes from the table —
// turning real edges into dead ones. Recoverable syntax diagnostics do NOT
// fail: the parser is fault-tolerant (ADR 0003) and a partially-valid file
// still contributes the classes it does declare.
func (t *Table) AddFile(path string) error {
	res, err := phpast.ParseFile(path)
	if err != nil {
		return fmt.Errorf("symbol: add file %q: %w", path, err)
	}
	t.Add(path, res.Root)
	return nil
}

// Collect builds a fresh Table by adding every file in paths, in order (Phase
// 1). It is the straightforward sequential driver for collection; callers that
// need concurrency can construct a Table with New and call Add / AddFile from
// several goroutines, since those methods are safe for concurrent use.
//
// Collection stops at the first file that fails to parse, returning the wrapped
// error so the caller does not resolve edges against an incomplete table.
func Collect(paths []string) (*Table, error) {
	t := New()
	for _, path := range paths {
		if err := t.AddFile(path); err != nil {
			return nil, err
		}
	}
	return t, nil
}
