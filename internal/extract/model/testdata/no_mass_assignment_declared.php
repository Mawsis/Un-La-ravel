<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Widget extends Model
{
    public function owner()
    {
        return $this->belongsTo(User::class);
    }
}
