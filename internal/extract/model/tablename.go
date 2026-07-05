// Package model is the Eloquent Model Extractor: it turns a Laravel project's
// app/Models/*.php files into the project model's Model and Relationship nodes
// (ADR 0001, CONTEXT.md glossary). Like the schema Extractor it is deep and
// static — it reads real model AST via internal/phpast rather than booting
// Laravel (ADR 0003) — and produces only in-memory model values.
//
// This file holds the table-name inference helper, a pure function with no
// dependency on the AST layer so it can be reasoned about and tested in
// isolation.
package model

import "strings"

// irregularPlurals maps lowercase singular class names whose English plural is
// not produced by the regular suffix rules below to their correct plural form.
// Laravel's real inflector (Doctrine Inflector) knows hundreds of these; this
// map is deliberately a SHORT, best-effort subset covering the cases most
// likely to appear as model names. See TableName's doc comment for the known
// limitation this implies.
var irregularPlurals = map[string]string{
	"person": "people",
	"child":  "children",
}

// uncountables is a small, best-effort set of lowercase words whose plural is
// the word itself (issue #36: RestaurantStaff must infer restaurant_staff,
// not restaurant_staffs). Deliberately bounded, like irregularPlurals.
var uncountables = map[string]struct{}{
	"staff":     {},
	"sheep":     {},
	"fish":      {},
	"deer":      {},
	"series":    {},
	"species":   {},
	"feedback":  {},
	"equipment": {},
}

// singularSEndings are trailing letter pairs for which a final "s" belongs to
// a genuinely singular word (status, bus, class, analysis) rather than an
// existing plural, so the sibilant "-es" rule still applies. Any other word
// ending in "s" is treated as already plural and kept as-is (issue #36:
// WaiterCalls must infer waiter_calls, not waiter_callses).
var singularSEndings = []string{"ss", "us", "is"}

// TableName infers the database table a Laravel Eloquent model maps to from its
// class name, following Laravel's default convention: the snake_case plural of
// the class name. It is only consulted when a model declares no explicit
// `protected $table` property — an explicit table always wins and is handled by
// the caller, not here.
//
// The transformation is: snake_case the class name (insert "_" before each
// interior uppercase letter, then lowercase everything), then pluralize the
// final word.
//
// Examples:
//
//	User      -> users
//	BlogPost  -> blog_posts
//	Category  -> categories
//	Company   -> companies
//	Post      -> posts
//	Tag       -> tags
//	Person    -> people     (irregular)
//	Child     -> children   (irregular)
//
// Known limitation: pluralization here is best-effort English built from a
// short irregular map plus three suffix rules (-y→-ies, sibilant→-es,
// otherwise -s). Laravel uses a far fuller inflector (Doctrine Inflector), so
// uncommon irregular nouns (e.g. "Sheep", "Datum", "Cactus") may be pluralized
// incorrectly. Such mismatches surface downstream as Disagreement findings
// rather than silent errors; a model with an explicit `$table` sidesteps this
// path entirely.
func TableName(className string) string {
	return pluralize(snakeCase(className))
}

// snakeCase converts a PascalCase/camelCase identifier to snake_case by
// inserting an underscore before each interior uppercase letter and lowercasing
// the whole string. A leading uppercase letter produces no leading underscore.
//
//	User     -> user
//	BlogPost -> blog_post
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && isUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(toLower(r))
	}
	return b.String()
}

// pluralize returns the best-effort English plural of a snake_cased identifier.
// Only the final underscore-delimited word is pluralized; any preceding words
// are a compound prefix that stays singular (e.g. "blog_post" -> "blog_posts").
func pluralize(s string) string {
	prefix, word := splitLastWord(s)
	return prefix + pluralizeWord(word)
}

// splitLastWord separates a snake_cased identifier into everything up to and
// including the final "_" (the prefix, possibly empty) and the final word.
func splitLastWord(s string) (prefix, word string) {
	if i := strings.LastIndexByte(s, '_'); i >= 0 {
		return s[:i+1], s[i+1:]
	}
	return "", s
}

// pluralizeWord applies the irregular map and suffix rules to a single
// lowercase English word. Rule order: irregular map first, then uncountables
// and already-plural stems (kept as-is), then "consonant + y" -> "ies", then
// sibilant endings (s/x/z/ch/sh) -> "es", otherwise append "s".
func pluralizeWord(word string) string {
	if word == "" {
		return word
	}
	if plural, ok := irregularPlurals[word]; ok {
		return plural
	}
	if _, ok := uncountables[word]; ok {
		return word
	}
	if isAlreadyPlural(word) {
		return word
	}
	if endsInConsonantY(word) {
		return word[:len(word)-1] + "ies"
	}
	if endsInSibilant(word) {
		return word + "es"
	}
	return word + "s"
}

// isAlreadyPlural reports whether a word ending in "s" is best read as an
// existing plural (calls, logs) rather than a singular sibilant. Words whose
// trailing "s" follows a singularSEndings pair (status, class, analysis) are
// singular and fall through to the "-es" rule. Best-effort like the rest of
// the inflector: rare singulars such as "lens" are misread as plural, and the
// mismatch surfaces downstream as a Disagreement.
func isAlreadyPlural(word string) bool {
	if !strings.HasSuffix(word, "s") || len(word) < 2 {
		return false
	}
	for _, ending := range singularSEndings {
		if strings.HasSuffix(word, ending) {
			return false
		}
	}
	return true
}

// endsInConsonantY reports whether word ends in "y" preceded by a consonant
// (e.g. "category", "company") — the case that pluralizes to "ies". A vowel
// before the "y" (e.g. "day") keeps the regular "s".
func endsInConsonantY(word string) bool {
	if !strings.HasSuffix(word, "y") || len(word) < 2 {
		return false
	}
	return !isVowel(rune(word[len(word)-2]))
}

// endsInSibilant reports whether word ends in a sibilant cluster (s, x, z, ch,
// sh) — the case that pluralizes to "es" (e.g. "box" -> "boxes",
// "dish" -> "dishes").
func endsInSibilant(word string) bool {
	for _, suffix := range []string{"ch", "sh", "s", "x", "z"} {
		if strings.HasSuffix(word, suffix) {
			return true
		}
	}
	return false
}

// isUpper reports whether r is an ASCII uppercase letter.
func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

// toLower lowercases an ASCII uppercase letter, leaving any other rune
// unchanged.
func toLower(r rune) rune {
	if isUpper(r) {
		return r + ('a' - 'A')
	}
	return r
}

// isVowel reports whether r is an ASCII vowel (a, e, i, o, u), used to decide
// between the "-ies" and "-s" pluralization of words ending in "y".
func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}
