package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// shortEscalatePoll shrinks escalate's poll interval and timeout for the
// duration of a test, restoring them on cleanup.
func shortEscalatePoll(t *testing.T, timeout time.Duration) {
	t.Helper()
	originalTimeout, originalInterval := escalateTimeout, escalatePollInterval
	escalateTimeout, escalatePollInterval = timeout, 5*time.Millisecond
	t.Cleanup(func() { escalateTimeout, escalatePollInterval = originalTimeout, originalInterval })
}

// fakeZellij writes a script that logs every invocation to $CALL_LOG, hands out a
// fresh pane id for each new-pane, and echoes canned output for dump-screen.
func fakeZellij(t *testing.T, dir string) (helper, log string) {
	t.Helper()
	log = filepath.Join(dir, "calls")
	helper = filepath.Join(dir, "zellij")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$CALL_LOG\"\n" +
		"case \"$4\" in\n" +
		"  new-pane) n=$(cat \"$ID_SEQ\" 2>/dev/null || echo 0); n=$((n+1)); echo \"$n\" > \"$ID_SEQ\"; echo \"terminal_$n\";;\n" +
		"  dump-screen) printf 'listening on :3000\\n';;\n" +
		"  list-panes) printf '%s' \"$PANES_JSON\";;\n" +
		"esac\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", log)
	t.Setenv("ID_SEQ", filepath.Join(dir, "idseq"))
	// The real MCP server runs as a child of the runner in the agent pane, so
	// the driver always has an owning pane to hand focus back to. Without this
	// every test here would exercise the no-owner fallback instead.
	t.Setenv("ZELLIJ_PANE_ID", "0")
	return helper, log
}

// livePanes makes list-panes report every id as a live terminal pane, which is
// what the pane registry's liveness check expects to see for a pane it opened.
func livePanes(t *testing.T, ids ...int) {
	t.Helper()
	entries := make([]string, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, fmt.Sprintf(`{"id":%d,"title":"pane"}`, id))
	}
	t.Setenv("PANES_JSON", "["+strings.Join(entries, ",")+"]")
}

func testManager(t *testing.T, dir, helper string) *paneManager {
	t.Helper()
	return newPaneManager(dir, launch.EditorCommand{Argv: []string{"vi", "+{line}", "{path}"}, Template: true}, mux.NewZellijHost(helper, "test-session"))
}

func TestOpenFilePinsPaneReturnsFocusAndReplacesPrevious(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("hello"), 0o644)
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.openFile(context.Background(), openFileInput{Path: "doc.md", Line: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.openFile(context.Background(), openFileInput{Path: "doc.md"}); err != nil {
		t.Fatal(err)
	}
	s := readLog(t, log)
	for _, want := range []string{
		"--session test-session",
		"new-pane --floating --pinned true",
		"--x 66% --y 3% --width 33% --height 94%",
		"Editor · Alt-f to view · quit to close",
		"focus-pane-id terminal_0",        // focus goes back to the agent pane by name, not by flipping the layer
		"close-pane --pane-id terminal_1", // second open replaces the first editor pane
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("calls missing %q:\n%s", want, s)
		}
	}
	// The editor pane is the one floated pane Esc must not touch: it holds the
	// user's real editor, where Esc leaves insert mode. So no shared wait is
	// appended to its command, and its title promises no Esc.
	if strings.Contains(s, "dismiss.sh") {
		t.Fatalf("the editor pane ends in the shared Esc wait; Esc there belongs to the editor:\n%s", s)
	}
	if strings.Contains(s, "Editor") && strings.Contains(s, "Editor · Alt-f to view · Esc") {
		t.Fatalf("the editor pane's title still offers Esc to close:\n%s", s)
	}
}

func TestRunCommandOpensPaneWithResolvedCwd(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "src", "app")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.run(context.Background(), runCommandInput{Command: "npm run dev", Name: "dev", Cwd: "src/app"}); err != nil {
		t.Fatal(err)
	}
	realRepo, _ := filepath.EvalSymlinks(repo)
	s := readLog(t, log)
	for _, want := range []string{
		"new-pane --floating --pinned true",
		"--x 48% --y 8% --width 50% --height 84%",
		"--name ▶ dev · Esc to close",
		"--cwd " + realRepo,
		"-- sh -lc npm run dev; " + launch.DismissCommand(0),
		// Always self-closing: the shared wait is what holds the pane open, so
		// Esc ending that wait has to take the pane with it.
		"--close-on-exit",
		"focus-pane-id terminal_0",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("calls missing %q:\n%s", want, s)
		}
	}
	if got := p.list(); len(got) != 1 || got[0] != "dev" {
		t.Fatalf("registry = %v, want [dev]", got)
	}
}

// Every pane the user is told to dismiss with Esc ends in the identical wait,
// and every one of them closes when that wait returns. This is the guard
// against the four routes drifting back into four dialects of "Esc closes
// this" — which is how the editor pane ended up promising an Esc that belonged
// to nvim.
func TestEveryDismissiblePaneEndsInTheSameEscWait(t *testing.T) {
	footer := launch.DismissCommand(0)
	for _, tc := range []struct {
		name   string
		open   func(*paneManager) error
		footer string
	}{
		{"run_command", func(p *paneManager) error {
			_, err := p.run(context.Background(), runCommandInput{Command: "ls", Name: "cmd"})
			return err
		}, footer},
		{"show_diff", func(p *paneManager) error {
			_, err := p.showDiff(context.Background(), showDiffInput{})
			return err
		}, footer},
		{"notify", func(p *paneManager) error {
			_, err := p.notify(context.Background(), notifyInput{Message: "done"})
			return err
		}, launch.DismissCommand(toastSeconds)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			helper, log := fakeZellij(t, dir)
			p := testManager(t, dir, helper)
			if err := tc.open(p); err != nil {
				t.Fatal(err)
			}
			s := readLog(t, log)
			if !strings.Contains(s, tc.footer) {
				t.Fatalf("pane does not end in the shared Esc wait %q:\n%s", tc.footer, s)
			}
			if !strings.Contains(s, "--close-on-exit") {
				t.Fatalf("pane survives its own Esc wait returning; Esc would not close it:\n%s", s)
			}
			if !strings.Contains(s, "Esc to close") {
				t.Fatalf("pane title does not tell the user which key dismisses it:\n%s", s)
			}
		})
	}
}

func TestRunCommandRejectsCwdEscapeReservedNameAndEmpty(t *testing.T) {
	dir := t.TempDir()
	helper, _ := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.run(context.Background(), runCommandInput{Command: "ls", Cwd: "../"}); err == nil {
		t.Fatal("accepted cwd outside the session")
	}
	if _, err := p.run(context.Background(), runCommandInput{Command: "ls", Name: editorPaneName}); err == nil {
		t.Fatal("accepted reserved editor name")
	}
	if _, err := p.run(context.Background(), runCommandInput{Command: "   "}); err == nil {
		t.Fatal("accepted empty command")
	}
}

func TestReadPaneDumpsScreenForNamedPane(t *testing.T) {
	dir := t.TempDir()
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.run(context.Background(), runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}
	out, err := p.read(context.Background(), readPaneInput{Name: "server", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "listening on :3000") {
		t.Fatalf("read output = %q", out)
	}
	if s := readLog(t, log); !strings.Contains(s, "dump-screen --pane-id terminal_1 --full") {
		t.Fatalf("dump-screen not targeted by id:\n%s", s)
	}
	if _, err := p.read(context.Background(), readPaneInput{Name: "missing"}); err == nil {
		t.Fatal("read of unknown pane should error")
	}
}

func TestClosePaneRemovesFromRegistry(t *testing.T) {
	dir := t.TempDir()
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.run(context.Background(), runCommandInput{Command: "tail -f log", Name: "logs"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.closePane(context.Background(), paneNameInput{Name: "logs"}); err != nil {
		t.Fatal(err)
	}
	if got := p.list(); len(got) != 0 {
		t.Fatalf("registry = %v, want empty", got)
	}
	if s := readLog(t, log); !strings.Contains(s, "close-pane --pane-id terminal_1") {
		t.Fatalf("close-pane not issued:\n%s", s)
	}
	if _, err := p.closePane(context.Background(), paneNameInput{Name: "logs"}); err == nil {
		t.Fatal("closing an unknown pane should error")
	}
}

func TestShowDiffOpensPaneForRepoAndAllRepos(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "src", "app")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.showDiff(context.Background(), showDiffInput{Repo: "src/app", Base: "main", Staged: true}); err != nil {
		t.Fatal(err)
	}
	realRepo, _ := filepath.EvalSymlinks(repo)
	s := readLog(t, log)
	for _, want := range []string{"--name ◆ diff:app", "git -C '" + realRepo + "' diff --staged 'main'", "--pinned true",
		// Esc ends the footer's wait, and close-on-exit turns that into a
		// dismissed pane.
		"Esc to close", launch.DismissCommand(0), "--close-on-exit"} {
		if !strings.Contains(s, want) {
			t.Fatalf("single-repo diff missing %q:\n%s", want, s)
		}
	}
	if got := p.list(); len(got) != 1 || got[0] != "diff:app" {
		t.Fatalf("registry = %v, want [diff:app]", got)
	}

	if _, err := p.showDiff(context.Background(), showDiffInput{}); err != nil {
		t.Fatal(err)
	}
	if s := readLog(t, log); !strings.Contains(s, "for d in src/*/") || !strings.Contains(s, "less -FRX") {
		t.Fatalf("all-repos diff should walk worktrees through a pager:\n%s", s)
	}

	if _, err := p.showDiff(context.Background(), showDiffInput{Repo: "../outside"}); err == nil {
		t.Fatal("accepted a repo path outside the session")
	}
}

func TestNotifyOpensSelfClosingToastWithSound(t *testing.T) {
	dir := t.TempDir()
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.notify(context.Background(), notifyInput{Message: "build finished"}); err != nil {
		t.Fatal(err)
	}
	s := readLog(t, log)
	script := filepath.Join(dir, ".qrouton", "notify.sh")
	for _, want := range []string{"--name 🔔 notification · Esc to close", "--close-on-exit", "'" + script + "'", "build finished", "focus-pane-id terminal_0",
		// Dismissable on Esc through the same shared wait as every other pane,
		// and still self-closing after toastSeconds.
		launch.DismissCommand(toastSeconds)} {
		if !strings.Contains(s, want) {
			t.Fatalf("notify toast missing %q:\n%s", want, s)
		}
	}
	if _, err := p.notify(context.Background(), notifyInput{Message: "  "}); err == nil {
		t.Fatal("accepted an empty notify message")
	}
}

func TestHelpFloatsTheSharedPanelWithFocus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.help(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := readLog(t, log)
	opts := launch.HelpSpawn(dir, "")
	for _, want := range []string{
		"--x " + opts.Geometry.X + " --y " + opts.Geometry.Y +
			" --width " + opts.Geometry.Width + " --height " + opts.Geometry.Height,
		"--name " + opts.Label,
		"--close-on-exit",
		"-- sh " + filepath.Join(dir, "config", "qrouton", "help.sh"),
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("help panel missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "toggle-floating-panes") || strings.Contains(s, "focus-pane-id") {
		t.Fatalf("help must keep focus on the panel; Esc is what dismisses it:\n%s", s)
	}
	// A second call replaces the panel rather than stacking another on top.
	if _, err := p.help(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readLog(t, log), "close-pane --pane-id") {
		t.Fatal("a second help call did not replace the panel already open")
	}
}

func TestEscalateSpawnsPickerFocusedAtPickerGeometry(t *testing.T) {
	dir := t.TempDir()
	helper, log := fakeZellij(t, dir)
	p := testManager(t, dir, helper)
	shortEscalatePoll(t, 200*time.Millisecond)

	// A cancelled stanza lets escalate return promptly once its poll notices
	// it, so the test doesn't wait out the full timeout to inspect the argv.
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = session.WriteManifest(dir, session.Manifest{
			Escalation: &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()},
		})
	}()

	message, err := p.escalate(context.Background(), escalateInput{Name: "webhook retry", BranchPrefix: "fix"})
	if err != nil {
		t.Fatal(err)
	}
	if message != escalationCancelledMessage {
		t.Fatalf("message = %q, want the cancelled message", message)
	}

	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := readLog(t, log)
	for _, want := range []string{
		"new-pane --floating --pinned true",
		"--x 20% --y 3% --width 60% --height 94%",
		"--name escalate · Esc to cancel",
		"-- " + bin + " pick --session-root " + dir + " --name webhook retry --prefix fix",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("calls missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "toggle-floating-panes") || strings.Contains(s, "focus-pane-id") {
		t.Fatalf("escalate must keep focus on the picker, not return it to the agent:\n%s", s)
	}
}

func TestEscalateRejectsBlankName(t *testing.T) {
	dir := t.TempDir()
	helper, _ := fakeZellij(t, dir)
	p := testManager(t, dir, helper)

	if _, err := p.escalate(context.Background(), escalateInput{Name: "   "}); err == nil {
		t.Fatal("accepted a blank name")
	}
}

func TestEscalateBlocksUntilConfirmed(t *testing.T) {
	dir := t.TempDir()
	helper, _ := fakeZellij(t, dir)
	p := testManager(t, dir, helper)
	shortEscalatePoll(t, time.Second)

	start := time.Now()
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = session.WriteManifest(dir, session.Manifest{
			Escalation: &session.EscalationOutcome{Status: session.EscalationConfirmed, At: time.Now()},
		})
	}()

	message, err := p.escalate(context.Background(), escalateInput{Name: "webhook retry"})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("escalate returned after %s, before the confirmed stanza landed", elapsed)
	}
	if message != escalationConfirmedMessage {
		t.Fatalf("message = %q, want the confirmed message", message)
	}
}

func TestEscalateBlocksUntilCancelled(t *testing.T) {
	dir := t.TempDir()
	helper, _ := fakeZellij(t, dir)
	p := testManager(t, dir, helper)
	shortEscalatePoll(t, time.Second)

	start := time.Now()
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = session.WriteManifest(dir, session.Manifest{
			Escalation: &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()},
		})
	}()

	message, err := p.escalate(context.Background(), escalateInput{Name: "webhook retry"})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("escalate returned after %s, before the cancelled stanza landed", elapsed)
	}
	if message != escalationCancelledMessage {
		t.Fatalf("message = %q, want the cancelled message", message)
	}
}

func TestEscalateTimesOutWhenPickerStaysOpen(t *testing.T) {
	dir := t.TempDir()
	helper, _ := fakeZellij(t, dir)
	p := testManager(t, dir, helper)
	shortEscalatePoll(t, 20*time.Millisecond)

	message, err := p.escalate(context.Background(), escalateInput{Name: "webhook retry"})
	if err != nil {
		t.Fatal(err)
	}
	if message != escalationTimeoutMessage {
		t.Fatalf("message = %q, want the timeout message", message)
	}
}

func TestMCPServerAdvertisesAllTools(t *testing.T) {
	ctx := context.Background()
	server := newMCPServer(t.TempDir(), launch.EditorCommand{Argv: []string{"vi"}}, mux.NewZellijHost("zellij", "test-session"))
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	want := map[string]bool{"open_file": false, "run_command": false, "read_pane": false, "show_diff": false, "notify": false, "close_pane": false, "list_panes": false, "escalate": false, "help": false}
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q was not advertised", name)
		}
	}
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The registry is qrouton's own map and nothing in it learns that the user
// dismissed a pane. Reading a name whose pane is gone must say so — and forget
// it — rather than passing a dead id to the backend and surfacing its failure.
func TestReadAndClosePruneAPaneTheUserDismissed(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*paneManager) error
	}{
		{"read_pane", func(p *paneManager) error {
			_, err := p.read(context.Background(), readPaneInput{Name: "server"})
			return err
		}},
		{"close_pane", func(p *paneManager) error {
			_, err := p.closePane(context.Background(), paneNameInput{Name: "server"})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			helper, _ := fakeZellij(t, dir)
			p := testManager(t, dir, helper)
			if _, err := p.run(context.Background(), runCommandInput{Command: "serve", Name: "server"}); err != nil {
				t.Fatal(err)
			}
			// The user closes it by hand: the pane is gone from the session, but
			// still registered here.
			livePanes(t, 99)

			err := tc.call(p)
			if err == nil {
				t.Fatal("addressing a pane the user dismissed should fail")
			}
			if !strings.Contains(err.Error(), "closed by the user") {
				t.Fatalf("error does not explain the pane is gone: %v", err)
			}
			if got := p.list(); len(got) != 0 {
				t.Fatalf("registry still holds a dismissed pane: %v", got)
			}
		})
	}
}

// A pane that is still there stays addressable, and the liveness check must not
// cost the caller anything when it passes.
func TestReadPaneKeepsALivePane(t *testing.T) {
	dir := t.TempDir()
	helper, _ := fakeZellij(t, dir)
	p := testManager(t, dir, helper)
	if _, err := p.run(context.Background(), runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}
	livePanes(t, 1)

	out, err := p.read(context.Background(), readPaneInput{Name: "server"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "listening on :3000") {
		t.Fatalf("read output = %q", out)
	}
	if got := p.list(); len(got) != 1 {
		t.Fatalf("live pane was pruned: %v", got)
	}
}

// An unreadable pane list is not evidence the pane is gone. Guessing "closed"
// from a backend hiccup would have the agent reopen panes that are fine, so the
// pane is kept and the real action reports whatever is actually wrong.
func TestReadPaneKeepsThePaneWhenLivenessCannotBeChecked(t *testing.T) {
	dir := t.TempDir()
	helper, _ := fakeZellij(t, dir)
	p := testManager(t, dir, helper)
	if _, err := p.run(context.Background(), runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANES_JSON", "not json")

	if _, err := p.read(context.Background(), readPaneInput{Name: "server"}); err != nil {
		t.Fatalf("an unparseable pane list should not fail the read: %v", err)
	}
	if got := p.list(); len(got) != 1 {
		t.Fatalf("pane pruned on an unreadable pane list: %v", got)
	}
}
