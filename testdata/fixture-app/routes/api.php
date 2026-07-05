<?php

use App\Http\Controllers\CommentController;
use App\Http\Controllers\PostController;
use App\Http\Controllers\UserController;
use Illuminate\Support\Facades\Route;

// The fixture-app API route file. It exercises every route shape this slice
// extracts and resolves (ADR 0006, ROUTE_FACTS.md): array-callable verb routes,
// a chained ->middleware()->name(), a Route::group with an inherited
// prefix+middleware, a Route::apiResource, and a legacy 'Controller@method'
// string. All routes here resolve to a declared Controller/Action EXCEPT the one
// deliberate dead route marked below.

// Simple verb routes with array callables. Both resolve:
//   GET  /posts        -> PostController@index
//   GET  /posts/{id}   -> PostController@show
Route::get('/posts', [PostController::class, 'index']);
Route::get('/posts/{id}', [PostController::class, 'show']);

// A verb route with a chained ->middleware()->name() modifier. The middleware
// name and route name attach to this single route:
//   POST /posts        -> PostController@store   [auth]   name=posts.store
Route::post('/posts', [PostController::class, 'store'])
    ->middleware('auth')
    ->name('posts.store');

// A legacy 'Controller@method' string action. The short controller name resolves
// through this file's use-imports to App\Http\Controllers\PostController:
//   GET  /legacy       -> PostController@show
Route::get('/legacy', 'PostController@show');

// DELIBERATE PUBLIC WRITE (unauthenticated_write blocker): a POST route with NO
// middleware — anyone can invoke it without logging in. It resolves cleanly to
// PostController@store, so it is NOT a dead route; the only thing wrong with it
// is the missing auth. This is the ONE intentional unauthenticated *write* in the
// fixture, exercising the auth classifier's blocker path end-to-end (issue #50),
// exactly as the dead route below exercises phase-two resolution.
//   POST /webhooks     -> PostController@store   ⚠ PUBLIC WRITE (unauthenticated_write)
Route::post('/webhooks', [PostController::class, 'store']);

// A route GROUP: prefix('admin') + middleware(['auth:sanctum','throttle:api'])
// wrap the inner routes. The prefix flattens into each inner URI and the group
// middleware is inherited by each inner route (the group-flattening algorithm).
Route::middleware(['auth:sanctum', 'throttle:api'])->prefix('admin')->group(function () {
    // Inner verb route. Inherits the /admin prefix and the group middleware:
    //   GET /admin/users -> UserController@index  [auth:sanctum, throttle:api]
    Route::get('/users', [UserController::class, 'index']);

    // apiResource inside the group. Expands to the five API routes, each under
    // the inherited /admin prefix with the inherited group middleware:
    //   GET    /admin/comments        -> CommentController@index
    //   POST   /admin/comments        -> CommentController@store
    //   GET    /admin/comments/{id}   -> CommentController@show
    //   PUT    /admin/comments/{id}   -> CommentController@update
    //   DELETE /admin/comments/{id}   -> CommentController@destroy
    Route::apiResource('comments', CommentController::class);

    // DELIBERATE DEAD ROUTE (missing_action): UserController is a real, declared
    // controller, but it has no destroy() method — only index(). Phase-two
    // resolution matches the class FQN but not the action, so this route is
    // reported as a Dead Route with kind "missing_action" (ADR 0006). This is the
    // ONE intentional dangling edge in the fixture.
    //   DELETE /admin/users/{id} -> UserController@destroy   ⚠ DEAD (missing_action)
    Route::delete('/users/{id}', [UserController::class, 'destroy']);
});
