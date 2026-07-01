<?php

use App\Http\Controllers\PostController;
use Illuminate\Support\Facades\Route;

// A verb route with a {param} placeholder in its URI. The placeholder is carried
// through URI joining untouched (uri.go). Expected extraction:
//   GET /posts/{id} -> PostController@show, no middleware.
Route::get('/posts/{id}', [PostController::class, 'show']);
