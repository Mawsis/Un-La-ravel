<?php

use App\Http\Controllers\PostController;
use Illuminate\Support\Facades\Route;

// A route file that imports the BASE-namespace PostController. When this file is
// the naming file (fromFile) for a short "PostController" reference, resolution
// must yield "App\Http\Controllers\PostController" via THIS file's use-map — not
// the Admin one — even though both classes are declared (ADR 0006 anti-wrong-edge).
Route::get('/posts', [PostController::class, 'index']);
