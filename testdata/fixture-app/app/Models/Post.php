<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Post extends Model
{
    // Inferred table: "posts" (snake_case + pluralize). The posts table exists
    // in the migrations, so this model AGREES with the schema.

    // Normal, non-empty mass-assignment declaration — fixture for the
    // straightforward $fillable extraction case.
    protected $fillable = ['title', 'body', 'category_id'];

    // belongsTo User with an explicit FK "user_id". The posts table has a
    // "user_id" column, so this relationship AGREES with the schema.
    public function author()
    {
        return $this->belongsTo(User::class, 'user_id');
    }

    // belongsTo Category (implicit FK). The posts table has a "category_id"
    // column (added by a later migration) and the categories table exists, so
    // this relationship AGREES with the schema.
    public function category()
    {
        return $this->belongsTo(Category::class);
    }

    // DELIBERATE DISAGREEMENT (missing_fk_column): belongsTo User with an
    // explicit FK "editor_id". No migration ever created an "editor_id" column
    // on the posts table, so this relationship DISAGREES with the schema. This
    // proves the Model<->Schema Disagreement finding fires.
    public function editor()
    {
        return $this->belongsTo(User::class, 'editor_id');
    }
}
