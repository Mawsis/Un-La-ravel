<?php

use App\Http\Controllers\PostController;
use Illuminate\Support\Facades\Route;

// A single verb route with an array callable action. Expected extraction:
//   GET /posts -> controller "PostController", action "index", no middleware.
Route::get('/posts', [PostController::class, 'index']);
