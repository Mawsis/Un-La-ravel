<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Credential extends Model
{
    protected $fillable = ['email'];

    protected $hidden = ['password', 'remember_token'];
}
