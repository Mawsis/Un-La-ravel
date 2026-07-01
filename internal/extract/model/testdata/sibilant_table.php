<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

// "Box" ends in a sibilant (x), so the inferred table pluralizes to "boxes"
// rather than "boxs". The relationship body is a non-relationship method call
// ($this->save()) that the extractor must ignore, plus one real belongsTo.
class Box extends Model
{
    public function repack()
    {
        return $this->save();
    }

    public function shelf()
    {
        return $this->belongsTo(Shelf::class);
    }
}
