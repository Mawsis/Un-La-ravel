<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Lens extends Model
{
    // DELIBERATE NAME MISMATCH (issue #36): the best-effort inflector reads
    // the trailing "s" of "lens" as an existing plural and infers the table
    // "lens", but the migration created "lenses" (Laravel's real Doctrine
    // inflector pluralizes correctly at runtime). This is the reconciliation
    // fixture: ER edges touching this model must retarget to the "lenses"
    // node marked unresolved — never crash the diagram — and the disagreement
    // must suggest `protected $table = 'lenses'`.

    // hasMany Post: the edge's From side is this model's own (missing)
    // inferred table, proving the declaring side is reconciled too.
    public function posts()
    {
        return $this->hasMany(Post::class);
    }
}
