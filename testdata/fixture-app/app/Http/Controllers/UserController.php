<?php

namespace App\Http\Controllers;

// UserController backs the admin group's /admin/users route in routes/api.php.
// It declares only index, so the deliberate dead route pointing at
// UserController@destroy (see routes/api.php) resolves this class but finds no
// such Action — a missing_action dead route (ADR 0006).
class UserController extends Controller
{
    // Public: routable Action. GET /admin/users -> UserController@index (resolves).
    public function index()
    {
        return [];
    }
}
