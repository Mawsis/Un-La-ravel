<?php

namespace App\Http\Controllers\Admin;

// One of two classes sharing the SHORT name "PostController" in DIFFERENT
// namespaces. Declares FQN "App\Http\Controllers\Admin\PostController". Paired
// with post_controller.php, this proves the anti-wrong-edge invariant (ADR 0006):
// resolving "PostController" must depend on the naming file's use-map, never on
// the short name alone.
class PostController extends Controller
{
    public function index()
    {
        return [];
    }
}
