<?php

namespace App\Http\Controllers;

// A controller with ONLY a constructor and non-public methods, so it yields no
// Actions. It must still be emitted (no base-class filter) with a non-nil, empty
// Actions slice so JSON serializes "actions": [] rather than null.
class EmptyController
{
    public function __construct()
    {
        //
    }

    private function helper()
    {
        return true;
    }

    protected function transform(array $data)
    {
        return $data;
    }
}
