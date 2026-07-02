package detector_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/detector"
)

// repoRoot walks up from this file's directory until it finds go.mod, returning
// the module root so testdata paths resolve regardless of which directory
// `go test` is invoked from. Mirrors internal/engine/engine_test.go's helper.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod above %s", dir)
		}
		dir = parent
	}
}

func fixtureAppPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "fixture-app")
}

func TestDetectLaravel_FixtureApp(t *testing.T) {
	project, err := detector.DetectLaravel(fixtureAppPath(t))
	if err != nil {
		t.Fatalf("DetectLaravel(fixture-app) returned error: %v", err)
	}

	if project.Version != "^11.0" {
		t.Errorf("Version = %q, want %q", project.Version, "^11.0")
	}
	if project.ComposerAnalysis == nil {
		t.Fatal("ComposerAnalysis is nil")
	}
	if project.ComposerAnalysis.ProjectName != "acme/blog" {
		t.Errorf("ProjectName = %q, want %q", project.ComposerAnalysis.ProjectName, "acme/blog")
	}
	if project.DirectoryTree == nil {
		t.Error("DirectoryTree is nil")
	}
}

func TestDetectLaravel_DefaultsToCurrentDirWhenPathEmpty(t *testing.T) {
	fixture := fixtureAppPath(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("os.Chdir(restore): %v", err)
		}
	}()
	if err := os.Chdir(fixture); err != nil {
		t.Fatalf("os.Chdir(%s): %v", fixture, err)
	}

	project, err := detector.DetectLaravel("")
	if err != nil {
		t.Fatalf("DetectLaravel(\"\") returned error: %v", err)
	}
	if project.Path != "." {
		t.Errorf("Path = %q, want %q", project.Path, ".")
	}
}

func TestDetectLaravel_MissingArtisan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "composer.json"), `{"require":{"laravel/framework":"^11.0"}}`)

	_, err := detector.DetectLaravel(dir)
	if err == nil {
		t.Fatal("expected error for missing artisan file, got nil")
	}
}

func TestDetectLaravel_MissingComposerJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "artisan"), "#!/usr/bin/env php\n")

	_, err := detector.DetectLaravel(dir)
	if err == nil {
		t.Fatal("expected error for missing composer.json, got nil")
	}
}

func TestDetectLaravel_ComposerJSONWithoutLaravelFramework(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "artisan"), "#!/usr/bin/env php\n")
	writeFile(t, filepath.Join(dir, "composer.json"), `{"require":{"symfony/console":"^7.0"}}`)

	_, err := detector.DetectLaravel(dir)
	if err == nil {
		t.Fatal("expected error for composer.json without laravel/framework, got nil")
	}
}

func TestDetectLaravel_MalformedComposerJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "artisan"), "#!/usr/bin/env php\n")
	writeFile(t, filepath.Join(dir, "composer.json"), `{not valid json`)

	_, err := detector.DetectLaravel(dir)
	if err == nil {
		t.Fatal("expected error for malformed composer.json, got nil")
	}
}

func TestBuildDirectoryTree_SkipsIgnoredDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "Models", "User.php"), "<?php")
	writeFile(t, filepath.Join(dir, "vendor", "autoload.php"), "<?php")
	writeFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"), "")
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main")

	root, err := detector.BuildDirectoryTree(dir)
	if err != nil {
		t.Fatalf("BuildDirectoryTree: %v", err)
	}

	names := childNames(root)
	for _, ignored := range []string{"vendor", "node_modules", ".git"} {
		if names[ignored] {
			t.Errorf("expected %q to be skipped, but it appeared as a child of root", ignored)
		}
	}
	if !names["app"] {
		t.Error("expected \"app\" to appear as a child of root")
	}
}

func childNames(node *detector.DirectoryNode) map[string]bool {
	names := make(map[string]bool)
	for _, child := range node.Children {
		names[child.Name] = true
	}
	return names
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
