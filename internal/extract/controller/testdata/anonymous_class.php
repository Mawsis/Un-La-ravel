<?php

namespace App\Http\Controllers;

// A named controller whose body instantiates an ANONYMOUS class. The visitor must
// emit only the named FooController (with its own action), and must NOT let the
// anonymous class's method leak into any controller: the anonymous class has no
// name, so the visitor clears "current" and drops its members (visitor.go
// StmtClass empty-name branch + StmtClassMethod current-nil guard).
class FooController extends Controller
{
    public function index()
    {
        return new class {
            public function leaked()
            {
                return 'must not appear as an action';
            }
        };
    }
}
