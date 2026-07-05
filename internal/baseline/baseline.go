// Package baseline suppresses already-known findings so a legacy Laravel project
// can adopt the doctor CI gate without fixing its entire history first (issue
// #49, parent #46). It fingerprints each per-item finding (findings.Item) into a
// stable, content-derived string and subtracts a committed set of those
// fingerprints from a live findings list, returning what survived, what was
// suppressed, and which baseline entries no longer match anything (stale).
//
// Fingerprints are positional-independent: they read a finding's identifying
// content (a dead route's method + URI + controller; an unguarded model's class;
// a disagreement's model + relationship), never its position in a slice or a
// source line number. So a refactor that reorders routes or moves code between
// files does not spuriously re-trigger a suppressed finding.
//
// The package imports internal/findings (for Item) and internal/model (for the
// kind constants) — nothing imports it back, so no cycle.
package baseline

import (
	"strings"

	"github.com/Mawsis/Un-La-ravel/internal/findings"
	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// fpSep separates the parts of a fingerprint. It is a byte that cannot appear in
// an HTTP method, a URI, a PHP class name, or a relationship method name, so no
// combination of field values can collide by straddling the separator (for
// example "a" + "b|c" can never equal "a|b" + "c").
const fpSep = "\x1f" // ASCII Unit Separator

// Fingerprint reduces a per-item finding to a stable, content-derived string
// that identifies THAT finding across reorderings and line moves. It reads only
// the identifying fields defined for the item's Kind:
//
//   - dead route   → kind, method, URI, controller
//   - unguarded    → kind, class
//   - disagreement → kind, model, relationship
//
// The Kind is always the first part, so two findings of different kinds can
// never share a fingerprint even if their remaining fields coincide. An unknown
// kind fingerprints on Kind alone — enough to be stable and distinct from known
// kinds, without inventing a field layout the caller did not define.
func Fingerprint(item findings.Item) string {
	switch item.Kind {
	case model.FindingDeadRoutes:
		return join(item.Kind, item.Method, item.URI, item.Controller)
	case model.FindingUnguarded:
		return join(item.Kind, item.Class)
	case model.FindingDisagreements:
		return join(item.Kind, item.Model, item.Relationship)
	default:
		return join(item.Kind)
	}
}

// join concatenates fingerprint parts with fpSep. Kept tiny and separate so the
// per-kind cases above read as a table of which fields each kind contributes.
func join(parts ...string) string {
	return strings.Join(parts, fpSep)
}
