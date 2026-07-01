<?php

namespace App\Http\Controllers;

use App\Http\Requests\StorePostRequest;
use App\Http\Requests\UpdatePostRequest;

// Exercises the ActionParams side map: store takes a FormRequest-typed param,
// update takes a FormRequest plus an untyped route-model param (the untyped one
// must be omitted), and index takes no parameters at all (absent from the map).
class PostController extends Controller
{
    public function index()
    {
    }

    public function store(StorePostRequest $request)
    {
    }

    public function update(UpdatePostRequest $request, $id)
    {
    }
}
