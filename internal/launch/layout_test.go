package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/mux"
)

// stageWorkspace runs the launch-side stamping plus the Zellij adapter's
// staging, mirroring what Launch does before entering the session, and
// returns the rendered layout. command is the runner's base command; the
// agent pane itself runs the supervisor, as Launch now builds it.
func stageWorkspace(t *testing.T, dir string, command []string) string {
	t.Helper()
	if err := writeSupport(dir, command); err != nil {
		t.Fatal(err)
	}
	runner := Runner{ID: filepath.Base(command[0]), Command: command}
	agentArgv := superviseArgv("/bin/qrouton", dir, runner,
		mux.Handle{Kind: "zellij", Session: "test-session"}, EditorCommand{Argv: []string{"vi"}}, false)
	if err := mux.NewZellij("zellij", "/tmp/zellij").Stage(workspace(dir, "test-session", agentArgv, runner.ID, "/bin/qrouton")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".qrouton", "layout.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStagedWorkspaceStartsShellWithShallowTree(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	layout := stageWorkspace(t, dir, []string{"codex"})
	if !strings.Contains(layout, "tree -L 2") || !strings.Contains(layout, `exec \"${SHELL:-/bin/sh}\" -l`) {
		t.Fatalf("shell pane does not show a shallow tree and remain interactive:\n%s", layout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".qrouton", "status.sh")); !os.IsNotExist(err) {
		t.Fatal("status.sh stamped; the repos pane is a qrouton subcommand")
	}
	help, err := os.ReadFile(filepath.Join(dir, ".qrouton", "help.sh"))
	if err != nil {
		t.Fatal("help script missing:", err)
	}
	for _, want := range []string{"delegate work to subagents", "Alt + arrow keys", "Alt-g (floating terminal)", "Ctrl-g, then Ctrl-q", "Press any key to begin"} {
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
	for _, want := range []string{`bind "Alt x"`, `bind "Alt g"`, `bind "Alt e"`, `"pick" "--session-root" "` + dir + `"`, `bind "Alt n"`, `"mode" "--session-root" "` + dir + `" "assistant"`, `name "qrouton · terminal"`, `width "90%"`, `height "90%"`, "mouse_mode true", "session_serialization false"} {
		if !strings.Contains(string(config), want) {
			t.Fatalf("Zellij config missing %q", want)
		}
	}
	if strings.Contains(string(config), `bind "Alt n" { NewPane; }`) {
		t.Fatal("NewPane still holds the Alt-n chord; it belongs to de-escalation")
	}
	if !strings.Contains(layout, `pane split_direction="vertical" size=6`) {
		t.Fatal("status panes are not fixed at six rows")
	}
	if !strings.Contains(layout, `pane name="repos"`) || !strings.Contains(layout, `pane name="agents"`) {
		t.Fatal("repo and agent status panes are not side by side")
	}
	if !strings.Contains(layout, `"repos" "--session-root"`) {
		t.Fatal("repos pane does not run the qrouton repos subcommand")
	}
	if !strings.Contains(layout, `"agent" "--session-root" "`+dir+`"`) || !strings.Contains(layout, `"--runner" "codex"`) {
		t.Fatal("agent pane does not run the qrouton agent supervisor")
	}
	if !strings.Contains(layout, `"--mux-json"`) || !strings.Contains(layout, `"--editor-json"`) {
		t.Fatal("supervisor argv lacks the handle/editor exec-boundary flags")
	}
	if !strings.Contains(layout, `floating_panes`) || !strings.Contains(layout, `name="qrouton · quick start"`) || !strings.Contains(layout, `close_on_exit=true`) {
		t.Fatal("quick-start help is not a disposable floating pane")
	}
	if !strings.Contains(layout, `close_on_exit=true focus=true`) {
		t.Fatal("quick-start pane is not focused; startup keys would land in the agent pane")
	}
	if !strings.Contains(layout, "session_name \"test-session\"") || !strings.Contains(layout, "attach_to_session true") {
		t.Fatal("layout does not name and self-attach the session")
	}
}

func TestStagedWorkspaceCarriesTheStatusStrip(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	layout := stageWorkspace(t, dir, []string{"codex"})
	if !strings.Contains(layout, `pane size=1 borderless=true name="status"`) {
		t.Fatalf("layout lacks the one-row borderless strip pane:\n%s", layout)
	}
	if !strings.Contains(layout, `"status" "--session-root" "`+dir+`"`) {
		t.Fatal("strip pane does not run the qrouton status subcommand")
	}
	if strings.Contains(layout, "zellij:status-bar") {
		t.Fatal("Zellij's status-bar survived; the strip owns the bottom row")
	}
	if !strings.Contains(layout, "zellij:tab-bar") {
		t.Fatal("tab-bar dropped; only the status-bar was meant to go")
	}
}

func TestWriteSupportHidesCodexDepthWarningAtTwo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[agents]\nmax_depth = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := writeSupport(dir, []string{"codex"}); err != nil {
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

func TestWriteSupportStampsNotifyScript(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	if err := writeSupport(dir, []string{"codex"}); err != nil {
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
