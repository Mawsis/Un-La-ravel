// Package formrequest extracts FormRequest nodes (ADR 0001) from a Laravel
// project's app/Http/Requests directory. Each PHP file may declare one or more
// classes; one model.FormRequest is emitted per class that extends a FormRequest
// base, carrying its fully qualified name and the request-body Fields parsed from
// its rules() method.
//
// The rules are recorded SOURCE-FAITHFULLY (model.Rule keeps each token's name
// and raw arguments; see model.FormRequest): the mapping from these parsed rules
// to an OpenAPI/JSON-Schema property lives in render/openapi, not here. This
// extractor's job is to turn PHP source into that faithful record.
//
// The FQNs produced here are the keys the two-phase symbol table (ADR 0006)
// resolves a controller action's FormRequest-typed parameter against, so the
// analyze step can link a Route to the request body it validates.
package formrequest

import (
	"fmt"
	"strings"

	domain "github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// formRequestBase is the short name of the Laravel base class every FormRequest
// extends (directly or via the fully-qualified
// Illuminate\Foundation\Http\FormRequest). A class is recognised as a
// FormRequest when the last segment of its parent name equals this.
const formRequestBase = "FormRequest"

// namespaceSeparator is the backslash that joins a file's namespace to a class's
// short name to form its fully qualified name (ADR 0006), for example
// "App\Http\Requests" + "\" + "StorePostRequest".
const namespaceSeparator = `\`

// ruleSeparator splits the string form of a ruleset ('required|string|max:255')
// into its individual rule tokens.
const ruleSeparator = "|"

// ruleArgSeparator splits a single rule token's name from its arguments,
// applied ONCE so an argument that itself contains a colon (e.g. a regex) is
// left intact in the argument portion. "max:255" → "max" + "255".
const ruleArgSeparator = ":"

// ruleArgListSeparator splits a rule's argument portion into individual
// arguments, for example the "a,b,c" of "in:a,b,c" → ["a", "b", "c"].
const ruleArgListSeparator = ","

// Extract parses each PHP file at the given paths and returns the FormRequest
// nodes they declare, as model.FormRequest values in first-discovery order with
// each request's Fields — and each field's Rules — in source-declaration order.
//
// A file may declare several classes; one model.FormRequest is emitted per class
// that extends a FormRequest base (see isFormRequest). Non-FormRequest classes
// are ignored. A FormRequest whose rules() method is absent, returns no array, or
// returns an empty array yields a FormRequest with an empty (non-nil) Fields
// slice.
//
// Each request's FQN is the file's namespace (via phpast.NamespaceName) joined to
// the class short name with a backslash; a class in the global namespace yields
// the bare class name as its FQN. Its Fields are the keys of the rules() array,
// each with the validation Rules parsed from that key's value — a string ruleset
// ('a|b|c', split on '|') or an array ruleset (['a', 'b'], one Rule per string
// element). A rule token is split once on ':' into a name and a comma-separated
// argument list; non-string array elements (closures, Rule objects) are out of
// scope and skipped.
//
// Files are processed in the order given, so callers control discovery order by
// sorting paths. The same class name appearing in two files yields two
// FormRequest values; this slice does no project-wide deduplication (the symbol
// table that correlates by FQN is assembled by the caller, per ADR 0006).
//
// Errors: a file that cannot be read or catastrophically fails to parse aborts
// the whole extraction with a wrapped error, because a silently dropped
// FormRequest would leave a route's request body undocumented. Recoverable
// per-file syntax diagnostics do NOT abort — the parser is fault-tolerant
// (ADR 0003).
func Extract(paths []string) ([]domain.FormRequest, error) {
	var requests []domain.FormRequest

	for _, path := range paths {
		res, err := phpast.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("formrequest: extract %q: %w", path, err)
		}

		namespace := phpast.NamespaceName(res.Root)

		v := newFormRequestVisitor()
		phpast.Walk(res.Root, v)

		for _, cb := range v.classes {
			requests = append(requests, buildFormRequest(namespace, cb))
		}
	}

	return requests, nil
}

// buildFormRequest converts a mutable classBuilder into an immutable
// model.FormRequest. The FQN is formed by joining the file namespace to the class
// short name; each raw rules() key/value pair becomes a model.Field with its
// parsed rules. A pair whose key is not a string scalar (an unexpected shape) is
// skipped, so the resulting Fields carry only real field names.
func buildFormRequest(namespace string, cb *classBuilder) domain.FormRequest {
	fr := domain.NewFormRequest(cb.name, qualify(namespace, cb.name))
	for _, pair := range cb.fields {
		name := phpast.StringLiteral(pair.Key)
		if name == "" {
			continue
		}
		fr.Fields = append(fr.Fields, domain.Field{
			Name:  name,
			Rules: parseRules(pair.Val),
		})
	}
	return fr
}

// parseRules parses a field's rule value into model.Rule tokens in source order.
// The value is either a string ruleset ('required|string|max:255', split on '|')
// or an array ruleset (['required', 'max:255'], one entry per string element).
// Any other shape — a closure or Rule object element, or a non-string/non-array
// value — yields no rules for that field (preserved as an empty rule list rather
// than crashing, per scope). The returned slice is non-nil so a field always
// serializes "rules": [].
func parseRules(val phpast.Vertex) []domain.Rule {
	rules := make([]domain.Rule, 0)

	if s := phpast.StringLiteral(val); s != "" {
		for _, token := range strings.Split(s, ruleSeparator) {
			if r, ok := parseRuleToken(token); ok {
				rules = append(rules, r)
			}
		}
		return rules
	}

	for _, item := range phpast.ArrayItems(val) {
		token := phpast.StringLiteral(item)
		if token == "" {
			// Non-string element (closure, Rule object): out of scope, skipped.
			continue
		}
		if r, ok := parseRuleToken(token); ok {
			rules = append(rules, r)
		}
	}
	return rules
}

// parseRuleToken parses one rule token into a model.Rule. The token is split ONCE
// on ':' into a name and an argument portion; the argument portion is split on
// ',' into the rule's arguments. "max:255" → {Name: "max", Args: ["255"]};
// "in:a,b" → {Name: "in", Args: ["a", "b"]}; "required" → {Name: "required"}.
// Surrounding whitespace on the name is trimmed. A blank token (ok=false) is
// skipped by the caller so an empty ruleset does not produce empty rules.
func parseRuleToken(token string) (domain.Rule, bool) {
	name, args, hasArgs := strings.Cut(token, ruleArgSeparator)
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Rule{}, false
	}
	rule := domain.Rule{Name: name}
	if hasArgs {
		rule.Args = strings.Split(args, ruleArgListSeparator)
	}
	return rule, true
}

// qualify joins a file namespace to a class short name to form a fully qualified
// class name (ADR 0006). A class in the global namespace (empty namespace) yields
// the bare short name, matching how PHP resolves an unqualified top-level class.
func qualify(namespace, shortName string) string {
	if namespace == "" {
		return shortName
	}
	return namespace + namespaceSeparator + shortName
}

// shortName returns the final backslash-delimited segment of a class name (e.g.
// "FormRequest" from "Illuminate\Foundation\Http\FormRequest"). A name with no
// backslash is returned unchanged. Used to recognise a FormRequest base class
// whether it is written unqualified or fully qualified.
func shortName(name string) string {
	if i := strings.LastIndex(name, namespaceSeparator); i >= 0 {
		return name[i+1:]
	}
	return name
}
