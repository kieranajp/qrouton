package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

func TestParseRepoSpec(t *testing.T) {
	cases := []struct {
		in          string
		owner, name string
		wantErr     bool
	}{
		{in: "kieranajp/qrouton", owner: "kieranajp", name: "qrouton"},
		{in: "  lifesum/lifesum-ios  ", owner: "lifesum", name: "lifesum-ios"},
		{in: "kieranajp/qrouton.git", owner: "kieranajp", name: "qrouton"},
		{in: "kieranajp/qrouton/", owner: "kieranajp", name: "qrouton"},
		{in: "qrouton", wantErr: true},
		{in: "a/b/c", wantErr: true},
		{in: "/qrouton", wantErr: true},
		{in: "kieranajp/", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		owner, name, err := parseRepoSpec(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseRepoSpec(%q) = (%q,%q), want error", tc.in, owner, name)
			}
			continue
		}
		if err != nil || owner != tc.owner || name != tc.name {
			t.Errorf("parseRepoSpec(%q) = (%q,%q,%v), want (%q,%q,nil)", tc.in, owner, name, err, tc.owner, tc.name)
		}
	}
}

// Assembly happens before the workbench is handed over, so its progress has to
// reach the terminal the user is still watching.
func TestProgressReachesTheParentsTerminal(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = write
	printProgress(session.Progress{Status: session.ProgressCompleted, Step: "cloned",
		Repo: &github.Repo{Org: "kieranajp", Name: "qrouton"}})
	printProgress(session.Progress{Status: session.ProgressAdvanced, Step: "fetching",
		Repo: &github.Repo{Org: "kieranajp", Name: "qrouton"}})
	os.Stderr = saved
	_ = write.Close()

	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "kieranajp/qrouton cloned") {
		t.Fatalf("stderr = %q, want the completed step", out)
	}
	if strings.Contains(string(out), "fetching") {
		t.Fatalf("stderr = %q, want outcomes only", out)
	}
}

// The one line the user gets back has to name something they recognise and a log
// they can open.
func TestOpenedLineNamesTheSessionAndItsLog(t *testing.T) {
	chosen := launch.WorkbenchSpec{SessionRoot: "/sessions/api-web", Socket: "/tmp/qrouton-sock/501/ab.sock"}
	line := fmt.Sprintf(openedFormat, subject(chosen.SessionRoot), workbenchLog(chosen))
	if !strings.Contains(line, "api-web") || !strings.Contains(line, "/sessions/api-web/.qrouton/workbench.log") {
		t.Fatalf("line = %q, want the session name and its log", line)
	}

	landing := launch.WorkbenchSpec{Socket: "/tmp/qrouton-sock/501/ab.sock"}
	line = fmt.Sprintf(openedFormat, subject(landing.SessionRoot), workbenchLog(landing))
	if !strings.Contains(line, sessionListSubject) || !strings.Contains(line, "/tmp/qrouton-sock/501/ab.log") {
		t.Fatalf("line = %q, want the session list and a log beside its socket", line)
	}
}

// The agent learns its own address only through the command the workbench builds
// as it boots that session.
func TestTheAgentCommandCarriesEachSessionsOwnSocket(t *testing.T) {
	cfg := &config.Config{Launch: map[string][]string{"codex": {"/bin/echo"}}}
	agent := agentCommand(cfg, "/bin/qrouton", "codex", launch.EditorCommand{Argv: []string{"vi"}})

	sockets := map[string]string{}
	for _, slug := range []string{"alpha", "beta"} {
		root := t.TempDir()
		socket := "/tmp/qrouton-sock/501/" + slug + ".sock"
		argv, _, err := agent(root, socket, false)
		if err != nil {
			t.Fatal(err)
		}
		handle, err := workbench.ParseHandle(flagValue(t, argv, "--workbench-json"))
		if err != nil {
			t.Fatal(err)
		}
		if handle.Socket != socket || handle.SessionRoot != root {
			t.Fatalf("%s's supervisor got %#v, want the socket %q rooted at %q", slug, handle, socket, root)
		}
		sockets[slug] = handle.Socket
	}
	if sockets["alpha"] == sockets["beta"] {
		t.Fatalf("both sessions' agents were pointed at %q", sockets["alpha"])
	}
}

// Two workbenches would each believe they were the only one holding a session's
// supervisor, so every path that opens on one refuses — bare qrouton included —
// and says where a session is started instead.
func TestOpeningASessionRefusesWhileAWorkbenchIsUp(t *testing.T) {
	root := t.TempDir()
	t.Setenv("QROUTON_ROOT", root)
	// No config and nothing on PATH, so a broken guard stops at an unresolvable
	// runner instead of detaching a second workbench out of the test binary.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(root, "octopus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(dir, session.Manifest{Slug: "octopus", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	answerOnAWorkbenchSocket(t)

	// A repository nobody can resolve, so a broken guard fails this test rather
	// than assembling a session.
	for _, args := range [][]string{{}, {"kieranajp/qrouton-no-such-repo"}, {t.TempDir()}} {
		err := onboard(rootContext(t, args...))
		if err == nil {
			t.Fatalf("qrouton %v opened a session with a workbench already open", args)
		}
		if !strings.Contains(err.Error(), "+ New session") {
			t.Fatalf("refusal %q for qrouton %v does not point at the button in the running window", err, args)
		}
	}
}

// Opening the app comes back to the session you were last in, not the newest one
// on disk.
func TestLastShownPrefersTheMostRecentlyStampedSession(t *testing.T) {
	root := t.TempDir()
	sessions := []session.Manifest{
		{Slug: "octopus", CreatedAt: time.Now().Add(-72 * time.Hour)},
		{Slug: "kraken", CreatedAt: time.Now().Add(-48 * time.Hour)},
		{Slug: "squid", CreatedAt: time.Now()},
	}
	stampSession(t, root, "octopus", time.Now().Add(-2*time.Hour))
	stampSession(t, root, "kraken", time.Now().Add(-time.Hour))

	got, ok := lastShown(root, sessions)
	if !ok || got.Slug != "kraken" {
		t.Fatalf("lastShown = %q, %v; want kraken, the last one shown", got.Slug, ok)
	}
}

func TestLastShownFallsBackToTheNewestSession(t *testing.T) {
	sessions := []session.Manifest{
		{Slug: "octopus", CreatedAt: time.Now().Add(-48 * time.Hour)},
		{Slug: "kraken", CreatedAt: time.Now()},
	}
	got, ok := lastShown(t.TempDir(), sessions)
	if !ok || got.Slug != "kraken" {
		t.Fatalf("lastShown with nothing stamped = %q, %v; want the newest session kraken", got.Slug, ok)
	}
}

// Nothing to come back to opens the landing list instead.
func TestLastShownReportsNothingWithoutASession(t *testing.T) {
	if got, ok := lastShown(t.TempDir(), nil); ok {
		t.Fatalf("lastShown named %q under a root holding no sessions", got.Slug)
	}
}

func stampSession(t *testing.T, root, slug string, at time.Time) {
	t.Helper()
	if err := session.MarkOpened(filepath.Join(root, slug), at); err != nil {
		t.Fatal(err)
	}
}

// answerOnAWorkbenchSocket stands a workbench up as far as Running() can tell: an
// address under the per-uid directory it scans, with something on the other end.
func answerOnAWorkbenchSocket(t *testing.T) {
	t.Helper()
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(socket)
	})
	if !workbench.Running() {
		t.Fatal("a workbench listening on its socket is not reported as running")
	}
}

// rootContext is the root action's arguments as the CLI would hand them over.
func rootContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet(appName, flag.ContinueOnError)
	set.Bool(refreshFlag, false, "")
	set.String(runnerFlag, "", "")
	set.String(workbenchSpecFlag, "", "")
	if err := set.Parse(args); err != nil {
		t.Fatal(err)
	}
	return cli.NewContext(cli.NewApp(), set, nil)
}

func flagValue(t *testing.T, argv []string, flag string) string {
	t.Helper()
	for i, arg := range argv {
		if arg == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	t.Fatalf("argv %v carries no %s", argv, flag)
	return ""
}

func TestAdhocName(t *testing.T) {
	single := adhocName([]github.Repo{{Name: "qrouton"}})
	if single != "qrouton" {
		t.Fatalf("single repo name = %q, want qrouton", single)
	}
	multi := adhocName([]github.Repo{{Name: "api"}, {Name: "web"}})
	if multi != "api-web" {
		t.Fatalf("multi repo name = %q, want api-web", multi)
	}
}
