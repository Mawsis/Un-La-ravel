<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

// The one Eloquent model of fixture-app11, matching the single posts migration.
// Like the rest of this fixture it is deliberately plain: fixture-app already
// pins relationship extraction, fillable/guarded handling, and the Model↔Schema
// disagreement correlation. What this fixture guards is the Laravel 11+
// middleware path (issue #67).
class Post extends Model
{
    protected $fillable = [
        'title',
        'body',
    ];
}
