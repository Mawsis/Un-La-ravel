<?php

namespace App\Http\Controllers;

// Two classes declared in ONE file, both in the same namespace. Extract emits one
// domain.Controller per class in source order, proving the visitor tracks the
// "current" class across members and attaches methods by visit order.
class FirstController extends Controller
{
    public function index()
    {
        return [];
    }
}

class SecondController extends Controller
{
    public function store()
    {
        return null;
    }
}
