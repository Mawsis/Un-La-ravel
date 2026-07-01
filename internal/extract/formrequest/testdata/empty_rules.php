<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

// A FormRequest whose rules() returns an empty array. It IS a FormRequest (so a
// node is emitted) but validates no fields: the extractor must yield a
// FormRequest with an empty, non-nil Fields slice (serializes "fields": []),
// never nil and never a crash.
// Expected FQN: App\Http\Requests\EmptyRequest.
class EmptyRequest extends FormRequest
{
    public function rules(): array
    {
        return [];
    }
}
