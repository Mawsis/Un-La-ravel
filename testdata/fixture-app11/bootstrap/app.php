<?php

use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Exceptions;
use Illuminate\Foundation\Configuration\Middleware;

// The Laravel 11+ application bootstrap, part of the end-to-end fixture (issue
// #67). Laravel 11 deleted app/Http/Kernel.php and moved its four middleware
// properties into the ->withMiddleware() closure below, as imperative calls on a
// Middleware configurator. The middleware slice reads those calls WITHOUT
// booting Laravel or requiring any of these middleware classes to exist on disk,
// and produces the same "app"-origin node shape the ≤10 Kernel path produces.
//
// The alias map deliberately declares `tenant`, the middleware the admin route
// group applies (routes/api.php). Without it, `tenant` would be an
// applied-but-undeclared name and surface as an "unknown"-origin node; here it
// resolves to App\Http\Middleware\EnsureTenant with origin "app". It also
// declares `auth` and `throttle` — both Laravel built-ins — proving a declared
// built-in is emitted once, in the declared tier with its resolved class, not
// duplicated as a bare framework node.
return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        api: __DIR__.'/../routes/api.php',
    )
    ->withMiddleware(function (Middleware $middleware) {
        // The alias → class map, Laravel 11's replacement for $middlewareAliases.
        // Each entry becomes an "app"-origin node whose "class" is the FQN.
        $middleware->alias([
            'auth' => \App\Http\Middleware\Authenticate::class,
            'guest' => \App\Http\Middleware\RedirectIfAuthenticated::class,
            'throttle' => \Illuminate\Routing\Middleware\ThrottleRequests::class,
            'tenant' => \App\Http\Middleware\EnsureTenant::class,
        ]);

        // Group membership, via both the define-wholesale and append-to forms.
        // An alias whose resolved class appears here carries the group name.
        $middleware->group('web', [
            \App\Http\Middleware\EncryptCookies::class,
            \Illuminate\Session\Middleware\StartSession::class,
        ]);

        $middleware->appendToGroup('api', [
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
        ]);

        // The global stack: these run on EVERY request, so each resolved alias
        // that lands here becomes a node with "global": true. Laravel accepts a
        // bare class-const as well as a list; the fixture exercises both.
        $middleware->append(\App\Http\Middleware\TrustProxies::class);
        $middleware->prepend([
            \Illuminate\Http\Middleware\HandleCors::class,
        ]);

        // Execution priority when it matters. A resolved class listed here gets
        // its 1-based position in "priority".
        $middleware->priority([
            \Illuminate\Session\Middleware\StartSession::class,
            \App\Http\Middleware\Authenticate::class,
            \Illuminate\Routing\Middleware\ThrottleRequests::class,
        ]);
    })
    ->withExceptions(function (Exceptions $exceptions) {
        //
    })->create();
