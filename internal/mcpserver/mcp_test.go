package mcpserver

import (
	"context"
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
		"esac\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", log)
	t.Setenv("ID_SEQ", filepath.Join(dir, "idseq"))
	return helper, log
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
		"Editor — Alt-f to view · Alt-x to close",
		"toggle-floating-panes",
		"close-pane --pane-id terminal_1", // second open replaces the first editor pane
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("calls missing %q:\n%s", want, s)
		}
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
		"--name ▶ dev",
		"--cwd " + realRepo,
		"-- sh -lc npm run dev",
		"toggle-floating-panes",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("calls missing %q:\n%s", want, s)
		}
	}
	if got := p.list(); len(got) != 1 || got[0] != "dev" {
		t.Fatalf("registry = %v, want [dev]", got)
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
	for _, want := range []string{"--name ◆ diff:app", "git -C '" + realRepo + "' diff --staged 'main'", "--pinned true"} {
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
	for _, want := range []string{"--name 🔔 notification", "--close-on-exit", "'" + script + "'", "build finished", "toggle-floating-panes"} {
		if !strings.Contains(s, want) {
			t.Fatalf("notify toast missing %q:\n%s", want, s)
		}
	}
	if _, err := p.notify(context.Background(), notifyInput{Message: "  "}); err == nil {
		t.Fatal("accepted an empty notify message")
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
		"--name escalate",
		"-- " + bin + " pick --session-root " + dir + " --name webhook retry --prefix fix",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("calls missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "toggle-floating-panes") {
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

	want := map[string]bool{"open_file": false, "run_command": false, "read_pane": false, "show_diff": false, "notify": false, "close_pane": false, "list_panes": false, "escalate": false}
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
