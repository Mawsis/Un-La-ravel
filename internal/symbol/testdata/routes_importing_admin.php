<?php

use App\Http\Controllers\Admin\PostController;
use Illuminate\Support\Facades\Route;

// A route file that imports the ADMIN-namespace PostController. As the naming
// file for a short "PostController" reference, resolution must yield
// "App\Http\Controllers\Admin\PostController" via THIS file's use-map. Contrast
// with routes_importing_base.php: same short name, different resolved FQN, chosen
// solely by the importing file (ADR 0006).
Route::get('/admin/posts', [PostController::class, 'index']);
