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

    // Serialization-hidden columns, as Laravel's own stock User model declares
    // them. Fixture for the $hidden extraction path (issue #65): a DECLARED,
    // non-empty $hidden. Category covers the declared-empty case, Post the
    // never-declared one, so all three states appear in the goldens.
    protected $hidden = [
        'password',
        'remember_token',
    ];

    // Laravel 11-style casts() method — fixture for the casts() extraction
    // path, which takes precedence over a $casts property when both exist.
    protected function casts(): array
    {
        return [
            'email_verified_at' => 'datetime',
        ];
    }
}
