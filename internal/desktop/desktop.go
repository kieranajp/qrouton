// Package desktop is qrouton's workbench: one Wails conversation window, the
// services its page binds, and the local client the core reaches the machine
// through.
package desktop

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type emitter = core.Emitter

type Options struct {
	Icon []byte
	// Version is the packaged release the binary was built as. Empty in a dev
	// build, which turns the update check off.
	Version string
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
	// Inspect leaves the webview its own right-click menu, for Inspect Element,
	// and logs the page's WebGL renderers coming and going.
	Inspect bool
	// Ticket is a canonical ticket offered after session boot is wired and
	// before the page can claim a blank draft. TicketPrompt is the full prompt
	// the external tool generated for the eventual runner's first turn.
	Ticket       string
	TicketPrompt string
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

	assembly  *Assembly
	updates   *Updates
	placement *Placement
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
	// The asset routes are built into the renderer, which is built before the
	// core they read through: they hold the variable rather than the core.
	var kernel *core.Core
	r := newWailsRenderer(assets, opts.Icon, opts.Config.EffectiveUIScale, func(token, rel string) (*os.File, error) {
		if kernel == nil {
			return nil, core.ErrNoSuchAsset
		}
		return kernel.Windows.OpenDeckAsset(token, rel)
	}, func(token string, index int) (*os.File, string, error) {
		if kernel == nil {
			return nil, "", core.ErrNoSuchAsset
		}
		return kernel.Windows.OpenGalleryAsset(token, index)
	})
	client := newLocalClient(r, opts.Launcher, r.chooseDirectory)
	var assemblyService *Assembly
	kernel = core.New(core.Options{
		Root:     opts.Root,
		Config:   opts.Config,
		Launcher: opts.Launcher,
		Vault:    core.OpenVault(opts.Config),
		Ollama:   vault.NewOllama(),
		OpenTicket: func(url, prompt string) (string, error) {
			return assemblyService.offer(url, prompt)
		},
	}, client)
	reg := kernel.Sessions
	r.register(application.NewService(&BugReports{reports: kernel.BugReports}))
	repos := newRepositories(opts.Config, r.Emit)
	picker := newPicker(opts.Config, reg, repos, opts.Launcher.Signal)
	quit := quitter(kernel, r)
	r.register(application.NewService(&Term{term: kernel.Term}))
	r.register(application.NewService(&Windows{windows: kernel.Windows}))
	r.register(application.NewService(&Sessions{sessions: reg}))
	r.register(application.NewService(repos))
	r.register(application.NewService(&Orgs{cfg: opts.Config}))
	r.register(application.NewService(&Chrome{chrome: kernel.Chrome}))
	updates := newUpdates(opts.Version, r.Emit, http.DefaultClient, updateEndpoint)
	opts.updates = updates
	r.register(application.NewService(updates))
	r.register(application.NewService(&Ollama{ollama: kernel.Ollama}))
	assemblyService = newAssembly(opts.Config, repos, reg, r.Emit, opts.Launcher.Signal, opts.Launcher.Runners)
	opts.assembly = assemblyService
	r.register(application.NewService(assemblyService))
	r.register(application.NewService(picker))
	r.register(application.NewService(newPresenter(r, r.Emit)))
	validateEditor, validateLaunch := validators(opts.Validator)
	r.register(application.NewService(newSettings(
		opts.Config, r.Emit, validateEditor, validateLaunch, opts.LinearCommand, opts.LinearEnvironment,
		withoutTicket(relaunchWith(opts.Relauncher)), quit, reg.Touch, core.ReconfigureVault(kernel.Vault),
		opts.Launcher.Runners, client,
	)))
	relaunch := pendingRelaunch(relaunchWith(opts.Relauncher), assemblyService)
	firstRun := newFirstRun(opts.Config, reg.Touch, relaunch, quit, client.ChooseDirectory, core.ReconfigureVault(kernel.Vault))
	r.register(application.NewService(firstRun))
	opts.placement = newPlacement(opts.Config, reg)
	firstRun.onWelcomed = func() { go opts.placement.checkOnce() }
	r.register(application.NewService(opts.placement))
	r.register(application.NewService(newWebGLLog(webglLogPath(opts.Inspect), kernel.TerminalSession)))
	return run(r, kernel, opts, quit)
}

// quitter is the teardown the main window's OnClose runs, shared with
// Settings.Quit so quitting from the panel is no worse than closing the window.
func quitter(kernel *core.Core, r renderer) func() {
	return sync.OnceFunc(func() {
		kernel.Shutdown()
		r.Quit()
	})
}

func pendingRelaunch(relaunch func(func() (string, string)) error, assembly *Assembly) func() error {
	if relaunch == nil {
		return nil
	}
	return func() error { return relaunch(assembly.pendingTicket) }
}

// withoutTicket relaunches carrying no pending ticket, as a settings restart has none.
func withoutTicket(relaunch func(func() (string, string)) error) func() error {
	if relaunch == nil {
		return nil
	}
	return func() error { return relaunch(nil) }
}

func validators(v Validator) (func([]string) error, func(map[string][]string) error) {
	if v == nil {
		return nil, nil
	}
	return v.ValidateEditor, v.ValidateLaunch
}

func relaunchWith(r Relauncher) func(func() (string, string)) error {
	if r == nil {
		return nil
	}
	return r.Relaunch
}

// run is Run with the renderer and the core already built, so the window
// lifecycle and the control socket are exercised against a double instead of a
// display.
func run(r renderer, kernel *core.Core, opts Options, quit func()) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if opts.updates != nil {
		go opts.updates.watch(ctx, updateInterval)
	}
	if opts.placement != nil {
		go opts.placement.checkOnce()
	}
	if err := kernel.Start(opts.Socket); err != nil {
		return err
	}
	defer kernel.Shutdown()
	if opts.Ticket != "" {
		if opts.assembly == nil {
			return ErrNoConfig
		}
		if _, err := opts.assembly.offer(opts.Ticket, opts.TicketPrompt); err != nil {
			return err
		}
	}
	if err := workbench.Publish(opts.Socket); err != nil {
		return err
	}
	defer workbench.Unpublish(opts.Socket)
	titleRoot := opts.SessionRoot
	if shown := kernel.Sessions.Current(); shown != nil {
		titleRoot = shown.Root()
	}

	// Closing the conversation window ends the app.
	if err := r.Open(windowSpec{
		Name:    mainWindowName,
		Title:   windowTitle(titleRoot),
		URL:     pageURL(opts.Inspect),
		Width:   mainWindowWidth,
		Height:  mainWindowHeight,
		Focus:   true,
		OnClose: quit,
	}); err != nil {
		return err
	}
	if opts.Ticket == "" && opts.SessionRoot != "" {
		if err := kernel.OpenSession(opts.SessionRoot, opts.Resume); err != nil {
			return err
		}
	}
	return r.Run()
}

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

func pageURL(inspect bool) string {
	if inspect {
		return frontendRoot + "?" + inspectQuery
	}
	return frontendRoot
}
