<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

// String-syntax ruleset only. Each value is a single '|'-delimited string the
// extractor splits into rule tokens. Covers a plain rule ('required'), a typed
// rule ('string'), and an arg-bearing rule ('max:255' → {Name:"max", Args:["255"]}).
// Expected FQN: App\Http\Requests\StoreArticleRequest.
class StoreArticleRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'title' => 'required|string|max:255',
            'body' => 'required|string',
        ];
    }
}
