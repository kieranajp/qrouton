package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

// The one line the user gets back has to name something they recognise and a log
// they can open.
func TestOpenedLineNamesTheSessionAndItsLog(t *testing.T) {
	chosen := launch.WorkbenchSpec{SessionRoot: "/sessions/api-web", Socket: "/tmp/qrouton-sock/501/ab.sock"}
	line := fmt.Sprintf(openedFormat, subject(chosen.SessionRoot), workbenchLog(chosen))
	if !strings.Contains(line, "api-web") || !strings.Contains(line, "/sessions/api-web/.qrouton/workbench.log") {
		t.Fatalf("line = %q, want the session name and its log", line)
	}

	empty := launch.WorkbenchSpec{Socket: "/tmp/qrouton-sock/501/ab.sock"}
	line = fmt.Sprintf(openedFormat, subject(empty.SessionRoot), workbenchLog(empty))
	if !strings.Contains(line, noSessionSubject) || !strings.Contains(line, "/tmp/qrouton-sock/501/ab.log") {
		t.Fatalf("line = %q, want an empty workbench and a log beside its socket", line)
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
		argv, _, err := agent(root, socket, "", false)
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

	err := open(rootContext(t))
	if err == nil {
		t.Fatal("qrouton opened a session with a workbench already open")
	}
	if !strings.Contains(err.Error(), "+ New session") {
		t.Fatalf("refusal %q does not point at the button in the running window", err)
	}
}

// The binary's session-facing surface is qrouton alone, so an argument left over
// from the paths that took one has to say so rather than opening a window.
func TestArgumentsAreRefusedRatherThanIgnored(t *testing.T) {
	err := open(rootContext(t, "lifesum/api"))
	if !errors.Is(err, errNoSessionArguments) {
		t.Fatalf("qrouton lifesum/api = %v, want %v", err, errNoSessionArguments)
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

// A session recorded against an agent that has since left the machine must say
// so. Substituting whatever is installed would run the work under a runner the
// session was never assembled for, silently.
func TestTheAgentCommandRefusesARunnerThatIsGone(t *testing.T) {
	cfg := &config.Config{Launch: map[string][]string{"codex": {"/bin/echo"}}}
	agent := agentCommand(cfg, "/bin/qrouton", "codex", launch.EditorCommand{Argv: []string{"vi"}})

	_, _, err := agent(t.TempDir(), "/tmp/s.sock", "no-such-agent", false)
	if !errors.Is(err, launch.ErrRunnerUnavailable) {
		t.Fatalf("booting a session against a missing agent = %v, want %v", err, launch.ErrRunnerUnavailable)
	}
	if err != nil && !strings.Contains(err.Error(), "no-such-agent") {
		t.Fatalf("refusal %q does not name the agent the session recorded", err)
	}
}
