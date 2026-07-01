// Package openapi is the OpenAPI Renderer (ADR 0001, ADR 0004): it turns a
// Project Model into a valid OpenAPI 3.0.3 JSON document — the showpiece output
// that presents the discovered Routes as an API specification, with request
// bodies derived from each Route's linked FormRequest validation rules.
//
// It is a pure model→JSON transform. It reads ONLY the in-memory model
// (model.ProjectModel) and never touches source files, the parser, or any other
// I/O (ADR 0004). Output is deterministic: the document is assembled into Go
// structs and Go maps and serialized with encoding/json, which sorts map keys,
// so path keys and HTTP methods come out in a stable order and identical models
// always produce byte-identical JSON — which golden-file tests rely on.
//
// This package owns the rules→JSON-Schema mapping. The model records a
// FormRequest's validation rules source-faithfully (model.Rule keeps the raw
// rule name and args); the interpretation of those rules into an OpenAPI object
// schema — types, formats, enums, numeric bounds, required flags — lives here,
// with the renderer that needs it, not in the model.
package openapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

const (
	// openAPIVersion is the OpenAPI Specification version this renderer emits.
	// The document's top-level "openapi" field must carry a 3.0.x version for
	// tooling to parse it; 3.0.3 is the patch release whose object model this
	// renderer targets.
	openAPIVersion = "3.0.3"

	// defaultInfoVersion is the info.version stamped on the document when the
	// model carries no Laravel version to report. OpenAPI requires info.version
	// to be a non-empty string, so a sensible default keeps the document valid.
	defaultInfoVersion = "1.0.0"

	// defaultInfoTitle is the info.title used when the model has no project name.
	// OpenAPI requires info.title to be present, so a placeholder keeps the
	// document valid rather than emitting an empty string.
	defaultInfoTitle = "Unlaravel API"

	// jsonMediaType is the request-body media type under which every derived
	// request schema is placed. Laravel FormRequests validate the request body,
	// which this renderer presents as a JSON payload.
	jsonMediaType = "application/json"

	// jsonIndent is the indentation for the serialized document. Two spaces keeps
	// golden-file diffs small and matches the model's own serializer.
	jsonIndent = "  "
)

const (
	// schemaTypeObject/String/Integer/Number/Boolean/Array are the JSON-Schema
	// "type" values this renderer emits. Defined once so the rule mapping and the
	// path parameters never hardcode the type literals.
	schemaTypeObject  = "object"
	schemaTypeString  = "string"
	schemaTypeInteger = "integer"
	schemaTypeNumber  = "number"
	schemaTypeBoolean = "boolean"
	schemaTypeArray   = "array"

	// defaultFieldType is the JSON-Schema type assigned to a request field whose
	// rules name no scalar type (FACTS: "default type when none → string").
	defaultFieldType = schemaTypeString
)

const (
	// paramIn is the "in" value for a path parameter derived from a "{param}"
	// placeholder in a Route URI. Only path parameters are derived here; query
	// and header parameters are out of scope for this slice.
	paramIn = "path"

	// defaultResponseStatus / defaultResponseDescription form the single minimal
	// response every Operation carries so the document is valid: OpenAPI requires
	// each Operation to declare at least one response.
	defaultResponseStatus      = "200"
	defaultResponseDescription = "OK"

	// operationIDSeparator joins a controller and action into a synthesized
	// operationId when a Route carries no explicit name (for example
	// "PostController_store").
	operationIDSeparator = "_"
)

// Validation-rule token names recognized by the rules→schema mapping. Laravel
// rules are matched against these; anything else is preserved in the property
// description rather than dropped (FACTS: "unknown rule → description; never
// crash or drop"). Defined once so the mapping never hardcodes the literals.
const (
	ruleRequired  = "required"
	ruleNullable  = "nullable"
	ruleSometimes = "sometimes"

	ruleString  = "string"
	ruleInteger = "integer"
	ruleNumeric = "numeric"
	ruleBoolean = "boolean"
	ruleArray   = "array"

	ruleEmail = "email"
	ruleURL   = "url"
	ruleUUID  = "uuid"
	ruleDate  = "date"

	ruleIn  = "in"
	ruleMax = "max"
	ruleMin = "min"
)

// String-format values emitted for the format-bearing rules. These are the
// OpenAPI/JSON-Schema "format" annotations layered onto a string-typed property.
const (
	formatEmail = "email"
	formatURI   = "uri"
	formatUUID  = "uuid"
	formatDate  = "date"
)

// unknownRuleDescriptionPrefix prefixes an unrecognized rule preserved in a
// property's description, so the raw rule survives in the document without being
// interpreted (FACTS: "preserve in description as 'rule: <name>'").
const unknownRuleDescriptionPrefix = "rule: "

// Render turns a whole Project Model into an OpenAPI 3.0.3 JSON document.
//
// It builds one path item per distinct Route URI (with "{param}" placeholders
// kept), and under each path one Operation per lowercased HTTP method. An
// Operation carries an operationId, the controller as its single tag, a summary,
// one path parameter per "{param}" in the URI, and a minimal 200 response. When
// the Route links a FormRequest (Route.FormRequest names a FormRequest present
// in the model), the Operation also carries a required JSON requestBody whose
// schema is derived from that FormRequest's Fields via the rules→schema mapping.
//
// A nil model renders a valid, empty document (info defaults, no paths) rather
// than panicking. The renderer reads ONLY the in-memory model (ADR 0004); it
// never loads source files or the parser. Output is deterministic: the document
// is serialized with encoding/json, whose map-key sorting yields stable path and
// method ordering for golden-file tests.
func Render(m *model.ProjectModel) ([]byte, error) {
	doc := buildDocument(m)

	out, err := json.MarshalIndent(doc, "", jsonIndent)
	if err != nil {
		return nil, fmt.Errorf("openapi: serialize OpenAPI document to JSON: %w", err)
	}
	return out, nil
}

// document is the top-level OpenAPI 3 document. Field order is the JSON field
// order; Paths is a map, so encoding/json sorts its keys, giving deterministic
// path ordering without the renderer sorting them itself.
type document struct {
	OpenAPI string              `json:"openapi"`
	Info    info                `json:"info"`
	Paths   map[string]pathItem `json:"paths"`
}

// info is the document's info block. OpenAPI requires both title and version to
// be present and non-empty.
type info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// pathItem is the set of Operations for one URI, keyed by lowercased HTTP
// method. It is a map so encoding/json sorts the method keys, giving
// deterministic method ordering under each path.
type pathItem map[string]operation

// operation is one HTTP method's Operation object. RequestBody is a pointer so
// it is omitted entirely for Operations with no linked FormRequest, rather than
// emitting an empty object.
type operation struct {
	OperationID string              `json:"operationId"`
	Tags        []string            `json:"tags,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Parameters  []parameter         `json:"parameters,omitempty"`
	RequestBody *requestBody        `json:"requestBody,omitempty"`
	Responses   map[string]response `json:"responses"`
}

// parameter is one OpenAPI parameter object. Only path parameters are produced
// here, so In is always "path" and Required is always true.
type parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Schema   schema `json:"schema"`
}

// requestBody is an OpenAPI requestBody object. Content is keyed by media type
// (only application/json here); it is a map so it serializes deterministically.
type requestBody struct {
	Required bool                     `json:"required"`
	Content  map[string]mediaTypeItem `json:"content"`
}

// mediaTypeItem is the schema wrapper under a media type in a requestBody.
type mediaTypeItem struct {
	Schema schema `json:"schema"`
}

// response is one OpenAPI response object. Only a description is emitted; the
// response body schema (from API Resources) is out of scope for this slice.
type response struct {
	Description string `json:"description"`
}

// schema is a JSON-Schema object as OpenAPI 3.0.3 uses it. Every field is
// omitempty so a property carries only the constraints its rules produced, and
// so a bare "{}" schema is emitted for a field with no recognized rules.
//
// Properties is an ordered concern for object schemas, but request-body property
// order is not semantically meaningful in JSON-Schema; encoding/json sorts the
// map keys, which keeps golden output deterministic.
//
// The numeric bound fields (MaxLength, MinLength, Maximum, Minimum) are pointers
// so a bound of zero is still emitted, and — critically — they are numeric types
// (int/float64), never strings, so the emitted JSON carries numbers as OpenAPI
// requires (FACTS BUG 1).
type schema struct {
	Type        string            `json:"type,omitempty"`
	Format      string            `json:"format,omitempty"`
	Nullable    bool              `json:"nullable,omitempty"`
	Enum        []string          `json:"enum,omitempty"`
	MaxLength   *int              `json:"maxLength,omitempty"`
	MinLength   *int              `json:"minLength,omitempty"`
	Maximum     *float64          `json:"maximum,omitempty"`
	Minimum     *float64          `json:"minimum,omitempty"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]schema `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
}

// buildDocument assembles the OpenAPI document value from the model, ready to
// serialize. A nil model yields a valid document with default info and no paths.
func buildDocument(m *model.ProjectModel) document {
	if m == nil {
		return document{
			OpenAPI: openAPIVersion,
			Info:    info{Title: defaultInfoTitle, Version: defaultInfoVersion},
			Paths:   map[string]pathItem{},
		}
	}

	requests := formRequestsByKey(m.FormRequests)

	paths := make(map[string]pathItem, len(m.Routes))
	for _, r := range m.Routes {
		item, ok := paths[r.URI]
		if !ok {
			item = pathItem{}
			paths[r.URI] = item
		}
		item[strings.ToLower(r.Method)] = buildOperation(r, requests)
	}

	return document{
		OpenAPI: openAPIVersion,
		Info: info{
			Title:   infoTitle(m.ProjectName),
			Version: infoVersion(m.LaravelVersion),
		},
		Paths: paths,
	}
}

// buildOperation builds the Operation for a single Route: its operationId, tag,
// summary, path parameters, an optional request body derived from a linked
// FormRequest, and the minimal default response.
func buildOperation(r model.Route, requests map[string]model.FormRequest) operation {
	op := operation{
		OperationID: operationID(r),
		Summary:     operationSummary(r),
		Parameters:  pathParameters(r.URI),
		Responses:   defaultResponses(),
	}
	if r.Controller != "" {
		op.Tags = []string{r.Controller}
	}
	if body, ok := requestBodyFor(r, requests); ok {
		op.RequestBody = body
	}
	return op
}

// requestBodyFor returns the request body for a Route when it links a
// FormRequest present in the model, reporting false when the Route has no linked
// FormRequest (its action takes no FormRequest parameter) or the linked class is
// unknown to the model. The body's schema is the object schema derived from the
// FormRequest's Fields.
func requestBodyFor(r model.Route, requests map[string]model.FormRequest) (*requestBody, bool) {
	if r.FormRequest == "" {
		return nil, false
	}
	fr, ok := requests[r.FormRequest]
	if !ok {
		return nil, false
	}

	body := &requestBody{
		Required: true,
		Content: map[string]mediaTypeItem{
			jsonMediaType: {Schema: objectSchema(fr.Fields)},
		},
	}
	return body, true
}

// objectSchema builds the JSON-Schema object for a FormRequest's fields: one
// property per Field (mapped from its rules) plus a "required" list naming the
// fields whose rules include "required". The required list preserves field
// declaration order so golden output is stable.
func objectSchema(fields []model.Field) schema {
	props := make(map[string]schema, len(fields))
	required := make([]string, 0, len(fields))

	for _, f := range fields {
		prop, isRequired := fieldSchema(f.Rules)
		props[f.Name] = prop
		if isRequired {
			required = append(required, f.Name)
		}
	}

	out := schema{
		Type:       schemaTypeObject,
		Properties: props,
	}
	if len(required) > 0 {
		out.Required = required
	}
	return out
}

// fieldSchema maps one field's parsed rule list to a JSON-Schema property and a
// flag reporting whether the field is required (which the caller records in the
// object's "required" list, not as a property key — FACTS).
//
// It runs in TWO PASSES so it is independent of rule order (FACTS BUG 2):
//   - Pass 1 resolves the property's type, nullability, format, and enum, and
//     detects the required flag.
//   - Pass 2 applies numeric bounds (max/min), which dispatch to string-length
//     vs numeric-value constraints based on the now-known type.
//
// Unrecognized rules are appended to the property description rather than
// dropped, so no rule is ever silently lost.
func fieldSchema(rules []model.Rule) (schema, bool) {
	prop := schema{Type: defaultFieldType}
	required := false
	var descriptions []string

	// Pass 1: type, required, nullable, format, enum, and unknown collection.
	for _, rule := range rules {
		switch rule.Name {
		case ruleRequired:
			required = true
		case ruleNullable, ruleSometimes:
			prop.Nullable = true
		case ruleString:
			prop.Type = schemaTypeString
		case ruleInteger:
			prop.Type = schemaTypeInteger
		case ruleNumeric:
			prop.Type = schemaTypeNumber
		case ruleBoolean:
			prop.Type = schemaTypeBoolean
		case ruleArray:
			prop.Type = schemaTypeArray
		case ruleEmail:
			prop.Type = schemaTypeString
			prop.Format = formatEmail
		case ruleURL:
			prop.Type = schemaTypeString
			prop.Format = formatURI
		case ruleUUID:
			prop.Type = schemaTypeString
			prop.Format = formatUUID
		case ruleDate:
			prop.Type = schemaTypeString
			prop.Format = formatDate
		case ruleIn:
			prop.Enum = append([]string(nil), rule.Args...)
		case ruleMax, ruleMin:
			// Applied in pass 2 once the type is known.
		default:
			descriptions = append(descriptions, unknownRuleDescriptionPrefix+rule.Name)
		}
	}

	// Pass 2: numeric bounds, now that the type is settled.
	for _, rule := range rules {
		switch rule.Name {
		case ruleMax:
			applyBound(&prop, prop.Type, rule, true)
		case ruleMin:
			applyBound(&prop, prop.Type, rule, false)
		}
	}

	if len(descriptions) > 0 {
		prop.Description = strings.Join(descriptions, "; ")
	}
	return prop, required
}

// applyBound applies a max (isMax true) or min (isMax false) bound to a property
// according to its resolved type: a string bound becomes maxLength/minLength, a
// numeric (integer/number) bound becomes maximum/minimum. The bound value is
// parsed from the rule's first argument into a NUMBER (never a string — FACTS
// BUG 1); a rule with no argument, or one whose argument is not numeric, is
// ignored rather than emitted malformed.
func applyBound(prop *schema, resolvedType string, rule model.Rule, isMax bool) {
	arg, ok := firstArg(rule)
	if !ok {
		return
	}

	switch resolvedType {
	case schemaTypeInteger, schemaTypeNumber:
		value, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return
		}
		if isMax {
			prop.Maximum = &value
		} else {
			prop.Minimum = &value
		}
	default:
		// Strings (and any non-numeric type default) use length bounds.
		value, err := strconv.Atoi(arg)
		if err != nil {
			return
		}
		if isMax {
			prop.MaxLength = &value
		} else {
			prop.MinLength = &value
		}
	}
}

// firstArg returns a rule's first argument and true, or ("", false) when the
// rule has no arguments — so a bound rule written without a value (a malformed
// "max") is skipped rather than crashing.
func firstArg(rule model.Rule) (string, bool) {
	if len(rule.Args) == 0 {
		return "", false
	}
	return rule.Args[0], true
}

// formRequestsByKey indexes the model's FormRequests so a Route's FormRequest
// reference (which the analyzer may set to either the FQN or the short name) can
// be resolved to the FormRequest value. Both keys are registered for each
// request, so the link resolves whichever spelling the analyzer used. The FQN
// key takes precedence when a short name would otherwise collide across
// namespaces.
func formRequestsByKey(requests []model.FormRequest) map[string]model.FormRequest {
	byKey := make(map[string]model.FormRequest, len(requests)*2)
	for _, fr := range requests {
		if fr.Name != "" {
			// Register the short name first so an FQN registered below always
			// wins on collision.
			if _, exists := byKey[fr.Name]; !exists {
				byKey[fr.Name] = fr
			}
		}
	}
	for _, fr := range requests {
		if fr.FQN != "" {
			byKey[fr.FQN] = fr
		}
	}
	return byKey
}

// pathParameters derives one OpenAPI path parameter per "{param}" placeholder in
// the URI, in first-seen order, de-duplicated so a URI mentioning the same
// placeholder twice yields a single parameter. Each parameter is a required
// string path parameter. A URI with no placeholders yields nil, so the
// Operation's parameters field is omitted.
func pathParameters(uri string) []parameter {
	names := pathParameterNames(uri)
	if len(names) == 0 {
		return nil
	}

	params := make([]parameter, 0, len(names))
	for _, name := range names {
		params = append(params, parameter{
			Name:     name,
			In:       paramIn,
			Required: true,
			Schema:   schema{Type: schemaTypeString},
		})
	}
	return params
}

// pathParameterNames extracts the placeholder names from a URI, stripping the
// enclosing braces and Laravel's optional-marker "?" suffix (for example
// "{id?}" yields "id"). Names are returned in first-seen order and de-duplicated.
// Malformed placeholders (an unmatched "{" with no closing "}") are ignored.
func pathParameterNames(uri string) []string {
	var names []string
	seen := make(map[string]struct{})

	rest := uri
	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			break
		}
		rest = rest[open+1:]
		close := strings.IndexByte(rest, '}')
		if close < 0 {
			break
		}
		name := strings.TrimSuffix(rest[:close], "?")
		rest = rest[close+1:]

		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

// operationID returns a stable operationId for a Route: its route name when one
// was assigned, else the controller and action joined (for example
// "PostController_store"). Falls back to the URI when neither is available, so
// the field is never empty (OpenAPI requires operationId to be unique when
// present; an empty string would collide).
func operationID(r model.Route) string {
	if r.Name != "" {
		return r.Name
	}
	if r.Controller != "" && r.Action != "" {
		return r.Controller + operationIDSeparator + r.Action
	}
	if r.Action != "" {
		return r.Action
	}
	return strings.ToLower(r.Method) + operationIDSeparator + r.URI
}

// operationSummary returns a human-readable summary for a Route: its
// "Controller@action" when both are known, else whichever is present, else the
// method and URI. It is a display aid only.
func operationSummary(r model.Route) string {
	switch {
	case r.Controller != "" && r.Action != "":
		return r.Controller + "@" + r.Action
	case r.Action != "":
		return r.Action
	default:
		return r.Method + " " + r.URI
	}
}

// defaultResponses returns the minimal response set every Operation carries so
// the document is valid: a single 200 with a plain description. Response-body
// schemas are out of scope for this slice.
func defaultResponses() map[string]response {
	return map[string]response{
		defaultResponseStatus: {Description: defaultResponseDescription},
	}
}

// infoTitle returns the document title: the project name, or a default when it
// is empty, so info.title is always a non-empty string as OpenAPI requires.
func infoTitle(projectName string) string {
	if projectName == "" {
		return defaultInfoTitle
	}
	return projectName
}

// infoVersion returns the document version: the model's Laravel version, or a
// default when it is empty, so info.version is always a non-empty string as
// OpenAPI requires.
func infoVersion(laravelVersion string) string {
	if laravelVersion == "" {
		return defaultInfoVersion
	}
	return laravelVersion
}
