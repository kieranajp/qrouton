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
	warning, err := writeSupport(dir, command)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{ID: filepath.Base(command[0]), Command: command}
	agentArgv := superviseArgv("/bin/qrouton", dir, runner,
		mux.Handle{Kind: "zellij", Session: "test-session"}, EditorCommand{Argv: []string{"vi"}}, false)
	if err := mux.NewZellij("zellij", "/tmp/zellij").Stage(workspace(dir, "test-session", agentArgv, runner.ID, "/bin/qrouton", warning)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".qrouton", "layout.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStagedWorkspaceStartsShellWithShallowTree(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	layout := stageWorkspace(t, dir, []string{"codex"})
	if !strings.Contains(layout, "tree -L 2") || !strings.Contains(layout, `exec \"${SHELL:-/bin/sh}\" -l`) {
		t.Fatalf("shell pane does not show a shallow tree and remain interactive:\n%s", layout)
	}
	if !strings.Contains(layout, `name="shell · Alt-g"`) {
		t.Fatal("shell pane is not retitled with the chord that opens a floating terminal")
	}
	if _, err := os.Stat(filepath.Join(dir, ".qrouton", "status.sh")); !os.IsNotExist(err) {
		t.Fatal("status.sh stamped; the repos pane is a qrouton subcommand")
	}
	if _, err := os.Stat(filepath.Join(dir, ".qrouton", "help.sh")); !os.IsNotExist(err) {
		t.Fatal("help.sh stamped into the session; it belongs to the config dir now, one copy globally")
	}
	help, err := os.ReadFile(filepath.Join(configHome, "qrouton", "help.sh"))
	if err != nil {
		t.Fatal("help script missing from the config dir:", err)
	}
	for _, want := range []string{
		"delegate work to subagents", // the fallback RPI tagline; the script resolves the real one at runtime
		"Move focus", "Alt-Tab", "Alt-e", "open the picker", "Alt-n", "back to assistant",
		"Alt-g", "floating shell", "Alt-f", "show / hide", "Alt-x", "Alt-+ / Alt--", "Alt-?",
		"Ctrl-g, then Ctrl-q", "Press any key to close",
	} {
		if !strings.Contains(string(help), want) {
			t.Fatalf("help panel missing %q", want)
		}
	}
	if !strings.Contains(string(help), "stty -icanon") || !strings.Contains(string(help), "dd bs=1 count=1") {
		t.Fatal("quick-reference panel does not dismiss on a single raw keypress")
	}
	if strings.Contains(string(help), "read -r") {
		t.Fatal("quick-reference panel still requires Enter to dismiss")
	}
	config, err := os.ReadFile(filepath.Join(dir, ".qrouton", "zellij-config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`bind "Alt x"`, `bind "Alt g"`, `bind "Alt e"`, `"pick" "--session-root" "` + dir + `"`, `bind "Alt n"`, `"mode" "--session-root" "` + dir + `" "assistant"`, `bind "Alt tab"`, `bind "Alt ?"`, `"sh" "` + filepath.Join(configHome, "qrouton", "help.sh") + `"`, `name "qrouton · terminal"`, `width "90%"`, `height "90%"`, "mouse_mode true", "session_serialization false"} {
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
	if !strings.Contains(layout, `floating_panes`) || !strings.Contains(layout, `name="keys · press any key to close"`) || !strings.Contains(layout, `close_on_exit=true`) {
		t.Fatal("quick-reference panel is not a disposable floating pane")
	}
	if !strings.Contains(layout, `close_on_exit=true focus=true`) {
		t.Fatal("quick-reference pane is not focused; startup keys would land in the agent pane")
	}
	if !strings.Contains(layout, "session_name \"test-session\"") || !strings.Contains(layout, "attach_to_session true") {
		t.Fatal("layout does not name and self-attach the session")
	}
}

// TestStagedWorkspacePassesTheCodexWarningOnlyAtStartup covers both directions
// of the argument that replaced @@WARNING@@: the startup invocation carries it
// for a shallow-depth Codex argv, and the Alt-? binding never does — that's a
// launch-time-only concern, not something to re-warn about on every re-summon.
func TestStagedWorkspacePassesTheCodexWarningOnlyAtStartup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir()) // no config.toml: Codex's own default (1) is under the required depth
	dir := t.TempDir()
	layout := stageWorkspace(t, dir, []string{"codex"})
	if !strings.Contains(layout, "agents.max_depth is under 2") || !strings.Contains(layout, "Set it to 3") {
		t.Fatalf("startup pane does not carry the Codex depth warning as an argument:\n%s", layout)
	}
	config, err := os.ReadFile(filepath.Join(dir, ".qrouton", "zellij-config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "agents.max_depth is under 2") {
		t.Fatal("Alt-? binding carries the Codex warning; it is a launch-time-only concern")
	}
}

func TestStagedWorkspaceCarriesTheStatusStrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[agents]\nmax_depth = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	warning, err := writeSupport(dir, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	if warning != "" {
		t.Fatalf("writeSupport returned a Codex depth warning at max_depth 2: %q", warning)
	}
}

func TestWriteSupportStampsHelpScriptUnderTheConfigDir(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("CODEX_HOME", t.TempDir())
	dirA, dirB := t.TempDir(), t.TempDir()
	if _, err := writeSupport(dirA, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSupport(dirB, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configHome, "qrouton", "help.sh")); err != nil {
		t.Fatal("help script missing from the config dir:", err)
	}
	if _, err := os.Stat(filepath.Join(dirA, ".qrouton", "help.sh")); !os.IsNotExist(err) {
		t.Fatal("help.sh stamped into a session directory; one global copy was the point")
	}
}

func TestWriteSupportStampsNotifyScript(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	if _, err := writeSupport(dir, []string{"codex"}); err != nil {
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
