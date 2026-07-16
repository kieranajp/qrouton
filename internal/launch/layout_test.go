package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSupportStartsShellWithShallowTree(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
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
	help, err := os.ReadFile(filepath.Join(dir, ".qrouton", "help.sh"))
	if err != nil {
		t.Fatal("help script missing:", err)
	}
	for _, want := range []string{"delegate work to subagents", "Alt + arrow keys", "Ctrl-g, then Ctrl-q", "Press Enter to begin"} {
		if !strings.Contains(string(help), want) {
			t.Fatalf("help panel missing %q", want)
		}
	}
	if !strings.Contains(string(help), "agents.max_depth is under 2") || !strings.Contains(string(help), "Set it to 3") {
		t.Fatal("Codex quick-start panel does not warn about shallow subagent nesting")
	}
	config, err := os.ReadFile(filepath.Join(dir, ".qrouton", "zellij-config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`bind "Alt x"`, "mouse_mode true", "session_serialization false"} {
		if !strings.Contains(string(config), want) {
			t.Fatalf("Zellij config missing %q", want)
		}
	}
	if !strings.Contains(string(b), `pane split_direction="vertical" size=6`) {
		t.Fatal("status panes are not fixed at six rows")
	}
	if !strings.Contains(string(b), `pane name="repos"`) || !strings.Contains(string(b), `pane name="agents"`) {
		t.Fatal("repo and agent status panes are not side by side")
	}
	if !strings.Contains(string(b), `floating_panes`) || !strings.Contains(string(b), `name="qrouton · quick start"`) || !strings.Contains(string(b), `close_on_exit=true`) {
		t.Fatal("quick-start help is not a disposable floating pane")
	}
}

func TestWriteSupportHidesCodexDepthWarningAtTwo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[agents]\nmax_depth = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := writeSupport(dir, "test-session", []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	help, err := os.ReadFile(filepath.Join(dir, ".qrouton", "help.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(help), "agents.max_depth is under 2") {
		t.Fatal("Codex quick-start panel warns when max_depth is two")
	}
}

func TestStatusScriptFindsSrcWorktrees(t *testing.T) {
	if !strings.Contains(statusScript, "src/*/.git") {
		t.Fatal("status script does not scan src worktrees")
	}
}
