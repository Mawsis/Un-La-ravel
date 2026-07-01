<?php

namespace App\Http\Controllers;

// A controller exercising every method-visibility case the action extractor must
// distinguish (visitor.go isPublicMethod / constructorName). Expected extraction:
//   FQN     = "App\Http\Controllers\PostController"
//   Actions = ["index", "show", "store", "__invoke"]  (source order)
// index/show/store are public (explicit or default visibility) and __invoke is
// the invokable-controller action, so all four are Actions. The constructor is
// excluded by name; the private and protected methods are excluded by visibility.
class PostController extends Controller
{
    // Excluded: constructor is never a routable Action.
    public function __construct()
    {
        //
    }

    // Action: explicit public visibility.
    public function index()
    {
        return [];
    }

    // Action: no visibility modifier — PHP defaults to public.
    function show($id)
    {
        return ['id' => $id];
    }

    // Action: explicit public visibility.
    public function store()
    {
        return null;
    }

    // Excluded: private methods are not routable.
    private function authorizeRequest()
    {
        return true;
    }

    // Excluded: protected methods are not routable.
    protected function transform(array $data)
    {
        return $data;
    }

    // Action: __invoke is the action of an invokable controller, so it is NOT
    // excluded despite being a magic method.
    public function __invoke()
    {
        return 'invoked';
    }
}
