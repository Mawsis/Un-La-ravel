<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

// Array-syntax ruleset only. Each value is a PHP array whose string elements are
// individual rule tokens (one Rule per element), NOT '|'-split. Covers a plain
// element ('nullable'), a typed element ('string'), and an arg-bearing element
// ('in:draft,published' → {Name:"in", Args:["draft","published"]}).
// Expected FQN: App\Http\Requests\UpdateArticleRequest.
class UpdateArticleRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'status' => ['nullable', 'string', 'in:draft,published'],
            'slug' => ['required', 'string'],
        ];
    }
}
