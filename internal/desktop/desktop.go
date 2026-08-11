// Package desktop is qrouton's workbench: the Wails application, the PTY the
// conversation runs in, the OS windows the session shows the user, and the
// control socket the agent's tools reach them through.
package desktop

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Options is the session the workbench opens on.
type Options struct {
	// SessionRoot is empty when no session has been chosen yet: onboarding runs
	// in the conversation terminal and adopts one over the control socket.
	SessionRoot string
	Socket      string
	Argv        []string
	Env         []string
	// Shell builds the user shell window's command for a session root, which is
	// why it is a function rather than an argv.
	Shell func(sessionRoot string) []string
	// Document is the window one of the session's own files opens in, named
	// relative to its root; it errors on a name resolving outside the session.
	Document func(sessionRoot, name string) (workbench.WindowOptions, error)
	// Dock sends the agent's windows to the tab strip rather than the screen.
	Dock bool
}

// Run opens the workbench and blocks until the session ends: closing the
// conversation window and the agent exiting are the same ending reached from
// either side. There is no detached server and nothing survives in the
// background.
func Run(opts Options) error {
	if len(opts.Argv) == 0 {
		return ErrNoAgentCommand
	}
	if opts.Socket == "" {
		return ErrNoControlSocket
	}
	assets, err := frontend()
	if err != nil {
		return err
	}
	r := newWailsRenderer(assets)
	term := newTerm(opts, r.Emit)
	windows := newWindows(r, r.Emit, opts.Dock)
	r.register(application.NewService(term))
	r.register(application.NewService(windows))
	return run(r, term, windows, opts)
}

// run is Run with the renderer already built, so the window lifecycle and the
// control socket are exercised against a double instead of a display.
func run(r renderer, term *Term, windows *Windows, opts Options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var root atomic.Pointer[string]
	root.Store(&opts.SessionRoot)
	sessionRoot := func() string { return *root.Load() }
	go watchChrome(ctx, sessionRoot, term.activity.state, r.Emit)

	record := &windowRecorder{root: sessionRoot, windows: windows}
	windows.observe(record.save)

	// Closing the conversation window and the agent exiting are the same ending
	// reached from either side, and either may arrive first.
	quit := sync.OnceFunc(func() {
		windows.observe(nil)
		windows.stopAll()
		term.Stop()
		r.Quit()
	})
	// A supervisor that failed keeps its window, so the reason stays readable.
	term.whenChildExits(func(code int) {
		if code == 0 {
			quit()
		}
	})

	shell := &shellWindow{windows: windows, argv: opts.Shell, root: sessionRoot}
	windows.newShell = shell.another
	windows.newDocument = func(name string) (string, error) {
		return openDocument(windows, opts.Document, sessionRoot(), name)
	}
	server, err := serveControl(opts.Socket, windows, controlHooks{
		adopt: func(adopted string) {
			root.Store(&adopted)
			r.Retitle(mainWindowName, windowTitle(adopted))
			shell.open(adopted)
			// A resumed session's manifest still lists the last run's windows,
			// and none of them are open.
			record.save()
		},
		attention: term.activity.hook,
	})
	if err != nil {
		return err
	}
	defer server.Close()

	if err := r.Open(windowSpec{
		Name:    mainWindowName,
		Title:   windowTitle(opts.SessionRoot),
		URL:     frontendRoot,
		Width:   mainWindowWidth,
		Height:  mainWindowHeight,
		Focus:   true,
		OnClose: quit,
	}); err != nil {
		return err
	}
	shell.open(opts.SessionRoot)
	record.save()
	return r.Run()
}

// shellWindow is the session's user shell: one opened unasked, and however many
// more the user asks for.
type shellWindow struct {
	windows *Windows
	argv    func(string) []string
	root    func() string

	mu sync.Mutex
	id string
	// opened counts the shells the session has had rather than the ones still
	// open: a number freed by a close would name two terminals at once.
	opened int
}

// open gives the session a shell without being asked. Adoption may call it
// again once the session is known, which must not leave two.
func (s *shellWindow) open(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.id != "" && s.windows.exists(s.id) {
		return
	}
	id, err := s.spawn(root)
	if err != nil {
		return
	}
	s.id = id
}

// another is the tab strip's + button.
func (s *shellWindow) another() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spawn(s.root())
}

// spawn is called with mu held, so the numbering matches the order the tabs
// open in.
func (s *shellWindow) spawn(root string) (string, error) {
	if root == "" || s.argv == nil {
		return "", ErrNoShellCommand
	}
	s.opened++
	return s.windows.openStructural(workbench.WindowOptions{
		Kind:    workbench.KindTerminal,
		Label:   shellLabel(s.opened),
		Cwd:     root,
		Command: s.argv(root),
	})
}

// openDocument puts a document in the right pane. The user asked for it, so it
// is a tab under either windows preference, and unrecorded like the shell.
func openDocument(windows *Windows, window func(string, string) (workbench.WindowOptions, error), root, name string) (string, error) {
	if window == nil {
		return "", ErrNoEditorCommand
	}
	opts, err := window(root, name)
	if err != nil {
		return "", err
	}
	return windows.openStructural(opts)
}

// shellLabel leaves the first shell unnumbered, so a session with one reads as
// it always did.
func shellLabel(n int) string {
	if n <= 1 {
		return shellWindowLabel
	}
	return fmt.Sprintf(shellWindowLabelNumbers, n)
}

// windowTitle names the session in the conversation's title bar; onboarding has
// not chosen one yet.
func windowTitle(root string) string {
	if root == "" {
		return mainWindowTitle
	}
	return mainWindowTitle + titleSeparator + filepath.Base(root)
}

// frontend is the embedded page tree the webview serves.
func frontend() (fs.FS, error) {
	return fs.Sub(assetFS, assetRoot)
}
