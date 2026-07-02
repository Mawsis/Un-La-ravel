<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('posts', function (Blueprint $table) {
            $table->id();
            $table->foreignId('user_id')->constrained();
            $table->string('title');
            $table->text('body');
            $table->boolean('published')->default(false);
            $table->timestamps();

            // user_id is indexed; category_id (added below via ALTER)
            // deliberately is not — fixture for a future missing-index finding.
            $table->index(['user_id']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('posts');
    }
};
