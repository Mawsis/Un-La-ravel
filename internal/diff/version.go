package diff

import "github.com/Mawsis/Un-La-ravel/internal/model"

// VersionCompatible reports whether two Project Models can be meaningfully
// diffed given their schema_version values (issue #53: "tolerated where fields
// allow, clear error where they do not"). The policy is major-version equality:
// within one major version the contract only ADDS fields backward-compatibly (a
// new field on a section is nil/absent on the older side and simply reads as a
// change), so every field the diff touches aligns; a different major version may
// have reshaped or removed a field, so the comparison could be misleading and
// the CLI should refuse with a clear error instead.
//
// This mirrors baseline.compatibleVersion so the two committed-artifact readers
// agree on what "same enough to compare" means. An empty version on either side
// is never compatible — a model with no stamped version is not a contract this
// tool produced, and silently diffing it would hide that.
//
// Diff itself is tolerant by construction (it only reads shared struct fields
// and never errors), so this check is the CLI's gate: it decides whether to
// proceed or to surface a clear version-mismatch error. It does not change what
// Diff computes.
func VersionCompatible(old, new *model.ProjectModel) bool {
	return old.SchemaVersion != "" && new.SchemaVersion != "" &&
		majorVersion(old.SchemaVersion) == majorVersion(new.SchemaVersion)
}

// majorVersion returns the major segment of a "major.minor.patch" version, or
// the whole string when there is no dot. Empty in, empty out. Kept local to the
// diff package so its version policy is stated here rather than reaching into
// another package's unexported helper.
func majorVersion(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			return v[:i]
		}
	}
	return v
}
