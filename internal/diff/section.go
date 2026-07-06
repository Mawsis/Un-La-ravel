package diff

import "reflect"

// pair is one entry that exists in both models under the same identity key but
// whose value differs — the shape every section's "changed" entry wraps.
type pair[T any] struct {
	Old T
	New T
}

// diffSection is the one algorithm every section shares: given the old and new
// slices and a function that maps an entry to its identity key, it returns what
// was added (in new order), removed (in old order), and changed (in old order),
// with equality decided by reflect.DeepEqual so slice-bearing entries (routes,
// tables, models) compare by value.
//
// The pairing is MULTISET-AWARE: a key may legitimately appear more than once in
// a single input (Laravel permits duplicate route registrations at the same
// method+URI, and the extractor emits each verbatim — see internal/extract/route,
// which has no dedup pass). Within each key group the pairing is EXACT-MATCH
// FIRST: an old occurrence that is byte-for-byte equal to some unclaimed new
// occurrence is paired with it and reported as unchanged, so a stable duplicate
// never masquerades as a change. Only after those exact matches are removed does
// what remains get paired positionally — the leftover old occurrences against
// the leftover new ones, in order — and any surplus becomes a removal (surplus
// old) or an addition (surplus new). So old [A, B] vs new [B] reports A removed
// and B unchanged, never a bogus "A→B changed". For a unique key this reduces to
// the obvious one-to-one comparison.
//
// It never iterates a map into a result slice — the per-key groups are built by
// walking the source-ordered inputs, and the added/removed/changed slices are
// emitted while ranging over those same source-ordered slices — so callers
// inherit the determinism convention for free. Returned slices are non-nil.
//
// This is the deep-module seam: five sections that used to repeat the pairing
// dance now state only their identity function and their wrapper types.
func diffSection[T any](old, new []T, key func(T) string) (added, removed []T, changed []pair[T]) {
	added, removed, changed = []T{}, []T{}, []pair[T]{}

	newByKey := groupByKey(new, key)

	// claimed[k] marks which occurrences of key k (by their index in newByKey[k])
	// have already been paired with an old entry, so no new occurrence is claimed
	// twice and the unclaimed ones are exactly the additions.
	claimed := make(map[string][]bool, len(newByKey))
	for k, group := range newByKey {
		claimed[k] = make([]bool, len(group))
	}

	// Pass 1 — exact matches: walk old in source order and claim, for each old
	// entry, an unclaimed new occurrence under the same key that is DeepEqual to
	// it. Running this to completion BEFORE any positional pairing is what makes
	// old [A, B] vs new [B] resolve to "B unchanged, A removed" rather than
	// letting A greedily claim B. oldClaimed marks the old entries handled here so
	// pass 2 skips them. Exact matches are unchanged, so nothing is emitted.
	oldClaimed := make([]bool, len(old))
	for oi, o := range old {
		k := key(o)
		if i := claimExact(newByKey[k], claimed[k], o); i >= 0 {
			claimed[k][i] = true
			oldClaimed[oi] = true
		}
	}

	// Pass 2 — positional pairing + removals: walk the remaining old entries in
	// source order. Claim the next unclaimed new occurrence under the same key (a
	// change); if none remains, the old occurrence was removed.
	for oi, o := range old {
		if oldClaimed[oi] {
			continue
		}
		k := key(o)
		if i := claimNext(claimed[k]); i >= 0 {
			claimed[k][i] = true
			changed = append(changed, pair[T]{Old: o, New: newByKey[k][i]})
			continue
		}
		removed = append(removed, o)
	}

	// Added: walk new in source order, emitting the occurrences no old entry
	// claimed. A per-key running index maps each new entry back to its position in
	// its key group so its claimed flag can be read. Walking new (not the map)
	// keeps additions in source order.
	seen := make(map[string]int, len(newByKey))
	for _, n := range new {
		k := key(n)
		idx := seen[k]
		seen[k] = idx + 1
		if !claimed[k][idx] {
			added = append(added, n)
		}
	}

	return added, removed, changed
}

// claimExact returns the index of the first unclaimed entry in group that is
// DeepEqual to want, or -1 if there is none. It lets an unchanged duplicate pair
// with its identical counterpart before positional pairing runs, so a stable
// duplicate is never reported as a change.
func claimExact[T any](group []T, claimed []bool, want T) int {
	for i := range group {
		if !claimed[i] && reflect.DeepEqual(group[i], want) {
			return i
		}
	}
	return -1
}

// claimNext returns the index of the first unclaimed entry, or -1 if all are
// claimed. It supplies the positional pairing for leftovers after exact matches.
func claimNext(claimed []bool) int {
	for i := range claimed {
		if !claimed[i] {
			return i
		}
	}
	return -1
}

// groupByKey buckets entries by their identity key, preserving each key's
// occurrence order (source order within the input slice). It is a lookup
// structure only — no result slice is ever built by iterating it — so it does
// not compromise determinism.
func groupByKey[T any](items []T, key func(T) string) map[string][]T {
	groups := make(map[string][]T, len(items))
	for _, it := range items {
		k := key(it)
		groups[k] = append(groups[k], it)
	}
	return groups
}
