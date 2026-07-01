<?php

use App\Http\Controllers\PostController;
use Illuminate\Support\Facades\Route;

// A verb route decorated with chained ->middleware() and ->name() modifiers.
// Expected extraction (one route):
//   POST /posts -> PostController@store, middleware ["auth"], name "posts.store".
Route::post('/posts', [PostController::class, 'store'])
    ->middleware('auth')
    ->name('posts.store');
