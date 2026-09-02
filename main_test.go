package main

import (
	"bufio"
	"encoding/json"
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
	"github.com/kieranajp/qrouton/internal/desktop"
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
	ports := testPorts(cfg, "codex", launch.EditorCommand{Argv: []string{"vi"}})

	sockets := map[string]string{}
	for _, slug := range []string{"alpha", "beta"} {
		root := t.TempDir()
		socket := "/tmp/qrouton-sock/501/" + slug + ".sock"
		command, err := ports.Agent(desktop.AgentRequest{SessionRoot: root, Socket: socket})
		if err != nil {
			t.Fatal(err)
		}
		handle, err := workbench.ParseHandle(flagValue(t, command.Argv, "--workbench-json"))
		if err != nil {
			t.Fatal(err)
		}
		if handle.Socket != socket || handle.SessionRoot != root {
			t.Fatalf("%s's supervisor got %#v, want the socket %q rooted at %q", slug, handle, socket, root)
		}
		if command.RunnerID != "codex" {
			t.Fatalf("%s resolved provider = %q, want codex", slug, command.RunnerID)
		}
		sockets[slug] = handle.Socket
	}
	if sockets["alpha"] == sockets["beta"] {
		t.Fatalf("both sessions' agents were pointed at %q", sockets["alpha"])
	}
}

func TestTheAgentCommandReturnsTheProviderResolvedForALegacyManifest(t *testing.T) {
	cfg := &config.Config{Launch: map[string][]string{
		"claude": {"/bin/echo"}, "codex": {"/bin/echo"}, "opencode": {"/bin/echo"},
	}}
	ports := testPorts(cfg, "", launch.EditorCommand{})
	command, err := ports.Agent(desktop.AgentRequest{SessionRoot: t.TempDir(), Socket: "/tmp/s.sock"})
	if err != nil {
		t.Fatal(err)
	}
	if command.RunnerID != "claude" {
		t.Fatalf("legacy manifest resolved provider = %q, want the first installed provider", command.RunnerID)
	}
}

// Every path that opens a workbench opens it, editor or no editor: resuming a
// session with nothing on PATH to edit with gets a window, not a terminal error.
func TestOpeningASessionWithNoEditorStillOpensTheWorkbench(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	previous := detachProcess
	defer func() { detachProcess = previous }()
	var spec launch.WorkbenchSpec
	detachProcess = func(s launch.WorkbenchSpec, _ []string) error { spec = s; return nil }

	dir := t.TempDir()
	if err := launchRunner(&config.Config{}, dir, launch.Runner{ID: "codex"}, true); err != nil {
		t.Fatalf("resuming a session with no editor: %v", err)
	}
	if spec.SessionRoot != dir || !spec.Resume {
		t.Fatalf("workbench spec = %#v, want %q resumed", spec, dir)
	}
	if len(spec.Editor.Argv) != 0 {
		t.Fatalf("spec editor = %#v, want none", spec.Editor)
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

func TestLinearIssueColdLaunchCarriesTheCanonicalTicketAndPrompt(t *testing.T) {
	t.Setenv("QROUTON_ROOT", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LINEAR_PROMPT", "Fix the login regression")
	previous := detachProcess
	previousDiscovery := discoverProcess
	defer func() { detachProcess, discoverProcess = previous, previousDiscovery }()
	discoverProcess = func() workbench.Discovery { return workbench.Discovery{} }
	var got launch.WorkbenchSpec
	detachProcess = func(spec launch.WorkbenchSpec, _ []string) error {
		got = spec
		return nil
	}

	if err := open(rootContext(t, "--linear-issue", "lif-2841")); err != nil {
		t.Fatal(err)
	}
	if got.LinearIssue != "https://linear.app/issue/LIF-2841" || got.LinearPrompt != "Fix the login regression" ||
		got.SessionRoot != "" || got.Socket == "" {
		t.Fatalf("cold workbench spec = %+v", got)
	}
}

func TestLinearIssueRejectsInvalidOrExtraInputBeforeLaunch(t *testing.T) {
	previous := detachProcess
	defer func() { detachProcess = previous }()
	called := false
	detachProcess = func(launch.WorkbenchSpec, []string) error { called = true; return nil }
	for _, args := range [][]string{
		{"--linear-issue", ""},
		{"--linear-issue", "not-an-issue"},
		{"--linear-issue", "LIF-2841", "extra"},
	} {
		if err := open(rootContext(t, args...)); err == nil {
			t.Fatalf("qrouton %v succeeded", args)
		}
	}
	if called {
		t.Fatal("invalid Linear input reached detach")
	}
}

func TestLinearIssueUsesThePublishedProcessEndpoint(t *testing.T) {
	t.Setenv("LINEAR_PROMPT", "Fix the warm path")
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
	previousDiscovery := discoverProcess
	defer func() { discoverProcess = previousDiscovery }()
	discoverProcess = func() workbench.Discovery { return workbench.Discovery{Socket: socket} }
	requests := make(chan workbench.Request, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			line, err := bufio.NewReader(conn).ReadBytes('\n')
			if err == nil {
				var req workbench.Request
				if json.Unmarshal(line, &req) == nil {
					requests <- req
					body, _ := json.Marshal(workbench.Response{Outcome: "queued"})
					_, _ = conn.Write(append(body, '\n'))
				}
			}
			_ = conn.Close()
		}
	}()

	if err := open(rootContext(t, "--linear-issue", "lif-2841")); err != nil {
		t.Fatal(err)
	}
	req := <-requests
	if req.Op != workbench.OpOpenLinearIssue || req.LinearIssue == nil ||
		req.LinearIssue.Ticket != "https://linear.app/issue/LIF-2841" || req.LinearIssue.Prompt != "Fix the warm path" {
		t.Fatalf("live request = %+v", req)
	}
}

func TestLinearIssueRefusesALegacyRunningWorkbench(t *testing.T) {
	previousDiscovery := discoverProcess
	defer func() { discoverProcess = previousDiscovery }()
	discoverProcess = func() workbench.Discovery { return workbench.Discovery{Legacy: true} }
	if err := open(rootContext(t, "--linear-issue", "LIF-2841")); !errors.Is(err, errLegacyWorkbench) {
		t.Fatalf("legacy workbench refusal = %v, want %v", err, errLegacyWorkbench)
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

	got, ok := session.Preferred(root, sessions)
	if !ok || got.Slug != "kraken" {
		t.Fatalf("lastShown = %q, %v; want kraken, the last one shown", got.Slug, ok)
	}
}

func TestLastShownFallsBackToTheNewestSession(t *testing.T) {
	sessions := []session.Manifest{
		{Slug: "octopus", CreatedAt: time.Now().Add(-48 * time.Hour)},
		{Slug: "kraken", CreatedAt: time.Now()},
	}
	got, ok := session.Preferred(t.TempDir(), sessions)
	if !ok || got.Slug != "kraken" {
		t.Fatalf("lastShown with nothing stamped = %q, %v; want the newest session kraken", got.Slug, ok)
	}
}

// Nothing to come back to opens the landing list instead.
func TestLastShownReportsNothingWithoutASession(t *testing.T) {
	if got, ok := session.Preferred(t.TempDir(), nil); ok {
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
	if d := workbench.Discover(); d.Socket == "" && !d.Legacy {
		t.Fatal("a workbench listening on its socket is not discovered")
	}
}

// rootContext is the root action's arguments as the CLI would hand them over.
func rootContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet(appName, flag.ContinueOnError)
	set.String(runnerFlag, "", "")
	set.String(linearIssueFlag, "", "")
	set.String(ticketFlag, "", "")
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
	ports := testPorts(cfg, "codex", launch.EditorCommand{Argv: []string{"vi"}})

	_, err := ports.Agent(desktop.AgentRequest{
		SessionRoot: t.TempDir(), Socket: "/tmp/s.sock", RunnerID: "no-such-agent",
	})
	if !errors.Is(err, launch.ErrRunnerUnavailable) {
		t.Fatalf("booting a session against a missing agent = %v, want %v", err, launch.ErrRunnerUnavailable)
	}
	if err != nil && !strings.Contains(err.Error(), "no-such-agent") {
		t.Fatalf("refusal %q does not name the agent the session recorded", err)
	}
}

// testPorts is the workbench's launch adapter, as workbenchProcess builds it.
func testPorts(cfg *config.Config, runner string, editor launch.EditorCommand) workbenchPorts {
	return workbenchPorts{cfg: cfg, bin: "/bin/qrouton",
		spec: launch.WorkbenchSpec{Runner: runner, Editor: editor}}
}

func TestTicketFlagCarriesTheCanonicalReferenceAndNoPrompt(t *testing.T) {
	t.Setenv("QROUTON_ROOT", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// LINEAR_PROMPT belongs to Linear Desktop's own request. A ticket offered
	// from a terminal carries none, and must not inherit a stale one.
	t.Setenv("LINEAR_PROMPT", "Fix the login regression")
	previous := detachProcess
	previousDiscovery := discoverProcess
	defer func() { detachProcess, discoverProcess = previous, previousDiscovery }()
	discoverProcess = func() workbench.Discovery { return workbench.Discovery{} }
	var got launch.WorkbenchSpec
	detachProcess = func(spec launch.WorkbenchSpec, _ []string) error {
		got = spec
		return nil
	}

	for _, tc := range []struct{ raw, want string }{
		{"https://github.com/Acme/API/issues/42", "https://github.com/acme/api/issues/42"},
		{"https://app.asana.com/0/123/456", "https://app.asana.com/0/123/456"},
		{"lif-2841", "https://linear.app/issue/LIF-2841"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got = launch.WorkbenchSpec{}
			if err := open(rootContext(t, "--ticket", tc.raw)); err != nil {
				t.Fatal(err)
			}
			if got.LinearIssue != tc.want || got.LinearPrompt != "" || got.Socket == "" {
				t.Fatalf("cold workbench spec = %+v, want ticket %q and no prompt", got, tc.want)
			}
		})
	}
}

func TestLinearIssueOutranksTicketAndKeepsItsPrompt(t *testing.T) {
	t.Setenv("QROUTON_ROOT", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LINEAR_PROMPT", "Fix the login regression")
	previous := detachProcess
	previousDiscovery := discoverProcess
	defer func() { detachProcess, discoverProcess = previous, previousDiscovery }()
	discoverProcess = func() workbench.Discovery { return workbench.Discovery{} }
	var got launch.WorkbenchSpec
	detachProcess = func(spec launch.WorkbenchSpec, _ []string) error {
		got = spec
		return nil
	}

	if err := open(rootContext(t, "--linear-issue", "lif-2841",
		"--ticket", "https://github.com/acme/api/issues/42")); err != nil {
		t.Fatal(err)
	}
	if got.LinearIssue != "https://linear.app/issue/LIF-2841" ||
		got.LinearPrompt != "Fix the login regression" {
		t.Fatalf("spec = %+v, want Linear Desktop's request to win", got)
	}
}

func TestTicketRejectsAnUnownedReferenceBeforeLaunch(t *testing.T) {
	previous := detachProcess
	defer func() { detachProcess = previous }()
	called := false
	detachProcess = func(launch.WorkbenchSpec, []string) error { called = true; return nil }
	for _, args := range [][]string{
		{"--ticket", ""},
		{"--ticket", "not-a-ticket"},
		{"--ticket", "https://github.com/acme/api/pull/42"},
		{"--ticket", "https://example.com/issues/42"},
	} {
		if err := open(rootContext(t, args...)); err == nil {
			t.Fatalf("qrouton %v succeeded", args)
		}
	}
	if called {
		t.Fatal("an unowned ticket reached detach")
	}
}
