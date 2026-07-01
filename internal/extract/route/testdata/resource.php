<?php

use App\Http\Controllers\PhotoController;
use Illuminate\Support\Facades\Route;

// Route::resource expands to the seven web routes: the five API actions plus the
// create and edit HTML-form pages, in Laravel's registration order
// (ROUTE_FACTS.md). Expected extraction (seven routes, controller
// "PhotoController", no middleware):
//   GET    /photos             index
//   GET    /photos/create      create
//   POST   /photos             store
//   GET    /photos/{id}        show
//   GET    /photos/{id}/edit   edit
//   PUT    /photos/{id}        update
//   DELETE /photos/{id}        destroy
Route::resource('photos', PhotoController::class);
