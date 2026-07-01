<?php

namespace App\Http\Controllers;

// The second class named "PostController", in the base controllers namespace.
// Declares FQN "App\Http\Controllers\PostController". Both this and
// admin_post_controller.php are added to the table in Phase 1, so the declared
// set holds BOTH FQNs; only the naming file's use-import decides which one a
// short "PostController" reference resolves to (ADR 0006).
class PostController extends Controller
{
    public function index()
    {
        return [];
    }
}
