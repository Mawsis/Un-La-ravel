package baseline

import (
	"encoding/json"
	"fmt"
	"sort"
)

// CurrentVersion is the schema version of the baseline file format. It is
// independent of model.CurrentSchemaVersion: the baseline is a NEW committed
// artifact (issue #49), not part of ProjectModel, so its shape evolves on its
// own timeline. Parse refuses a file whose major version it does not recognize,
// so a newer, incompatibly-shaped baseline fails loudly on an older binary
// rather than being read as a partial (and silently wrong) suppression set.
//
// The format is intentionally minimal — a version and a sorted list of opaque
// fingerprint strings — because the fingerprint scheme (baseline.Fingerprint) is
// the real contract; the file is just its committed, diff-friendly container.
const CurrentVersion = "1.0"

// baselineFile is the on-disk JSON shape of a Baseline: a version and the sorted
// fingerprint entries. Struct field order is the emitted-JSON order (version
// first, then entries), matching the model package's determinism convention.
type baselineFile struct {
	// Version is the baseline format version, CurrentVersion at write time.
	Version string `json:"version"`
	// Entries are the suppressed fingerprints, ALWAYS sorted so the file is
	// byte-deterministic and diffs cleanly under version control. A non-nil empty
	// slice serializes as [] (an empty baseline), never null.
	Entries []string `json:"entries"`
}

// Marshal renders a Baseline as indented, deterministic JSON suitable for
// committing: the current version and the fingerprint entries sorted
// lexicographically. The same set of fingerprints always yields identical bytes,
// regardless of the order the items were added — the sort is what guarantees it,
// since the underlying set is an (unordered) map.
func Marshal(b *Baseline) ([]byte, error) {
	entries := make([]string, 0, len(b.fingerprints))
	for fp := range b.fingerprints {
		entries = append(entries, fp)
	}
	sort.Strings(entries)

	// Indent so a committed baseline is human-reviewable in a diff; a trailing
	// newline is added by the file writer, not here.
	data, err := json.MarshalIndent(baselineFile{
		Version: CurrentVersion,
		Entries: entries,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal baseline: %w", err)
	}
	return data, nil
}

// Parse reads a committed baseline file back into a Baseline, rejecting a file
// whose major version this binary does not understand. An unknown version is an
// error — not a silently-empty baseline — because a baseline the tool cannot
// fully honor would fail the CI gate on findings the user believes are
// suppressed; failing loudly tells them to upgrade instead.
func Parse(data []byte) (*Baseline, error) {
	var file baselineFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	if !compatibleVersion(file.Version) {
		return nil, fmt.Errorf("parse baseline: unsupported version %q (this binary understands %q)", file.Version, CurrentVersion)
	}

	b := &Baseline{fingerprints: make(map[string]struct{}, len(file.Entries))}
	for _, fp := range file.Entries {
		b.fingerprints[fp] = struct{}{}
	}
	return b, nil
}

// compatibleVersion reports whether a file's version can be read by this binary.
// The rule is major-version equality: within a major version the format only
// adds, so an older-or-equal minor is readable, but a different major (or an
// unparseable/empty version) is not. Kept as its own function so the policy is
// stated in one place if the format later needs a range check.
func compatibleVersion(v string) bool {
	return major(v) == major(CurrentVersion) && v != ""
}

// major returns the major-version segment of a "major.minor" string, or the
// whole string if there is no dot. Empty in, empty out.
func major(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			return v[:i]
		}
	}
	return v
}
