<?php

namespace App\Http\Controllers;

// CommentController backs the Route::apiResource('comments', ...) in the admin
// group. It declares all five API resource Actions (index, store, show, update,
// destroy), so every route the apiResource macro expands to resolves cleanly —
// exercising the "resolved resource under an inherited prefix" path (ADR 0006).
class CommentController extends Controller
{
    public function index()
    {
        return [];
    }

    public function store()
    {
        return null;
    }

    public function show($id)
    {
        return ['id' => $id];
    }

    public function update($id)
    {
        return ['id' => $id];
    }

    public function destroy($id)
    {
        return null;
    }
}
