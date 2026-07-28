<?php

use Illuminate\Support\Facades\Route;

// A legacy 'Controller@method' string action whose controller carries a
// namespace segment. The extractor splits once on '@' and records the
// controller half verbatim, preserving the Admin\ segment (issue #63).
// Expected extraction:
//   GET /admin/legacy -> controller "Admin\AdminDashboardController",
//                        action "show", no middleware.
Route::get('/admin/legacy', 'Admin\AdminDashboardController@show');
