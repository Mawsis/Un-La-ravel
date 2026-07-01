<?php

use App\Http\Controllers\DashboardController;
use Illuminate\Support\Facades\Route;

// Two behaviors in one file:
//
//  1. A route whose OWN path is '/' inside a prefixed group: the group prefix
//     becomes the whole URI, with no trailing slash (joinURI collapses the empty
//     own-path). Expected: GET /admin -> DashboardController@index, mw ["auth"].
//
//  2. A non-route facade static call and a non-verb Route:: method are BOTH
//     ignored — only genuine verb/resource declarations become routes. Neither
//     Cache::get(...) nor Route::pattern(...) contributes a route.
Cache::get('warm');
Route::pattern('id', '[0-9]+');

Route::middleware('auth')->prefix('admin')->group(function () {
    Route::get('/', [DashboardController::class, 'index']);
});
