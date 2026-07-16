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
	if _, err := os.Stat(filepath.Join(dir, ".qrouton", "status.sh")); !os.IsNotExist(err) {
		t.Fatal("stale status.sh still stamped; the repos pane is a qrouton subcommand now")
	}
	help, err := os.ReadFile(filepath.Join(dir, ".qrouton", "help.sh"))
	if err != nil {
		t.Fatal("help script missing:", err)
	}
	for _, want := range []string{"delegate work to subagents", "Alt + arrow keys", "Ctrl-g, then Ctrl-q", "Press any key to begin"} {
		if !strings.Contains(string(help), want) {
			t.Fatalf("help panel missing %q", want)
		}
	}
	if !strings.Contains(string(help), "stty -icanon") || !strings.Contains(string(help), "dd bs=1 count=1") {
		t.Fatal("quick-start panel does not dismiss on a single raw keypress")
	}
	if strings.Contains(string(help), "read -r") {
		t.Fatal("quick-start panel still requires Enter to dismiss")
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
	if !strings.Contains(string(b), `"repos" "--session-root"`) {
		t.Fatal("repos pane does not run the qrouton repos subcommand")
	}
	if !strings.Contains(string(b), `floating_panes`) || !strings.Contains(string(b), `name="qrouton · quick start"`) || !strings.Contains(string(b), `close_on_exit=true`) {
		t.Fatal("quick-start help is not a disposable floating pane")
	}
	if !strings.Contains(string(b), `close_on_exit=true focus=true`) {
		t.Fatal("quick-start pane is not focused; startup keys would land in the agent pane")
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

func TestWriteSupportRemovesStaleStatusScript(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	stale := filepath.Join(dir, ".qrouton", "status.sh")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSupport(dir, "test-session", []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("resumed session kept an orphaned status.sh")
	}
}

func TestWriteSupportStampsNotifyScript(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	if _, err := writeSupport(dir, "test-session", []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, ".qrouton", "notify.sh"))
	if err != nil {
		t.Fatal("notify script missing:", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatal("notify script is not executable")
	}
	if !strings.Contains(notifyScript, "afplay") || !strings.Contains(notifyScript, `printf '\a'`) {
		t.Fatal("notify script lacks a player and bell fallback")
	}
}
