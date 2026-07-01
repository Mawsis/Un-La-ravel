<?php

use Illuminate\Support\Facades\Route;

// A legacy 'Controller@method' string action (pre-array-callable Laravel syntax).
// The extractor splits on '@'. Expected extraction:
//   GET /legacy -> controller "LegacyController", action "show", no middleware.
// The controller short name is recorded as written; FQN resolution is phase two.
Route::get('/legacy', 'LegacyController@show');
