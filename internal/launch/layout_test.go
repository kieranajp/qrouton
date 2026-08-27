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

// The launcher stamps the zero editor when it could not resolve one, and the
// supervisor it stamps for must read that as having no editor rather than as a
// broken argument — every session used to die on this.
func TestLaunchStampsAnAbsentEditorTheSupervisorCanRead(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	argv, env, err := Launch(t.TempDir(), Runner{ID: "codex", Command: []string{"codex"}}, "/bin/qrouton",
		"/tmp/qrouton/501/deadbeef.sock", EditorCommand{}, false)
	if err != nil {
		t.Fatal(err)
	}
	stamped := argv[flagValue(argv, editorJSONFlag)]
	editor, err := ParseEditor(stamped)
	if err != nil {
		t.Fatalf("the supervisor cannot read the stamped editor %q: %v", stamped, err)
	}
	if len(editor.Argv) != 0 {
		t.Fatalf("stamped editor = %#v, want none", editor)
	}
	if !contains(env, EditorEnvVar+"="+stamped) {
		t.Fatalf("environment does not carry the absent editor: %v", env)
	}
}

func TestShellArgvRootsTheShellInTheSession(t *testing.T) {
	if joined := strings.Join(ShellArgv("/bin/qrouton", "/sessions/octopus"), " "); joined != "/bin/qrouton shell --session-root /sessions/octopus" {
		t.Fatalf("shell argv = %q", joined)
	}
}

// A cleaned path would name the wrong directory for a session whose root came
// through a symlink, and quoting it is a directory Finder cannot find.
func TestRevealArgvSelectsTheDirectoryVerbatim(t *testing.T) {
	dir := "/sessions/octopus/../octopus/"
	if got := RevealArgv(dir); len(got) != 3 || got[0] != "open" || got[1] != "-R" || got[2] != dir {
		t.Fatalf("reveal argv = %q, want open -R %q", got, dir)
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
