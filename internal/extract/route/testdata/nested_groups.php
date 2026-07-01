<?php

use App\Http\Controllers\SettingController;
use Illuminate\Support\Facades\Route;

// Nested route groups: the outer and inner prefixes concatenate and both groups'
// middleware accumulate onto the innermost route (the group-flattening algorithm
// composes, ROUTE_FACTS.md). Expected extraction (one route):
//   GET /admin/settings/general -> SettingController@show,
//        middleware ["auth", "verified"] (outer "auth" then inner "verified").
Route::middleware('auth')->prefix('admin')->group(function () {
    Route::middleware('verified')->prefix('settings')->group(function () {
        Route::get('/general', [SettingController::class, 'show']);
    });
});
