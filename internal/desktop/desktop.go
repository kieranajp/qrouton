// Package desktop is qrouton's workbench: one Wails conversation window, its
// tab registry and PTYs, and the control socket the agent's tools reach through.
package desktop

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Options is the workbench: where its sessions live, which one it opens on, and
// how it builds what each of them runs.
type Options struct {
	// SessionRoot is empty when there is no session to open on, which is the
	// window whose only content is the assembly overlay.
	SessionRoot string
	// Resume asks the runner of the session the workbench opens on to continue
	// its previous conversation. A session the rail boots always resumes.
	Resume bool
	// Root is where sessions live, so the rail can boot one this workbench has
	// never run.
	Root string
	// Socket answers the launcher's readiness poll.
	Socket string
	Env    []string
	// Agent builds a session's supervisor command and environment against the
	// control socket the workbench will serve that session on. runnerID names
	// the agent the session was assembled with; empty means the workbench's own.
	Agent func(sessionRoot, socket, runnerID string, resume bool) (argv, env []string, err error)
	// Shell builds the user shell window's command for a session root, which is
	// why it is a function rather than an argv.
	Shell func(sessionRoot string) []string
	// Document is the window one of the session's own files opens in, named
	// relative to its root; it errors on a name resolving outside the session.
	Document func(sessionRoot, name string) (workbench.WindowOptions, error)
	// Reveal builds the command that shows a session's directory in the file
	// manager, and is a function for the same reason Shell is.
	Reveal func(sessionRoot string) []string
	// Config is the sessions root and the configured owners the overlay assembles
	// against.
	Config *config.Config
	// Runners is the agents the overlay offers, mapped off launch's own rows so
	// nothing here imports launch.
	Runners func() ([]assembly.Runner, error)
	// Signal relaunches a session's runner after an escalation, which is
	// launch.SignalSupervisor reached without importing it.
	Signal func(sessionRoot string)
	// ValidateEditor and ValidateLaunch reach launch.ResolveEditor and
	// launch.Runners without desktop importing launch, the same shape Agent,
	// Shell, Signal and Runners already use.
	ValidateEditor func(argv []string) error
	ValidateLaunch func(overrides map[string][]string) error
}

// Run opens the workbench and blocks until the window closes. Every session it
// booted goes with it; nothing survives in the background.
func Run(opts Options) error {
	// A workbench with no way to build an agent command is an error; a workbench
	// with no session is the ordinary case.
	if opts.Agent == nil {
		return ErrNoAgentCommand
	}
	if opts.Socket == "" {
		return ErrNoControlSocket
	}
	if opts.Config == nil {
		return ErrNoConfig
	}
	assets, err := frontend()
	if err != nil {
		return err
	}
	r := newWailsRenderer(assets)
	reg := newSessions()
	term := newTerm(reg, r.Emit)
	windows := newWindows(r.Emit, reg)
	repos := newRepositories(opts.Config, r.Emit)
	picker := newPicker(opts.Config, reg, repos, opts.Signal)
	// The same teardown the main window's OnClose runs, shared with Settings.Quit
	// so quitting from the panel is no worse than closing the window.
	quit := sync.OnceFunc(func() {
		windows.observe(nil)
		windows.stopAll()
		reg.stopAll()
		r.Quit()
	})
	r.register(application.NewService(term))
	r.register(application.NewService(windows))
	r.register(application.NewService(reg))
	r.register(application.NewService(repos))
	r.register(application.NewService(&Orgs{cfg: opts.Config}))
	r.register(application.NewService(newAssembly(opts.Config, repos, reg, r.Emit, opts.Signal, opts.Runners)))
	r.register(application.NewService(picker))
	r.register(application.NewService(newSettings(opts.Config, opts.ValidateEditor, opts.ValidateLaunch, quit)))
	return run(r, term, windows, opts, quit)
}

// run is Run with the renderer already built, so the window lifecycle and the
// control socket are exercised against a double instead of a display. quit is
// the same teardown Run wires into Settings.Quit.
func run(r renderer, term *Term, windows *Windows, opts Options, quit func()) error {
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
			return serveControl(socket, windows, state,
				controlHooks{attention: state.activity.hook, picker: reg.queuePicker})
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
		// The destructive path addresses a session by the sessions root and the
		// manifest's slug, so opts.Root is what it takes and not the session's own.
		uncommitted: func(root string) ([]string, error) {
			m, err := session.Load(root)
			if err != nil {
				return nil, err
			}
			return session.DirtyWorktrees(opts.Root, m)
		},
		cleanup: func(root string) error {
			m, err := session.Load(root)
			if err != nil {
				return err
			}
			if dir := filepath.Base(root); m.Slug != dir {
				return mismatchedManifest(dir, m.Slug)
			}
			return session.Delete(opts.Root, m)
		},
		reveal: func(root string) error {
			if opts.Reveal == nil {
				return ErrNoRevealCommand
			}
			argv := opts.Reveal(root)
			if len(argv) == 0 {
				return ErrNoRevealCommand
			}
			// open(1) hands the request to the desktop and exits, so waiting on it
			// costs nothing and turns a bad path into an error the page can show.
			return exec.Command(argv[0], argv[1:]...).Run()
		},
	}
	go watchChrome(ctx, reg, opts.Root, opts.Config, r.Emit)

	// Closing the conversation window ends the app; a supervisor exiting ends
	// only its own session, and a failed one keeps its terminal readable.
	term.whenChildExits(func(state *sessionState, code int) {
		if code == 0 {
			reg.retire(state)
		}
	})

	windows.newShell = func() (string, error) { return shell.another(reg.current()) }
	windows.newDocument = func(name string) (string, error) {
		return openDocument(windows, reg.current(), opts.Document, name)
	}
	server, err := serveControl(opts.Socket, windows, nil, controlHooks{picker: reg.queuePicker})
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
	if opts.SessionRoot != "" {
		opened, err := reg.start(opts.SessionRoot, "", opts.Resume)
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

// openDocument puts a document in the right pane, unrecorded like the shell.
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
	assets, err := fs.Sub(assetFS, assetRoot)
	if err != nil {
		return nil, err
	}
	if err := validateFrontend(assets); err != nil {
		return nil, err
	}
	return assets, nil
}
