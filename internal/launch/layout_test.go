package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// The handle and the editor reach the MCP child only through the supervisor's
// argv, so a missing or misspelled flag here is a tool call that fails much
// later, at the agent's first window.
func TestLaunchReturnsTheSupervisorArgvAndEditorEnvironment(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := t.TempDir()
	runner := Runner{ID: "codex", Command: []string{"codex"}}
	editor := EditorCommand{Argv: []string{"vi"}}
	socket := "/tmp/qrouton/501/deadbeef.sock"

	argv, env, err := Launch(dir, runner, "/bin/qrouton", socket, editor, false)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"/bin/qrouton agent", "--session-root " + dir, "--runner codex", "--workbench-json", "--editor-json"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("supervisor argv %q missing %q", joined, want)
		}
	}
	if strings.Contains(joined, "--mux-json") {
		t.Fatalf("supervisor argv still spells the handle flag the old way: %q", joined)
	}
	if strings.Contains(joined, "--resume") {
		t.Fatalf("fresh launch asked the supervisor to resume: %q", joined)
	}
	handle, err := workbench.ParseHandle(argv[flagValue(argv, "--workbench-json")])
	if err != nil {
		t.Fatal(err)
	}
	if handle.Socket != socket || handle.SessionRoot != dir {
		t.Fatalf("handle = %#v, want the socket %q rooted at %q", handle, socket, dir)
	}
	if !contains(env, EditorEnvVar+"="+editor.Marshal()) {
		t.Fatalf("environment does not carry the resolved editor: %v", env)
	}
}

func TestLaunchAsksTheSupervisorToResume(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	argv, _, err := Launch(t.TempDir(), Runner{ID: "codex", Command: []string{"codex"}}, "/bin/qrouton",
		"/tmp/qrouton/501/deadbeef.sock", EditorCommand{Argv: []string{"vi"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(argv, "--resume") {
		t.Fatalf("resumed launch lacks --resume: %v", argv)
	}
}

// Onboarding runs as the conversation terminal's own child, so the socket it
// adopts the chosen session on has to reach it through this argv.
func TestOnboardArgvCarriesTheSocketAndTheLaunchFlags(t *testing.T) {
	socket := "/tmp/qrouton-sock/501/deadbeef.sock"
	argv := OnboardArgv("/bin/qrouton", socket, "codex", true)
	if joined := strings.Join(argv, " "); joined != "/bin/qrouton onboard --socket "+socket+" --runner codex --refresh" {
		t.Fatalf("onboard argv = %q", joined)
	}
	bare := strings.Join(OnboardArgv("/bin/qrouton", socket, "", false), " ")
	if strings.Contains(bare, "--runner") || strings.Contains(bare, "--refresh") {
		t.Fatalf("onboard argv invents flags nobody asked for: %q", bare)
	}
}

// Onboarding in a pane has no terminal to hand over, so it has to be told to
// stop at adoption and let the workbench boot the agent.
func TestOnboardPaneArgvAdoptsWithoutHandingOver(t *testing.T) {
	socket := "/tmp/qrouton-sock/501/deadbeef.sock"
	joined := strings.Join(OnboardPaneArgv("/bin/qrouton", socket), " ")
	if joined != "/bin/qrouton onboard --socket "+socket+" --adopt-only" {
		t.Fatalf("onboard pane argv = %q", joined)
	}
}

func TestShellArgvRootsTheShellInTheSession(t *testing.T) {
	if joined := strings.Join(ShellArgv("/bin/qrouton", "/sessions/octopus"), " "); joined != "/bin/qrouton shell --session-root /sessions/octopus" {
		t.Fatalf("shell argv = %q", joined)
	}
}

func flagValue(argv []string, flag string) int {
	for i, arg := range argv {
		if arg == flag && i+1 < len(argv) {
			return i + 1
		}
	}
	return 0
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
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
