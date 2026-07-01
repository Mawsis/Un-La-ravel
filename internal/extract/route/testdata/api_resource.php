<?php

use App\Http\Controllers\CommentController;
use Illuminate\Support\Facades\Route;

// Route::apiResource expands to the five API-facing routes (no create/edit HTML
// pages), in Laravel's canonical order (ROUTE_FACTS.md). Expected extraction
// (five routes, controller "CommentController", no middleware):
//   GET    /comments        index
//   POST   /comments        store
//   GET    /comments/{id}   show
//   PUT    /comments/{id}   update
//   DELETE /comments/{id}   destroy
Route::apiResource('comments', CommentController::class);
