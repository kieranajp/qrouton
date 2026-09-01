// Package desktop is qrouton's workbench: one Wails conversation window, its
// tab registry and PTYs, and the control socket the agent's tools reach through.
package desktop

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Options struct {
	Icon []byte
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
	// LinearIssue is a canonical ticket offered after session boot is wired and
	// before the page can claim a blank draft. LinearPrompt is the full prompt
	// Linear generated for the eventual runner's first turn.
	LinearIssue  string
	LinearPrompt string
	// LinearCommand is the executable and fixed arguments written before
	// Linear's issue identifier template in coding-tools.json.
	LinearCommand     []string
	LinearEnvironment []string
	Env               []string
	Config            *config.Config

	// Launcher builds every command the workbench runs; Validator and Relauncher
	// answer the settings panel and first run.
	Launcher   Launcher
	Validator  Validator
	Relauncher Relauncher

	assembly *Assembly
	chrome   *Chrome
	picker   *Picker
}

// Run opens the workbench and blocks until the window closes. Every session it
// booted goes with it; nothing survives in the background.
func Run(opts Options) error {
	// A workbench with no way to build an agent command is an error; a workbench
	// with no session is the ordinary case.
	if opts.Launcher == nil {
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
	r := newWailsRenderer(assets, opts.Icon)
	reg := newSessions()
	term := newTerm(reg, r.Emit)
	windows := newWindows(r.Emit, reg)
	repos := newRepositories(opts.Config, r.Emit)
	picker := newPicker(opts.Config, reg, repos, opts.Launcher.Signal)
	// The same teardown the main window's OnClose runs, shared with Settings.Quit
	// so quitting from the panel is no worse than closing the window.
	quit := sync.OnceFunc(func() {
		windows.stopAll()
		reg.stopAll()
		r.Quit()
	})
	r.register(application.NewService(term))
	r.register(application.NewService(windows))
	r.register(application.NewService(reg))
	r.register(application.NewService(repos))
	r.register(application.NewService(&Orgs{cfg: opts.Config}))
	chrome := newChrome(r.Emit)
	opts.chrome = chrome
	r.register(application.NewService(chrome))
	assemblyService := newAssembly(opts.Config, repos, reg, r.Emit, opts.Launcher.Signal, opts.Launcher.Runners)
	opts.assembly = assemblyService
	r.register(application.NewService(assemblyService))
	opts.picker = picker
	r.register(application.NewService(picker))
	validateEditor, validateLaunch := validators(opts.Validator)
	r.register(application.NewService(newSettings(
		opts.Config, r.Emit, validateEditor, validateLaunch,
		opts.LinearCommand, opts.LinearEnvironment, quit,
	)))
	relaunch := pendingRelaunch(relaunchWith(opts.Relauncher), assemblyService)
	r.register(application.NewService(newFirstRun(opts.Config, reg, relaunch, quit, r.chooseDirectory)))
	return run(r, term, windows, opts, quit)
}

func pendingRelaunch(relaunch func(func() (string, string)) error, assembly *Assembly) func() error {
	if relaunch == nil {
		return nil
	}
	return func() error { return relaunch(assembly.pendingLinear) }
}

// validators are the settings panel's checks, which a workbench without a
// Validator simply does not make.
func validators(v Validator) (func([]string) error, func(map[string][]string) error) {
	if v == nil {
		return nil, nil
	}
	return v.ValidateEditor, v.ValidateLaunch
}

// relaunchWith is the successor first run opens, and nil in a workbench with no
// way to open one.
func relaunchWith(r Relauncher) func(func() (string, string)) error {
	if r == nil {
		return nil
	}
	return r.Relaunch
}

// run is Run with the renderer already built, so the window lifecycle and the
// control socket are exercised against a double instead of a display.
func run(r renderer, term *Term, windows *Windows, opts Options, quit func()) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := term.sessions
	launcher := opts.Launcher
	shell := &shellWindow{windows: windows, launcher: launcher}

	reg.boot = booting{
		root:  func(slug string) string { return session.Resumable(opts.Root, slug) },
		agent: launcher.Agent,
		serve: func(state *sessionState, socket string) (io.Closer, error) {
			return serveControl(socket, windows, state, controlHooks{
				attention: func(value string, generation uint64) {
					if state.agents.attention(generation, value) {
						reg.touch()
					}
				},
				generation: func(req workbench.RunnerGenerationRequest) {
					if req.Provider != state.provider {
						return
					}
					if state.agents.begin(req.Provider, req.Generation) {
						reg.touch()
					}
				},
				lifecycle: func(req workbench.DelegatedLifecycleRequest) {
					if state.agents.lifecycle(req) {
						reg.touch()
					}
				},
				picker:   reg.queuePicker,
				addRepos: addReposHook(opts.picker),
			})
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
		},
		teardown: windows.stop,
		// The destructive paths address a session by the sessions root and the
		// manifest's slug, so opts.Root is what they take and not the session's own.
		uncommitted: func(root string) ([]string, error) { return session.Uncommitted(opts.Root, root) },
		cleanup:     func(root string) error { return session.Remove(opts.Root, root) },
		reveal:      func(root string) error { return reveal(launcher.Reveal(root)) },
	}
	chromeEmit := r.Emit
	if opts.chrome != nil {
		chromeEmit = opts.chrome.publish
	}
	go watchChrome(ctx, reg, opts.Root, opts.Config, chromeEmit)

	// Closing the conversation window ends the app; a supervisor exiting ends
	// only its own session, and a failed one keeps its terminal readable.
	term.whenChildExits(func(state *sessionState, code int) {
		if state.agents.exitWithProvider(state.provider, code) {
			reg.touch()
		}
		if code == 0 {
			reg.retire(state)
		}
	})

	windows.newShell = func() (string, error) { return shell.another(reg.current()) }
	windows.newDocument = func(name string) (string, error) {
		return openDocument(windows, reg.current(), launcher, name)
	}
	processHooks := controlHooks{picker: reg.queuePicker}
	if opts.assembly != nil {
		processHooks.linearIssue = opts.assembly.offer
		processHooks.focus = func() { r.Focus(mainWindowName) }
	}
	server, err := serveControl(opts.Socket, windows, nil, processHooks)
	if err != nil {
		return err
	}
	defer server.Close()
	if opts.LinearIssue != "" {
		if opts.assembly == nil {
			return ErrNoConfig
		}
		if _, err := opts.assembly.offer(opts.LinearIssue, opts.LinearPrompt); err != nil {
			return err
		}
	}
	if err := workbench.Publish(opts.Socket); err != nil {
		return err
	}
	defer workbench.Unpublish(opts.Socket)
	titleRoot := opts.SessionRoot
	if shown := reg.current(); shown != nil {
		titleRoot = shown.root()
	}

	if err := r.Open(windowSpec{
		Name:    mainWindowName,
		Title:   windowTitle(titleRoot),
		URL:     frontendRoot,
		Width:   mainWindowWidth,
		Height:  mainWindowHeight,
		Focus:   true,
		OnClose: quit,
	}); err != nil {
		return err
	}
	if opts.LinearIssue == "" && opts.SessionRoot != "" {
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
	windows  *Windows
	launcher Launcher
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
	if root == "" || s.launcher == nil {
		return "", ErrNoShellCommand
	}
	argv := s.launcher.Shell(root)
	if len(argv) == 0 {
		return "", ErrNoShellCommand
	}
	owner.shells++
	return s.windows.openStructural(owner, workbench.WindowOptions{
		Kind:        workbench.KindTerminal,
		Label:       shellLabel(owner.shells),
		Cwd:         root,
		Command:     argv,
		CloseOnExit: true,
	})
}

func openDocument(windows *Windows, owner *sessionState, launcher Launcher, name string) (string, error) {
	if owner == nil || launcher == nil {
		return "", ErrNoEditorCommand
	}
	opts, err := launcher.Document(owner.root(), name)
	if err != nil {
		return "", err
	}
	return windows.openStructural(owner, opts)
}

// reveal shows a session's directory in the file manager. open(1) hands the
// request to the desktop and exits, so waiting on it costs nothing and turns a
// bad path into an error the page can show.
func reveal(argv []string) error {
	if len(argv) == 0 {
		return ErrNoRevealCommand
	}
	return exec.Command(argv[0], argv[1:]...).Run()
}

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
