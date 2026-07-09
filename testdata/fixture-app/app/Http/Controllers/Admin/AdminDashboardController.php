<?php

namespace App\Http\Controllers\Admin;

use App\Http\Controllers\Controller;

// AdminDashboardController lives in the app/Http/Controllers/Admin subdirectory,
// so its FQN carries the Admin\ namespace segment
// (App\Http\Controllers\Admin\AdminDashboardController). It backs the admin
// routes added to routes/api.php for issue #63: before the fix, the route
// extractor collapsed the controller reference to the bare "AdminDashboardController"
// and phase-two resolution rebuilt a wrong FQN under App\Http\Controllers, so
// these live routes were reported as dead. The extractor now records the
// reference verbatim, so the Admin\ segment survives and the routes resolve.
class AdminDashboardController extends Controller
{
    // Public: routable Action. GET /admin/dashboard -> resolves via the inline
    // fully-qualified reference at the route site.
    public function index()
    {
        return [];
    }

    // Public: routable Action. GET /admin/dashboard/stats -> resolves via the
    // `use`-imported short reference at the route site.
    public function stats()
    {
        return [];
    }
}
