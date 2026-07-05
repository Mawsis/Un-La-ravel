package baseline

import (
	"sort"

	"github.com/Mawsis/Un-La-ravel/internal/findings"
)

// Baseline is a committed set of finding fingerprints to suppress: the findings
// a legacy project already knows about and has chosen not to gate CI on. It is
// an in-memory set keyed by fingerprint; the on-disk JSON form (with a version
// and sorted entries) is produced by Marshal and read by Parse in file.go.
//
// The zero value is not usable — construct with New (from live items) or Parse
// (from a committed file).
type Baseline struct {
	// fingerprints is the suppression set. A map gives O(1) membership for
	// Subtract; iteration order is never relied on — the file form sorts on the
	// way out, so determinism does not depend on this map.
	fingerprints map[string]struct{}
}

// New builds a Baseline from a slice of live findings — the operation behind
// "write the current findings to a baseline file" (a team adopting the gate on a
// legacy project). Duplicate items collapse to one entry, since a set cannot
// hold the same fingerprint twice.
func New(items []findings.Item) *Baseline {
	b := &Baseline{fingerprints: make(map[string]struct{}, len(items))}
	for _, it := range items {
		b.fingerprints[Fingerprint(it)] = struct{}{}
	}
	return b
}

// has reports whether fp is in the suppression set.
func (b *Baseline) has(fp string) bool {
	_, ok := b.fingerprints[fp]
	return ok
}

// Subtract splits items into survivors (not baselined) and suppressed
// (baselined), and reports stale baseline fingerprints — entries that matched no
// item in this run. Order within survivors and suppressed follows items, so the
// output stays as deterministic as its input; stale is returned sorted so its
// order does not depend on map iteration.
//
// Stale entries are RETURNED, not dropped: a baselined finding that has since
// been fixed should be visible (so the baseline can be pruned) rather than
// silently forgotten — see the issue's "reported, not silently dropped".
func Subtract(items []findings.Item, b *Baseline) (survivors, suppressed []findings.Item, stale []string) {
	survivors = make([]findings.Item, 0, len(items))
	suppressed = make([]findings.Item, 0)

	// Track which baseline fingerprints we actually matched, to find the stale
	// remainder afterward.
	matched := make(map[string]struct{}, len(b.fingerprints))
	for _, it := range items {
		fp := Fingerprint(it)
		if b.has(fp) {
			suppressed = append(suppressed, it)
			matched[fp] = struct{}{}
		} else {
			survivors = append(survivors, it)
		}
	}

	stale = staleFingerprints(b, matched)
	return survivors, suppressed, stale
}

// staleFingerprints returns the baseline fingerprints not present in matched,
// sorted for deterministic output. A stale entry is one the baseline suppresses
// but no current finding produces — typically a finding that was fixed.
func staleFingerprints(b *Baseline, matched map[string]struct{}) []string {
	stale := make([]string, 0)
	for fp := range b.fingerprints {
		if _, ok := matched[fp]; !ok {
			stale = append(stale, fp)
		}
	}
	sort.Strings(stale)
	return stale
}
