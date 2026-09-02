package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequiresExplicitPaths(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("run() succeeded")
	}
}

func TestRunReportsSortedDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "z.go"), "package z\n// one\n// two\n")
	writeFile(t, filepath.Join(root, "a.go"), "package a\n// one\n// two\n")
	policy := filepath.Join(root, "policy.json")
	writeFile(t, policy, `{"maxCommentRun":1,"narrationPhrases":["turns out"],"pathExtensions":["go"]}`)
	var stdout bytes.Buffer
	err := run([]string{"-root", root, "-policy", policy}, &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run() succeeded")
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "a.go:2:1:") || !strings.HasPrefix(lines[1], "z.go:2:1:") {
		t.Fatalf("run() output = %q", stdout.String())
	}
}

func TestRunPassesCleanTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "clean.go"), "package clean\n")
	policy := filepath.Join(root, "policy.json")
	writeFile(t, policy, `{"maxCommentRun":1,"narrationPhrases":["turns out"],"pathExtensions":["go"]}`)
	if err := run([]string{"-root", root, "-policy", policy}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
