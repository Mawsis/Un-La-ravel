<?php

use Illuminate\Support\Facades\Route;

// Route::apiResource whose controller argument is a FULLY-QUALIFIED, sub-
// namespaced class written inline. The resource macro expansion records the
// controller reference verbatim on every expanded route (issue #63) — the
// Admin\ segment survives, so none of the five routes collapse to the bare
// short name. Expected extraction (five routes, controller
// "App\Http\Controllers\Admin\CommentController", no middleware):
//   GET    /comments        index
//   POST   /comments        store
//   GET    /comments/{id}   show
//   PUT    /comments/{id}   update
//   DELETE /comments/{id}   destroy
Route::apiResource('comments', App\Http\Controllers\Admin\CommentController::class);
