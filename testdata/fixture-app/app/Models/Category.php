<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Category extends Model
{
    // Inferred table: "categories" (snake_case + irregular pluralize). The
    // categories table exists in the migrations, so this model AGREES with the
    // schema.

    // DELIBERATE: fully mass-assignable model (fixture for a future
    // doctor mass-assignment finding). Distinguishes a declared-empty
    // $guarded (non-nil, empty) from a model that never declares it (nil).
    protected $guarded = [];

    // hasMany Post: a category has many posts. The posts table has a
    // "category_id" foreign key (added by a later migration), so this
    // relationship AGREES with the schema.
    public function posts()
    {
        return $this->hasMany(Post::class);
    }
}
