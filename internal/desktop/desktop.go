// Package desktop is qrouton's workbench: the Wails application, the PTY the
// conversation runs in, the OS windows the session shows the user, and the
// control socket the agent's tools reach them through.
package desktop

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Options is the workbench: where its sessions live, which one it opens on, and
// how it builds what each of them runs.
type Options struct {
	// SessionRoot is empty when no session has been chosen yet: onboarding runs
	// in the conversation terminal and adopts one over the control socket.
	SessionRoot string
	// Resume asks the runner of the session the workbench opens on to continue
	// its previous conversation. A session the rail boots always resumes.
	Resume bool
	// Root is where sessions live, so the rail can boot one this workbench has
	// never run.
	Root string
	// Socket answers the launcher's readiness poll and the adopt an onboarding
	// child with no session of its own sends.
	Socket string
	// Onboard is the conversation's command on the landing path, which has no
	// session to build an agent command from.
	Onboard []string
	Env     []string
	// Agent builds a session's supervisor command and environment against the
	// control socket the workbench will serve that session on.
	Agent func(sessionRoot, socket string, resume bool) (argv, env []string, err error)
	// Shell builds the user shell window's command for a session root, which is
	// why it is a function rather than an argv.
	Shell func(sessionRoot string) []string
	// Document is the window one of the session's own files opens in, named
	// relative to its root; it errors on a name resolving outside the session.
	Document func(sessionRoot, name string) (workbench.WindowOptions, error)
	// Picker builds the repository picker's command, as Shell does.
	Picker func(sessionRoot string) []string
	// Onboarding builds the command that assembles a new session. It takes the
	// control socket, not a root: it names the session it makes by adopting it.
	Onboarding func(socket string) []string
	// Dock sends the agent's windows to the tab strip rather than the screen.
	Dock bool
}

// Run opens the workbench and blocks until the window closes. Every session it
// booted goes with it; nothing survives in the background.
func Run(opts Options) error {
	if opts.Agent == nil || (opts.SessionRoot == "" && len(opts.Onboard) == 0) {
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
	reg := newSessions()
	term := newTerm(reg, r.Emit)
	windows := newWindows(r, r.Emit, opts.Dock, reg)
	r.register(application.NewService(term))
	r.register(application.NewService(windows))
	r.register(application.NewService(reg))
	return run(r, term, windows, opts)
}

// run is Run with the renderer already built, so the window lifecycle and the
// control socket are exercised against a double instead of a display.
func run(r renderer, term *Term, windows *Windows, opts Options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := term.sessions
	shell := &shellWindow{windows: windows, argv: opts.Shell}
	record := &windowRecorder{windows: windows}
	windows.observe(record.save)

	reg.boot = booting{
		root: func(slug string) string {
			if opts.Root == "" || slug == "" {
				return ""
			}
			root := filepath.Join(opts.Root, slug)
			// A rail row can outlive the directory it names, and a directory
			// without a manifest is not resumable.
			if _, err := os.Stat(sessionpaths.Manifest(root)); err != nil {
				return ""
			}
			return root
		},
		agent: opts.Agent,
		serve: func(state *sessionState, socket string) (io.Closer, error) {
			return serveControl(socket, windows, state, controlHooks{attention: state.activity.hook})
		},
		shown: func(state *sessionState) {
			r.Retitle(mainWindowName, windowTitle(state.root()))
			if state == nil {
				return
			}
			if root := state.root(); root != "" {
				_ = session.MarkOpened(root, time.Now())
			}
			shell.open(state)
			// A resumed session's manifest still lists the last run's windows,
			// and none of them are open.
			record.save(state)
		},
		teardown: windows.stop,
	}
	go watchChrome(ctx, reg, opts.Root, r.Emit)

	// Closing the conversation window ends the app; a supervisor exiting ends
	// only its own session, and a failed one keeps its terminal readable.
	quit := sync.OnceFunc(func() {
		windows.observe(nil)
		windows.stopAll()
		reg.stopAll()
		r.Quit()
	})
	term.whenChildExits(func(state *sessionState, code int) {
		if code == 0 {
			reg.retire(state)
		}
	})

	windows.newShell = func() (string, error) { return shell.another(reg.current()) }
	windows.newDocument = func(name string) (string, error) {
		return openDocument(windows, reg.current(), opts.Document, name)
	}
	windows.newPicker = func() (string, error) {
		return openPicker(windows, reg.current(), opts.Picker)
	}
	windows.newOnboard = func() (string, error) {
		return openOnboard(windows, reg.current(), opts.Onboarding, opts.Socket, opts.Root)
	}

	// The landing path's conversation is registered before it is served, because
	// the process socket is the session it goes on to adopt.
	var landing *sessionState
	if opts.SessionRoot == "" {
		landing = reg.add("", opts.Onboard, withTerminalEnv(opts.Env))
	}
	// The landing path's supervisor keeps talking to the process socket across the
	// handover, so that is where its runner raises attention.
	hooks := controlHooks{adopt: reg.adopt}
	if landing != nil {
		hooks.attention = landing.activity.hook
	}
	server, err := serveControl(opts.Socket, windows, landing, hooks)
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
	if landing != nil {
		reg.reveal(landing)
	} else {
		opened, err := reg.start(opts.SessionRoot, opts.Resume)
		if err != nil {
			return err
		}
		reg.reveal(opened)
	}
	return r.Run()
}

// shellWindow opens a session's user shells: one unasked, and however many more
// the user asks for. The numbering is the session's, so a second one restarts it.
type shellWindow struct {
	windows *Windows
	argv    func(string) []string
}

// open gives a session a shell without being asked. Adoption may call it again
// once the session is known, which must not leave two.
func (s *shellWindow) open(owner *sessionState) {
	if owner == nil {
		return
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.shell != "" && s.windows.exists(owner.shell) {
		return
	}
	id, err := s.spawn(owner)
	if err != nil {
		return
	}
	owner.shell = id
}

// another is the tab strip's + button.
func (s *shellWindow) another(owner *sessionState) (string, error) {
	if owner == nil {
		return "", ErrNoShellCommand
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return s.spawn(owner)
}

// spawn is called with the session's lock held, so the numbering matches the
// order the tabs open in.
func (s *shellWindow) spawn(owner *sessionState) (string, error) {
	root := owner.root()
	if root == "" || s.argv == nil {
		return "", ErrNoShellCommand
	}
	owner.shells++
	return s.windows.openStructural(owner, workbench.WindowOptions{
		Kind:    workbench.KindTerminal,
		Label:   shellLabel(owner.shells),
		Cwd:     root,
		Command: s.argv(root),
	})
}

// openDocument puts a document in the right pane. The user asked for it, so it
// is a tab under either windows preference, and unrecorded like the shell.
func openDocument(windows *Windows, owner *sessionState, window func(string, string) (workbench.WindowOptions, error), name string) (string, error) {
	if owner == nil || window == nil {
		return "", ErrNoEditorCommand
	}
	opts, err := window(owner.root(), name)
	if err != nil {
		return "", err
	}
	return windows.openStructural(owner, opts)
}

// openPicker puts the repository picker in the right pane. A tab, not a window
// of its own: adding a repository is not worth losing sight of the conversation.
func openPicker(windows *Windows, owner *sessionState, argv func(string) []string) (string, error) {
	if owner == nil || argv == nil {
		return "", ErrNoPickerCommand
	}
	root := owner.root()
	if root == "" {
		return "", ErrNoPickerCommand
	}
	return windows.openStructural(owner, workbench.WindowOptions{
		Kind:        workbench.KindTerminal,
		Label:       pickerWindowLabel,
		Cwd:         root,
		Command:     argv(root),
		Source:      pickerSource,
		CloseOnExit: true,
	})
}

// openOnboard puts the session assembly in the right pane, belonging to whichever
// session is on screen — or to none, which is the window with no session at all.
func openOnboard(windows *Windows, owner *sessionState, argv func(string) []string, socket, root string) (string, error) {
	if argv == nil || socket == "" {
		return "", ErrNoOnboardCommand
	}
	return windows.openStructural(owner, workbench.WindowOptions{
		Kind:        workbench.KindTerminal,
		Label:       onboardWindowLabel,
		Cwd:         root,
		Command:     argv(socket),
		Source:      onboardSource,
		CloseOnExit: true,
	})
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
