<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

// StorePostRequest validates the POST /posts request body. It is the FormRequest
// the FormRequest↔Route link (ADR 0006) attaches to PostController@store, because
// store type-hints a parameter of this class; the symbol table resolves that
// parameter type through this file's namespace to App\Http\Requests\StorePostRequest.
//
// Its rules() exercises every rules→JSON-Schema mapping branch the OpenAPI
// renderer handles:
//   - 'title'     string syntax, required + typed + max:255 (maxLength as a NUMBER)
//   - 'body'      string syntax, required + typed, no constraints
//   - 'published' string syntax, boolean type only (not required)
//   - 'status'    ARRAY syntax with nullable + in:draft,published (nullable + enum)
//   - 'tags'      ARRAY syntax carrying an UNKNOWN/custom rule ('alpha_dash') that
//                 must be preserved, not dropped, proving unknown-rule survival
class StorePostRequest extends FormRequest
{
    // Every inbound user is authorized in this fixture; authorization is out of
    // scope for the static rules() extraction (ADR 0003).
    public function authorize(): bool
    {
        return true;
    }

    // The validation ruleset. Keys are request-body field names (the FormRequest
    // Fields); values are either a '|'-delimited string ruleset or an array
    // ruleset. Both syntaxes appear here on purpose so the extractor exercises
    // each parse path.
    public function rules(): array
    {
        return [
            'title' => 'required|string|max:255',
            'body' => 'required|string',
            'published' => 'boolean',
            'status' => ['nullable', 'in:draft,published'],
            'tags' => ['nullable', 'alpha_dash'],
        ];
    }
}
