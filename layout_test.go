package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSupportStartsShellWithShallowTree(t *testing.T) {
	dir := t.TempDir()
	layout, err := writeSupport(dir, "test-session", []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(layout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "tree -L 2") || !strings.Contains(string(b), `exec \"${SHELL:-/bin/sh}\" -l`) {
		t.Fatalf("shell pane does not show a shallow tree and remain interactive:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(dir, ".qrouton", "status.sh")); err != nil {
		t.Fatal("status script missing:", err)
	}
}

func TestStatusScriptFindsSrcWorktrees(t *testing.T) {
	if !strings.Contains(statusScript, "src/*/.git") {
		t.Fatal("status script does not scan src worktrees")
	}
}
