<?php

namespace App\Models;

use Illuminate\Foundation\Auth\User as Authenticatable;

class User extends Authenticatable
{
    // Inferred table: "users" (snake_case + pluralize). The users table exists
    // in the migrations, so this model AGREES with the schema.

    // hasMany Post: a user authors many posts. The posts table has a "user_id"
    // foreign key (created by foreignId('user_id')->constrained()), so this
    // relationship AGREES with the schema.
    public function posts()
    {
        return $this->hasMany(Post::class);
    }
}
