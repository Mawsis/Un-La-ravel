<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

// Focuses on rules WITH arguments across both syntaxes, so the token parser is
// exercised for each. Args are split from the token's single ':' suffix, then on
// ','. A single-arg rule ('max:255' → Args ["255"]), a multi-arg list rule
// ('in:a,b,c' → Args ["a","b","c"]), and 'between:1,10' (Args ["1","10"]) all
// appear. The colon is cut ONCE so a value that itself contains ':' stays in the
// arg portion. Multiple fields, mixed string and array syntax.
// Expected FQN: App\Http\Requests\FilterRequest.
class FilterRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'size' => 'required|max:255',
            'kind' => 'in:a,b,c',
            'count' => ['integer', 'between:1,10'],
        ];
    }
}
