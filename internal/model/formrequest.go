package model

// This file defines the FormRequest-related Node types of the Project Model
// (ADR 0001): the FormRequest class, the request-body Field it validates, and
// the parsed validation Rule attached to each Field.
//
// A FormRequest is one class from app/Http/Requests whose rules() method
// returns the validation ruleset for an inbound request body. Its Fields are
// the array keys of that ruleset (the request-body field names); each Field's
// Rules are the parsed validation tokens for that key.
//
// These types record the rules SOURCE-FAITHFULLY: a Rule keeps the raw rule
// name and its arguments exactly as written (for example {Name: "max", Args:
// ["255"]}), with no interpretation. The mapping from these parsed rules to an
// OpenAPI/JSON-Schema property lives in the OpenAPI renderer (render/openapi),
// not here, so this package stays a faithful record and the mapping travels
// with the renderer that needs it.
//
// These are pure data types: no parsing, no I/O, no rule interpretation. The
// AST parsing that produces them lives in an extractor; this package only
// carries the resulting values.

// Rule is one parsed validation rule from a FormRequest field, kept
// source-faithfully. It is the smallest unit of a validation ruleset.
type Rule struct {
	// Name is the rule token as written, without its arguments (for example
	// "required", "string", "max", "in").
	Name string `json:"name"`
	// Args are the rule's arguments, split from the token's colon suffix (for
	// example ["255"] for "max:255", or ["draft","published"] for
	// "in:draft,published"). Omitted from JSON when the rule takes no arguments.
	Args []string `json:"args,omitempty"`
}

// Field is one request-body field of a FormRequest — one key of the rules()
// array — together with its parsed validation Rules in source order.
type Field struct {
	// Name is the field name, i.e. the rules() array key as written (for example
	// "title"), with any surrounding quotes stripped.
	Name string `json:"name"`
	// Rules are the field's parsed validation rules in source-declaration order.
	Rules []Rule `json:"rules"`
}

// FormRequest is one class extracted from app/Http/Requests within the Project
// Model. It carries the fully qualified name used to link the request to a
// controller action's typed parameter (ADR 0006) and the request-body Fields
// parsed from its rules() method. Fields preserve their source-declaration
// order.
type FormRequest struct {
	// Name is the request class name as written (for example
	// "StorePostRequest").
	Name string `json:"name"`
	// FQN is the fully qualified class name (namespace + class, for example
	// "App\\Http\\Requests\\StorePostRequest"). It is the key an action's
	// parameter type resolves against to link a Route to this FormRequest.
	FQN string `json:"fqn"`
	// Fields are the request-body fields parsed from the rules() method, in
	// source-declaration order.
	Fields []Field `json:"fields"`
}

// NewFormRequest returns a FormRequest with the given name and FQN and a
// non-nil Fields slice, so JSON serialization yields "fields": [] rather than
// null for a request whose fields have not yet been appended.
func NewFormRequest(name, fqn string) FormRequest {
	return FormRequest{
		Name:   name,
		FQN:    fqn,
		Fields: []Field{},
	}
}
