<?php

namespace App\Http\Requests;

// A plain class that does NOT extend a FormRequest base, even though it lives in
// app/Http/Requests and declares a rules() method returning an array. The
// extractor must IGNORE it entirely and emit no FormRequest node — recognition is
// by the parent class (last segment "FormRequest"), not by having a rules()
// method or by directory. Its rules() body must not be parsed.
class RequestHelper
{
    public function rules(): array
    {
        return [
            'ignored' => 'required|string',
        ];
    }
}
