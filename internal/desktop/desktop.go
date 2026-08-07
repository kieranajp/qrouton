// Package desktop is qrouton's workbench: the Wails application, the PTY the
// conversation runs in, the OS windows the session shows the user, and the
// control socket the agent's tools reach them through.
package desktop

import (
	"context"
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
	windows := newWindows(r, r.Emit)
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
	go watchChrome(ctx, sessionRoot, r.Emit)

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

	shell := &shellWindow{windows: windows, argv: opts.Shell}
	server, err := serveControl(opts.Socket, windows, func(adopted string) {
		root.Store(&adopted)
		r.Retitle(mainWindowName, windowTitle(adopted))
		shell.open(adopted)
		// A resumed session's manifest still lists the last run's windows, and
		// none of them are open.
		record.save()
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

// shellWindow is the session's user shell: exactly one, opened as soon as the
// session is known, which onboarding may only decide after the workbench opens.
type shellWindow struct {
	windows *Windows
	argv    func(string) []string
	opened  atomic.Bool
}

func (s *shellWindow) open(root string) {
	if root == "" || s.argv == nil || !s.opened.CompareAndSwap(false, true) {
		return
	}
	if _, err := s.windows.openStructural(workbench.WindowOptions{
		Kind:    workbench.KindTerminal,
		Label:   shellWindowLabel,
		Cwd:     root,
		Command: s.argv(root),
	}); err != nil {
		s.opened.Store(false)
	}
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
