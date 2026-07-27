<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

// One minimal migration, so fixture-app11 is a plausible Laravel app rather than
// a schema-less shell. This fixture exists to pin the Laravel 11+ middleware
// path (issue #67), not to re-cover schema extraction — fixture-app already
// pins every column type, the cross-file ALTER merge, and the Model↔Schema
// disagreements. Keeping the schema to a single table keeps this fixture's
// golden focused on what it is actually guarding.
return new class extends Migration
{
    public function up(): void
    {
        Schema::create('posts', function (Blueprint $table) {
            $table->id();
            $table->string('title');
            $table->text('body');
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('posts');
    }
};
