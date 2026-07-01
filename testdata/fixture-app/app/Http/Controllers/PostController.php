<?php

namespace App\Http\Controllers;

use App\Http\Requests\StorePostRequest;

// PostController backs the /posts routes in routes/api.php. Its public methods
// index, show and store are the routable Actions the symbol table matches route
// controller/action edges against (ADR 0006); a route naming any OTHER method on
// this controller would resolve the class but dangle on the action, producing a
// missing_action dead route.
//
// store() type-hints StorePostRequest: the controller extractor records that
// action-param type, the symbol table resolves it through this file's use-import
// to App\Http\Requests\StorePostRequest, and analyze links POST /posts to that
// FormRequest's request body (the FormRequest↔Route link).
class PostController extends Controller
{
    // Public: routable Action. GET /posts -> PostController@index (resolves).
    public function index()
    {
        return [];
    }

    // Public: routable Action. GET /posts/{id} -> PostController@show (resolves).
    public function show($id)
    {
        return ['id' => $id];
    }

    // Public: routable Action. POST /posts -> PostController@store (resolves).
    // Type-hints StorePostRequest, which links POST /posts to that FormRequest's
    // request body (ADR 0006). The $request param is FormRequest-typed, unlike
    // show()'s untyped $id, which is how the extractor tells the two apart.
    public function store(StorePostRequest $request)
    {
        return null;
    }
}
