<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Account extends Model
{
    protected $casts = ['email_verified_at' => 'datetime'];
}
