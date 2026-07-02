package detector_test

import (
	"path/filepath"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/detector"
)

func TestAnalyzeComposer_FixtureApp(t *testing.T) {
	analysis, err := detector.AnalyzeComposer(fixtureAppPath(t))
	if err != nil {
		t.Fatalf("AnalyzeComposer(fixture-app) returned error: %v", err)
	}

	if analysis.LaravelVersion != "^11.0" {
		t.Errorf("LaravelVersion = %q, want %q", analysis.LaravelVersion, "^11.0")
	}
	if analysis.ProjectName != "acme/blog" {
		t.Errorf("ProjectName = %q, want %q", analysis.ProjectName, "acme/blog")
	}

	foundLaravelFramework := false
	for _, dep := range analysis.Dependencies {
		if dep.Name == "laravel/framework" {
			foundLaravelFramework = true
			if !dep.IsLaravelCore {
				t.Error("laravel/framework dependency should have IsLaravelCore = true")
			}
		}
	}
	if !foundLaravelFramework {
		t.Error("expected laravel/framework in Dependencies")
	}

	foundPHPUnit := false
	for _, dep := range analysis.DevDependencies {
		if dep.Name == "phpunit/phpunit" {
			foundPHPUnit = true
		}
	}
	if !foundPHPUnit {
		t.Error("expected phpunit/phpunit in DevDependencies")
	}
}

func TestAnalyzeComposer_MissingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := detector.AnalyzeComposer(dir)
	if err == nil {
		t.Fatal("expected error for missing composer.json, got nil")
	}
}

func TestAnalyzeComposer_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "composer.json"), `{"require": [this is not valid`)

	_, err := detector.AnalyzeComposer(dir)
	if err == nil {
		t.Fatal("expected error for malformed composer.json, got nil")
	}
}

func TestAnalyzeComposer_MissingLaravelFramework(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "composer.json"), `{"require":{"symfony/console":"^7.0"}}`)

	_, err := detector.AnalyzeComposer(dir)
	if err == nil {
		t.Fatal("expected error when laravel/framework is absent, got nil")
	}
}

func TestCheckIfLaravelCore(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		want        bool
	}{
		{"laravel framework", "laravel/framework", true},
		{"laravel sanctum", "laravel/sanctum", true},
		{"non-laravel package", "symfony/console", false},
		{"empty string", "", false},
		{"prefix without slash", "laravel", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.CheckIfLaravelCore(tt.packageName)
			if got != tt.want {
				t.Errorf("CheckIfLaravelCore(%q) = %v, want %v", tt.packageName, got, tt.want)
			}
		})
	}
}
