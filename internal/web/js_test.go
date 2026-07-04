package web

import (
	"os/exec"
	"strings"
	"testing"
)

// TestJSUnitTests runs the dashboard's JavaScript unit tests (the entity-chip /
// cross-link helper, issue #24) under Go's test runner so `go test ./...` and
// CI cover them alongside the Go suite. It shells out to Node's zero-dependency
// built-in test runner — there is no package.json, no node_modules, and no
// build step, matching the web assets' "plain ES modules, no build step"
// constraint.
//
// If Node is not installed the test skips rather than fails: the Go binary and
// the dashboard do not need Node to build or run — it is only needed to execute
// these JS unit tests. CI runs on an image that has Node, so coverage is real
// there; a Go-only contributor machine simply skips them.
func TestJSUnitTests(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found on PATH; skipping JS unit tests (JS runs in the browser, Node only runs the tests)")
	}

	// Pass an explicit glob (not a bare directory): Node's --test treats a
	// directory argument as a module to import, but a glob is expanded to the
	// matching test files. Node expands this glob itself — no shell involved.
	cmd := exec.Command(node, "--test", "jstest/**/*.test.js")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node --test failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "pass ") {
		t.Fatalf("node --test produced no pass summary; output:\n%s", out)
	}
}
