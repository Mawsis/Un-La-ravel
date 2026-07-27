<?php

use App\Http\Controllers\PostController;
use Illuminate\Support\Facades\Route;

// The fixture-app11 API route file, deliberately minimal: this fixture exists to
// prove the Laravel 11+ bootstrap/app.php middleware path (issue #67), not to
// re-cover route extraction — fixture-app already pins every route shape. What
// it must exercise is the interaction between applied middleware and the
// declared tier read from bootstrap/app.php:
//
//   - `auth` and `tenant` are APPLIED here and DECLARED in bootstrap/app.php, so
//     they resolve to "app"-origin nodes with their classes.
//   - `audit.log` is APPLIED here and declared NOWHERE — and, unlike `auth` or
//     `subscribed`, is not a Laravel built-in either — so it stays an
//     "unknown"-origin node, proving the union's third tier still fires on 11+.
Route::get('/posts', [PostController::class, 'index']);

Route::post('/posts', [PostController::class, 'store'])
    ->middleware('auth');

Route::middleware(['tenant', 'audit.log'])->prefix('admin')->group(function () {
    Route::get('/posts', [PostController::class, 'adminIndex']);
});
