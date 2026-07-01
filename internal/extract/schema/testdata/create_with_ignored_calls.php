<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        DB::statement('SET FOREIGN_KEY_CHECKS=0');
        Schema::drop('old_table');
        Schema::create('profiles', function (Blueprint $table) {
            $table->id();
            $table->string('bio')->nullable();
        });
        Schema::drop('another_old_table');
        DB::statement('SET FOREIGN_KEY_CHECKS=1');
    }
};
