package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/mux"
)

// stageWorkspace runs the launch-side stamping plus the Zellij adapter's
// staging, mirroring what Launch does before entering the session, and
// returns the rendered layout. command is the runner's base command; the
// agent pane itself runs the supervisor, as Launch now builds it.
func stageWorkspace(t *testing.T, dir string, command []string) string {
	t.Helper()
	if err := writeSupport(dir); err != nil {
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

func TestStagedWorkspaceStartsPermanentShellStack(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	layout := stageWorkspace(t, dir, []string{"codex"})
	if !strings.Contains(layout, `pane stacked=true`) {
		t.Fatalf("shell region is not a stack:\n%s", layout)
	}
	if !strings.Contains(layout, `name="shell" close_on_exit=true`) ||
		!strings.Contains(layout, `"shell" "--session-root" "`+dir+`"`) {
		t.Fatal("initial shell does not run qrouton's stack-aware shell command")
	}
	if _, err := os.Stat(filepath.Join(dir, ".qrouton", "status.sh")); !os.IsNotExist(err) {
		t.Fatal("status.sh stamped; utility panes are qrouton subcommands")
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
		"Move focus", "Alt-Tab", "Alt-e", "Research → Plan → Implement", "Alt-n", "open-ended assistant",
		"Alt-g", "New shell", "Switch shells", "Alt-up/down", "Close shell", "Ctrl-d", "Alt-f", "show / hide the overlay layer",
		"User popups", "the picker and this reference are the two exceptions",
		"Dismiss a popup", "agent-opened panes", "Workspace layout", "protected", "only the shell and dock stacks can grow", "Detach", "Ctrl-g o d", "Alt-+ / Alt--", "Alt-?",
		"Ctrl-g Ctrl-q", "Press Esc to close",
		// The richer panel also explains the workspace itself, not just its keys.
		"Scroll a pane", "Ctrl-g s", "the agent you are talking to", "shell stack", "minimised agent panes and subagent activity",
		"run it in a pane", "qrouton.json", "src/<repo>", "thoughts/shared",
	} {
		if !strings.Contains(string(help), want) {
			t.Fatalf("help panel missing %q", want)
		}
	}
	// The panel owns no wait of its own: it hands off to the shared script, by
	// $0's directory, which is the whole reason the two are staged together.
	if !strings.Contains(string(help), `exec sh "$(dirname "$0")/dismiss.sh"`) {
		t.Fatal("quick-reference panel does not hand its dismissal to the shared Esc wait")
	}
	if strings.Contains(string(help), "dd bs=1 count=1") {
		t.Fatal("quick-reference panel still reads keys itself; the shared script owns that")
	}
	config, err := os.ReadFile(filepath.Join(dir, ".qrouton", "zellij-config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`bind "Alt g"`, `Run "/bin/qrouton" "shell" "--session-root" "` + dir + `"`, `stacked true`, `close_on_exit true`,
		`bind "Alt e"`, `"pick" "--session-root" "` + dir + `"`, `bind "Alt n"`, `"mode" "--session-root" "` + dir + `" "--shell-stack" "assistant"`, `bind "Alt tab"`,
		// Shifted Alt chords carry both spellings; a Kitty-protocol terminal
		// reports the unshifted keycap plus Shift.
		`bind "Alt ?" "Alt Shift /"`, `bind "Alt +" "Alt Shift ="`, `"sh" "` + filepath.Join(configHome, "qrouton", "help.sh") + `"`, "mouse_mode true", "session_serialization false"} {
		if !strings.Contains(string(config), want) {
			t.Fatalf("Zellij config missing %q", want)
		}
	}
	for _, forbidden := range []string{"NewPane", "NewTab", "BreakPane", "MovePane", `name "qrouton · terminal"`, "\n    zellij:link"} {
		if strings.Contains(string(config), forbidden) {
			t.Fatalf("Zellij config still exposes layout-changing action %q", forbidden)
		}
	}
	if got := strings.Count(string(config), "floating true"); got != 2 {
		t.Fatalf("user keybindings create %d floating panes, want only quick reference and escalation picker", got)
	}
	// Pane closure belongs to the pane, not to Zellij: a session-wide
	// CloseFocus action cannot distinguish a dismissible overlay from one of
	// qrouton's permanent panes. Esc belongs to transient panes themselves,
	// Ctrl-d to shells, and the editor closes when its process exits.
	if strings.Contains(string(config), `bind "esc" { CloseFocus`) {
		t.Fatal("Esc still closes the focused pane; that is the editor pane's own key")
	}
	if strings.Contains(string(config), "CloseFocus") {
		t.Fatal("a session-wide binding can still close a permanent workspace pane")
	}
	// Nothing switches input mode on qrouton's behalf any more: the modes are
	// the user's Ctrl-g gateway, not a per-pane state register a poller drives.
	if !strings.Contains(string(config), `bind "Ctrl g" { SwitchToMode "tmux"; }`) ||
		strings.Contains(string(config), `SwitchToMode "normal"`) {
		t.Fatal("the keyboard can still reach normal mode, which no longer means anything")
	}
	if !strings.Contains(layout, `pane split_direction="vertical" size=6`) {
		t.Fatal("status panes are not fixed at six rows")
	}
	if !strings.Contains(layout, `pane name="dock"`) || !strings.Contains(layout, `pane name="agents"`) {
		t.Fatal("dock and agent status panes are not side by side")
	}
	if !strings.Contains(layout, `command "/bin/qrouton"`) || !strings.Contains(layout, `args "dock"`) {
		t.Fatal("dock pane does not run the qrouton dock subcommand")
	}
	if strings.Contains(layout, `pane name="repos"`) || strings.Contains(layout, `args "repos"`) {
		t.Fatal("retired repos watcher is still in the workspace layout")
	}
	if !strings.Contains(layout, `"agent" "--session-root" "`+dir+`"`) || !strings.Contains(layout, `"--runner" "codex"`) {
		t.Fatal("agent pane does not run the qrouton agent supervisor")
	}
	if !strings.Contains(layout, `"--mux-json"`) || !strings.Contains(layout, `"--editor-json"`) {
		t.Fatal("supervisor argv lacks the handle/editor exec-boundary flags")
	}
	// A floating pane declared in the layout is sized against the clientless
	// server's default viewport, which is what made the panel come up squished.
	// The supervisor spawns it from inside the attached session instead.
	if strings.Contains(layout, `floating_panes`) || strings.Contains(layout, helpPaneName) {
		t.Fatalf("quick-reference panel is back in the staged layout; it would come up squished:\n%s", layout)
	}
	if !strings.Contains(layout, "session_name \"test-session\"") || !strings.Contains(layout, "attach_to_session true") {
		t.Fatal("layout does not name and self-attach the session")
	}
}

// TestHelpSpawnCarriesTheCodexWarningOnlyAtStartup covers both directions of
// help.sh's $1: the startup panel carries it for a shallow-depth Codex argv,
// and the Alt-? binding never does — that's a launch-time-only concern, not
// something to re-warn about on every re-summon.
func TestHelpSpawnCarriesTheCodexWarningOnlyAtStartup(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("CODEX_HOME", t.TempDir()) // no config.toml: Codex's own default (1) is under the required depth
	dir := t.TempDir()
	stageWorkspace(t, dir, []string{"codex"})

	warning := codexWarning([]string{"codex"})
	if !strings.Contains(warning, "agents.max_depth is under 2") || !strings.Contains(warning, "Set it to 3") {
		t.Fatalf("shallow Codex argv produced no depth warning: %q", warning)
	}
	startup := HelpSpawn(dir, warning)
	if len(startup.Command) != 3 || startup.Command[2] != warning {
		t.Fatalf("startup panel does not carry the warning as help.sh's $1: %v", startup.Command)
	}
	if resummoned := HelpSpawn(dir, ""); len(resummoned.Command) != 2 {
		t.Fatalf("re-summoned panel carries an argument; the warning is launch-time only: %v", resummoned.Command)
	}
	config, err := os.ReadFile(filepath.Join(dir, ".qrouton", "zellij-config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "agents.max_depth is under 2") {
		t.Fatal("Alt-? binding carries the Codex warning; it is a launch-time-only concern")
	}
}

// TestHelpSpawnMirrorsTheAltQuestionBinding is the drift guard between the two
// routes to the panel: the keybinding's geometry and pane name are written by
// hand in the vendored config, and HelpSpawn's are Go values.
func TestHelpSpawnMirrorsTheAltQuestionBinding(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	stageWorkspace(t, dir, []string{"codex"})
	config, err := os.ReadFile(filepath.Join(dir, ".qrouton", "zellij-config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	opts := HelpSpawn(dir, "")
	for _, want := range []string{
		`x "` + opts.Geometry.X + `"`, `y "` + opts.Geometry.Y + `"`,
		`width "` + opts.Geometry.Width + `"`, `height "` + opts.Geometry.Height + `"`,
		`name "` + opts.Label + `"`,
	} {
		if !strings.Contains(string(config), want) {
			t.Fatalf("Alt-? binding has drifted from HelpSpawn; missing %q", want)
		}
	}
	if opts.Cwd != dir {
		t.Fatalf("panel cwd is %q, not the session root; help.sh reads ./qrouton.json for the mode tagline", opts.Cwd)
	}
	if !opts.Focus || !opts.CloseOnExit {
		t.Fatal("panel must take focus and close itself; it is dismissed with a keypress")
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
	// status-bar, for the keybinding hints; compact-bar showed the mode alone,
	// which told the user nothing about what the mode could do. It sits at the
	// top — qrouton's own strip still owns the bottom row.
	if !strings.Contains(layout, "zellij:status-bar") {
		t.Fatal("status-bar dropped; the keybinding hints go with it")
	}
}

func TestCodexWarningHiddenAtDepthTwo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[agents]\nmax_depth = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if warning := codexWarning([]string{"codex"}); warning != "" {
		t.Fatalf("Codex depth warning returned at max_depth 2: %q", warning)
	}
	if warning := codexWarning([]string{"claude"}); warning != "" {
		t.Fatalf("non-Codex runner warned about Codex depth: %q", warning)
	}
}

func TestWriteSupportStampsHelpScriptUnderTheConfigDir(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("CODEX_HOME", t.TempDir())
	dirA, dirB := t.TempDir(), t.TempDir()
	if err := writeSupport(dirA); err != nil {
		t.Fatal(err)
	}
	if err := writeSupport(dirB); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configHome, "qrouton", "help.sh")); err != nil {
		t.Fatal("help script missing from the config dir:", err)
	}
	if _, err := os.Stat(filepath.Join(dirA, ".qrouton", "help.sh")); !os.IsNotExist(err) {
		t.Fatal("help.sh stamped into a session directory; one global copy was the point")
	}
}

// The shared Esc wait is staged beside the help panel, executable, and in the
// same directory — help.sh reaches it by $0's dirname, so "beside" is load
// bearing rather than tidy.
func TestWriteSupportStampsDismissScriptBesideTheHelpPanel(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	if err := writeSupport(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(config.DismissScriptPath())
	if err != nil {
		t.Fatal("dismiss script missing from the config dir:", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("dismiss script is not executable: %v", info.Mode())
	}
	if got, want := filepath.Dir(config.DismissScriptPath()), filepath.Dir(config.HelpScriptPath()); got != want {
		t.Fatalf("dismiss script lives in %q, not beside help.sh in %q; help.sh finds it by $0", got, want)
	}
	body, err := os.ReadFile(config.DismissScriptPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"033",                // Esc, and only Esc, ends the wait
		"od -An -b",          // octal, so an empty read stays distinct from a key that is not Esc
		"min 1 time 0",       // an indefinite wait blocks rather than polling
		"min 0 time 5",       // a timed one ticks, so the deadline is checked
		`[ -z "$rest" ]`,     // an arrow key is Esc plus more bytes, and must not dismiss
		`[ -z "$deadline" ]`, // a closed stdin must not strand the pane
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("dismiss script missing %q", want)
		}
	}
}

// Esc dismisses; the script exits so close_on_exit can take the pane with it.
// This runs the real script over a pipe rather than a tty, which is enough to
// pin that an Esc byte ends the wait and that the exit status is clean.
func TestDismissScriptExitsOnEsc(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	if err := writeSupport(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shellBin, config.DismissScriptPath(), "600")
	cmd.Stdin = strings.NewReader("\033")
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("dismiss script exited non-zero on Esc: %v", err)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("dismiss script did not exit on Esc; it waited out its timeout instead")
	}
}

// DismissCommand is the single fragment every dismissible pane ends with, so
// the callers in mcpserver cannot each grow their own dialect of "Esc closes
// this".
func TestDismissCommandNamesTheSharedScriptAndItsTimeout(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	indefinite := DismissCommand(0)
	if !strings.Contains(indefinite, ShellQuote(config.DismissScriptPath())) {
		t.Fatalf("DismissCommand does not run the staged script: %q", indefinite)
	}
	if strings.HasSuffix(indefinite, " 0") {
		t.Fatalf("an indefinite wait passed a zero timeout: %q", indefinite)
	}
	if timed := DismissCommand(8); !strings.HasSuffix(timed, " 8") {
		t.Fatalf("DismissCommand(8) does not pass the timeout in seconds: %q", timed)
	}
}

func TestWriteSupportStampsNotifyScript(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	if err := writeSupport(dir); err != nil {
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
