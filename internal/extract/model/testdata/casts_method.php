<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Invoice extends Model
{
    public function casts(): array
    {
        return [
            'paid_at' => 'datetime',
            'total' => 'integer',
        ];
    }
}
