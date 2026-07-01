<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

// Proves unknown/custom rules are PRESERVED source-faithfully, not dropped.
// 'alpha_dash' and 'uppercase' are real Laravel rules the JSON-Schema mapping
// does not model; the extractor still records them as plain Rule tokens (Name
// set, Args empty) so the renderer can surface them in a description. The custom
// 'required' + 'alpha_dash' mix appears in both string and array syntax.
// The 'code' field carries the unknown rule 'alpha_dash'.
// Expected FQN: App\Http\Requests\StoreCouponRequest.
class StoreCouponRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'code' => 'required|alpha_dash',
            'label' => ['required', 'uppercase'],
        ];
    }
}
