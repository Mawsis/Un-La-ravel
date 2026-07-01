<?php

namespace App\Http\Requests;

// Recognition must match on the LAST backslash segment of the parent name, so a
// fully-qualified `extends Illuminate\Foundation\Http\FormRequest` (no `use`
// import) is recognised exactly like the unqualified form. This class must be
// emitted as a FormRequest with FQN App\Http\Requests\StoreTagRequest and its
// single 'name' field parsed.
class StoreTagRequest extends \Illuminate\Foundation\Http\FormRequest
{
    public function rules(): array
    {
        return [
            'name' => 'required|string|max:50',
        ];
    }
}
