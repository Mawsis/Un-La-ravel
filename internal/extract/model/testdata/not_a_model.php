<?php

namespace App\Services;

// Plain class that does NOT extend an Eloquent base. The extractor must ignore
// it entirely and emit no Model node, even though it contains a method whose
// body looks like a relationship call.
class ReportBuilder
{
    public function items()
    {
        return $this->hasMany(Item::class);
    }
}
