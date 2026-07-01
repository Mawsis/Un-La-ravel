<?php

namespace App\Http\Controllers;

// PostController backs the /posts routes in routes/api.php. Its public methods
// index, show and store are the routable Actions the symbol table matches route
// controller/action edges against (ADR 0006); a route naming any OTHER method on
// this controller would resolve the class but dangle on the action, producing a
// missing_action dead route.
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
    public function store()
    {
        return null;
    }
}
