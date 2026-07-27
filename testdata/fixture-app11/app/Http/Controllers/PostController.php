<?php

namespace App\Http\Controllers;

// The one controller of fixture-app11, present so the routes above resolve to a
// real action rather than registering as dead routes and muddying the signal
// this fixture exists for (the bootstrap/app.php middleware path, issue #67).
class PostController
{
    public function index()
    {
        return [];
    }

    public function store()
    {
        return [];
    }

    public function adminIndex()
    {
        return [];
    }
}
