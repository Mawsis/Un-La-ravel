<?php

use Illuminate\Support\Facades\Route;

// A route file that references "PostController" WITHOUT importing it. Its use-map
// holds no "PostController" entry, so:
//   - Resolve("PostController", thisFile) leaves the name bare ("PostController")
//     and reports found=false (no bare "PostController" class is declared).
//   - ResolveWithDefault("PostController", thisFile, "App\Http\Controllers")
//     applies the Laravel default namespace, yielding
//     "App\Http\Controllers\PostController" and found=true.
// This exercises the default-namespace fallback path (ADR 0006).
Route::get('/posts', [PostController::class, 'index']);
