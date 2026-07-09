<?php

use Illuminate\Support\Facades\Route;

// A verb route whose array callable names the controller by its FULLY-QUALIFIED
// class, written inline with no `use` import. The extractor records the
// reference verbatim — the namespace segments are NOT collapsed to the short
// name (issue #63). Expected extraction:
//   GET /admin -> controller "App\Http\Controllers\Admin\AdminDashboardController",
//                 action "index", no middleware.
Route::get('/admin', [App\Http\Controllers\Admin\AdminDashboardController::class, 'index']);
