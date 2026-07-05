<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateLensesTable extends Migration
{
    public function up()
    {
        Schema::create('lenses', function (Blueprint $table) {
            $table->id();
            $table->string('name');
            $table->integer('focal_length');
            $table->timestamps();
        });
    }

    public function down()
    {
        Schema::dropIfExists('lenses');
    }
}
