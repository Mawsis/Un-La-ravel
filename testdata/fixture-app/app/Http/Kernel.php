<?php

namespace App\Http;

use Illuminate\Foundation\Http\Kernel as HttpKernel;

// A Laravel-10-style HTTP Kernel, part of the end-to-end fixture (issue #66). The
// middleware slice reads its four array-literal properties WITHOUT booting
// Laravel or requiring any of these middleware classes to exist on disk: the
// reader parses the arrays, resolves each alias to its class FQN, reads group
// membership, the global stack, and the priority ordering, and emits an
// "app"-origin node per declared alias — sorted ahead of the built-in backstop.
//
// The alias map deliberately declares `tenant`, the middleware the admin route
// group applies (routes/api.php). Before this Kernel existed, `tenant` was an
// applied-but-undeclared name and surfaced as an "unknown"-origin node; now it
// resolves to App\Http\Middleware\EnsureTenant with origin "app". It also
// declares `auth` and `throttle` — both Laravel built-ins — proving a declared
// built-in is emitted once, in the Kernel tier with its resolved class, not
// duplicated as a bare framework node.
class Kernel extends HttpKernel
{
    // The global middleware stack: these run on EVERY request. Each becomes an
    // app node with "global": true.
    protected $middleware = [
        \App\Http\Middleware\TrustProxies::class,
        \Illuminate\Foundation\Http\Middleware\PreventRequestsDuringMaintenance::class,
        \Illuminate\Http\Middleware\HandleCors::class,
    ];

    // Route middleware groups. An alias whose resolved class appears here carries
    // the matching group name(s) in its "groups" field.
    protected $middlewareGroups = [
        'web' => [
            \App\Http\Middleware\EncryptCookies::class,
            \Illuminate\Session\Middleware\StartSession::class,
            \Illuminate\View\Middleware\ShareErrorsFromSession::class,
        ],

        'api' => [
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
            \Illuminate\Routing\Middleware\SubstituteBindings::class,
        ],
    ];

    // The alias → class map (Laravel 9+ name). Each entry becomes an "app"-origin
    // node whose "class" is the resolved FQN.
    protected $middlewareAliases = [
        'auth' => \App\Http\Middleware\Authenticate::class,
        'auth.basic' => \Illuminate\Auth\Middleware\AuthenticateWithBasicAuth::class,
        'guest' => \App\Http\Middleware\RedirectIfAuthenticated::class,
        'throttle' => \Illuminate\Routing\Middleware\ThrottleRequests::class,
        'verified' => \Illuminate\Auth\Middleware\EnsureEmailIsVerified::class,
        'tenant' => \App\Http\Middleware\EnsureTenant::class,
    ];

    // Execution priority when it matters. An alias's resolved class listed here
    // gets its 1-based position in "priority".
    protected $middlewarePriority = [
        \Illuminate\Session\Middleware\StartSession::class,
        \Illuminate\View\Middleware\ShareErrorsFromSession::class,
        \App\Http\Middleware\Authenticate::class,
        \Illuminate\Routing\Middleware\ThrottleRequests::class,
        \Illuminate\Routing\Middleware\SubstituteBindings::class,
    ];
}
