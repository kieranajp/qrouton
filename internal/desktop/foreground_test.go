package desktop

import (
	"encoding/base64"
	"sync"
	"testing"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// shellBench is a registry and its terminals with no background poll, so a
// test drives every foreground read itself.
type shellBench struct {
	tabs      *registry
	terminals *terminals
	owner     *sessionState
	mu        sync.Mutex
	announced int
}

func newShellBench(t *testing.T) *shellBench {
	t.Helper()
	b := &shellBench{}
	sessions := testRegistry(t, t.TempDir())
	b.owner = sessions.current()
	emit := func(event string, _ any) {
		if event == windowsEvent {
			b.mu.Lock()
			b.announced++
			b.mu.Unlock()
		}
	}
	b.tabs = newRegistry(emit, sessions)
	b.terminals = newTerminals(emit, b.tabs)
	t.Cleanup(b.tabs.stopAll)
	return b
}

func (b *shellBench) launch(t *testing.T, shell bool, argv ...string) string {
	t.Helper()
	opts := workbench.WindowOptions{Kind: workbench.KindTerminal, Label: "tab", Cwd: t.TempDir(), Command: argv}
	open := b.tabs.openStructural
	if shell {
		open = b.tabs.openShell
	}
	id, err := open(b.owner, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.terminals.launch(id, unmountedCols, unmountedRows); err != nil {
		t.Fatal(err)
	}
	return id
}

func (b *shellBench) status(t *testing.T, id string) string {
	t.Helper()
	for _, tab := range b.tabs.surfaces(b.owner).Tabs {
		if tab.ID == id {
			return tab.Status
		}
	}
	t.Fatalf("no tab %s", id)
	return ""
}

func (b *shellBench) announcements() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.announced
}

// Catches the + shell opening as a plain terminal, which spins before the shell
// has even started.
func TestAShellOpensIdle(t *testing.T) {
	w, _ := testWindows(t)
	boot := newStubBoot()
	boot.shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	shells := &shellWindow{windows: w, launcher: boot}

	id, err := shells.another(w.shown())
	if err != nil {
		t.Fatal(err)
	}
	tabs := w.surfaces(w.shown()).Tabs
	if len(tabs) != 1 || tabs[0].ID != id || tabs[0].Status != tabStatusIdle {
		t.Fatalf("tabs after + = %+v, want one idle shell", tabs)
	}
}

// Catches an agent's command reading as idle because its own process holds the
// terminal, as an idle shell's does.
func TestAnAgentCommandSpinsUntilItExits(t *testing.T) {
	b := newShellBench(t)
	id := b.launch(t, false, "/bin/sh", "-c", "sleep 0.3; exit 2")
	for range 3 {
		b.terminals.pollForeground()
		if got := b.status(t, id); got != tabStatusRunning {
			t.Fatalf("agent command status = %q, want %q", got, tabStatusRunning)
		}
	}
	waitFor(t, "the command to fail", func() bool { return b.status(t, id) == tabStatusFailed })
}

// Catches a read of the wrong terminal or a comparison against the wrong pid,
// through the real ioctl.
func TestAShellSpinsOnlyWhileAForegroundJobRuns(t *testing.T) {
	b := newShellBench(t)
	id := b.launch(t, true, "/bin/sh", "-i")
	typed := func(text string) {
		if err := b.terminals.write(id, base64.StdEncoding.EncodeToString([]byte(text))); err != nil {
			t.Fatal(err)
		}
	}
	settle := func(want string) {
		waitFor(t, "a shell that is "+want, func() bool {
			b.terminals.pollForeground()
			return b.status(t, id) == want
		})
	}

	settle(tabStatusIdle)
	typed("set -m\nsleep 30\n")
	settle(tabStatusRunning)
	typed("\x03")
	settle(tabStatusIdle)
}

// Catches the strip being re-announced on every tick rather than on a change.
func TestThePollerAnnouncesOnlyChanges(t *testing.T) {
	b := newShellBench(t)
	id := b.launch(t, true, "/bin/cat")
	busy := false
	b.terminals.sample = func(*ptyProcess) (bool, bool) { return busy, true }
	before := b.announcements()

	b.terminals.pollForeground()
	b.terminals.pollForeground()
	if got := b.announcements() - before; got != 0 {
		t.Fatalf("unchanged idle shell announced %d times", got)
	}
	busy = true
	b.terminals.pollForeground()
	b.terminals.pollForeground()
	if got := b.announcements() - before; got != 1 || b.status(t, id) != tabStatusRunning {
		t.Fatalf("after a job started: %d announcements, status %q", got, b.status(t, id))
	}
	busy = false
	b.terminals.pollForeground()
	b.terminals.pollForeground()
	if got := b.announcements() - before; got != 2 || b.status(t, id) != tabStatusIdle {
		t.Fatalf("after the job ended: %d announcements, status %q", got, b.status(t, id))
	}
}

// Catches a failed read being taken as an idle shell.
func TestAFailedReadKeepsTheLastState(t *testing.T) {
	b := newShellBench(t)
	id := b.launch(t, true, "/bin/cat")
	readable := true
	b.terminals.sample = func(*ptyProcess) (bool, bool) { return readable, readable }
	b.terminals.pollForeground()
	before := b.announcements()

	readable = false
	b.terminals.pollForeground()
	if got := b.status(t, id); got != tabStatusRunning {
		t.Fatalf("status after a failed read = %q, want %q", got, tabStatusRunning)
	}
	if got := b.announcements() - before; got != 0 {
		t.Fatalf("a failed read announced %d times", got)
	}
}
