<?php

use App\Http\Controllers\UserController;
use Illuminate\Support\Facades\Route;

// A single-level route group whose prefix and middleware flatten into the inner
// route (the group-flattening algorithm, ROUTE_FACTS.md). Expected extraction
// (one route):
//   GET /admin/users -> UserController@index,
//        middleware ["auth:sanctum", "throttle:api"] (inherited from the group).
Route::middleware(['auth:sanctum', 'throttle:api'])->prefix('admin')->group(function () {
    Route::get('/users', [UserController::class, 'index']);
});
