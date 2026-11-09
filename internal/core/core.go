package core

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type Options struct {
	// Root is where sessions live, so the rail can boot one this workbench has
	// never run.
	Root     string
	Config   *config.Config
	Launcher Launcher
	// Vault is the thoughts index the control socket searches; nil serves none.
	Vault *vault.Set
	// Ollama is the embedding server Settings reports on; nil leaves Core.Ollama unset.
	Ollama OllamaClient
	// OpenTicket answers the process socket's open-ticket request, which is
	// refused when it is nil.
	OpenTicket func(url, prompt string) (string, error)
}

type Core struct {
	Sessions   *Sessions
	Term       *Term
	Windows    *Windows
	BugReports *BugReports
	Chrome     *Chrome
	Ollama     *Ollama
	Vault      *vault.Set

	opts   Options
	client Client
	ctx    context.Context
	cancel context.CancelFunc
	server io.Closer
	once   sync.Once
}

func New(opts Options, client Client) *Core {
	ctx, cancel := context.WithCancel(context.Background())
	reg := newSessions()
	windows := newWindows(client.Emit, reg)
	c := &Core{
		Sessions:   reg,
		Term:       newTerm(reg, client.Emit),
		Windows:    windows,
		BugReports: newBugReports(reg),
		Chrome:     newChrome(client.Emit),
		Vault:      opts.Vault,
		opts:       opts,
		client:     client,
		ctx:        ctx,
		cancel:     cancel,
	}
	if opts.Ollama != nil {
		c.Ollama = NewOllama(opts.Ollama, client.Emit)
	}
	c.wire()
	return c
}

func (c *Core) wire() {
	reg, windows, opts, client := c.Sessions, c.Windows, c.opts, c.client
	launcher := opts.Launcher
	shell := &shellWindow{windows: windows, launcher: launcher}
	overviewTab := &overviewWindow{windows: windows}

	reg.boot = booting{
		root: func(slug string) string { return session.Resumable(opts.Root, slug) },
		stickers: func() []session.Sticker {
			var ids []session.Sticker
			for _, s := range opts.Config.EffectiveStickers() {
				ids = append(ids, session.Sticker(s.ID))
			}
			return ids
		},
		agent: agentCommand(launcher),
		serve: func(state *SessionState, socket string) (io.Closer, error) {
			ring := ringer(opts.Config, client, state, func(s *SessionState) bool {
				return reg.Current() == s && client.Focused()
			})
			notify := attend(state, reg.Current, reg.Touch, ring)
			return serveControl(socket, windows, state, controlHooks{
				bugReports: c.BugReports,
				vault:      c.Vault,
				thoughts:   opts.Config.ThoughtsRoots,
				attention: func(value string, generation uint64) {
					notify(state.agents.attention(generation, value))
				},
				ring: ring,
				generation: func(req workbench.RunnerGenerationRequest) {
					if req.Provider != state.provider {
						return
					}
					if state.agents.begin(req.Provider, req.Generation) {
						reg.Touch()
					}
				},
				lifecycle: func(req workbench.DelegatedLifecycleRequest) {
					if state.agents.lifecycle(req) {
						reg.Touch()
					}
				},
				agentEvent: func(req workbench.AgentEventRequest) {
					notify(state.agents.apply(req))
				},
				picker: reg.QueuePicker,
			})
		},
		shown: func(state *SessionState) {
			client.SessionShown(state.Root())
			if state == nil {
				return
			}
			if root := state.Root(); root != "" {
				_ = session.MarkOpened(root, time.Now())
			}
			overviewTab.open(state)
		},
		teardown: windows.stop,
		// The destructive paths address a session by the sessions root and the
		// manifest's slug, so opts.Root is what they take and not the session's own.
		uncommitted: func(root string) ([]string, error) { return session.Uncommitted(opts.Root, root) },
		cleanup:     func(root string) error { return session.Remove(opts.Root, root) },
		reveal:      client.Reveal,
		copyImage:   client.CopyImage,
		copyFile:    client.CopyFile,
	}
	reg.trees = newAgentTrees(client.Emit, agentTreeInterval, reg.Current, sessionTree)
	c.Chrome.answerTrees(func(slug string) status.AgentTree {
		if state := reg.BySlug(slug); state != nil {
			return sessionTree(state)
		}
		return agentTree(agentActivitySnapshot{}, manifestRunner(session.Resumable(opts.Root, slug)), "")
	})

	// A supervisor exiting ends only its own session, and a failed one keeps its
	// terminal readable.
	c.Term.whenChildExits(func(state *SessionState, code int) {
		if state.agents.exitWithProvider(state.provider, code) {
			reg.Touch()
		}
		if code == 0 {
			reg.retire(state)
		}
	})

	windows.newShell = func() (string, error) { return shell.another(reg.Current()) }
	windows.newDocument = func(name string) (string, error) {
		return openDocument(windows, reg.Current(), launcher, name)
	}
}

// Start serves the process socket and starts the watchers that run for the
// core's lifetime.
func (c *Core) Start(socket string) error {
	hooks := controlHooks{picker: c.Sessions.QueuePicker}
	if c.opts.OpenTicket != nil {
		hooks.openTicket = c.opts.OpenTicket
		hooks.focus = c.client.Raise
	}
	server, err := serveControl(socket, c.Windows, nil, hooks)
	if err != nil {
		return err
	}
	c.server = server
	go watchChrome(c.ctx, c.Sessions, c.opts.Root, c.opts.Config, c.Chrome.publish)
	if c.Vault != nil {
		go c.Vault.Warm(c.ctx)
	}
	if c.Ollama != nil {
		c.Ollama.Bind(c.ctx)
	}
	return nil
}

// OpenSession boots the session at root and puts it on screen.
func (c *Core) OpenSession(root string, resume bool) error {
	opened, err := c.Sessions.start(root, "", resume)
	if err != nil {
		return err
	}
	c.Sessions.reveal(opened)
	return nil
}

// Shutdown stops every tab while its session is still there, then every
// session, then the process socket and the watchers. Only the first call does
// anything.
func (c *Core) Shutdown() {
	c.once.Do(func() {
		c.Windows.stopAll()
		c.Sessions.stopAll()
		if c.server != nil {
			_ = c.server.Close()
		}
		c.cancel()
	})
}

// TerminalSession names the session a conversation or terminal tab belongs to.
func (c *Core) TerminalSession(terminal string) string {
	if state, ok := c.Sessions.ByTerminal(terminal); ok {
		return state.Slug()
	}
	if window, ok := c.Windows.window(terminal); ok && window.session != nil {
		return window.session.Slug()
	}
	return ""
}

// ringer is the one place a session's chime is rung from, so the setting, the
// session in front of the user, and the once-until-you-type rule cannot be
// skipped by any caller.
func ringer(cfg *config.Config, client Client, state *SessionState, watched func(*SessionState) bool) func() {
	return func() {
		if (cfg != nil && cfg.Snapshot().Quiet) || watched(state) || !state.agents.ring() {
			return
		}
		client.Chime(state.Root())
	}
}

// attend is the one path a session's attention change takes: mark it seen if
// it is on screen, push the chrome, and ring when the change asks for it.
func attend(state *SessionState, shown func() *SessionState, touch, ring func()) func(changed, alert bool) {
	return func(changed, alert bool) {
		if !changed {
			return
		}
		if shown() == state {
			state.agents.seen()
		}
		touch()
		if alert {
			ring()
		}
	}
}

func sessionTree(state *SessionState) status.AgentTree {
	snapshot := state.agents.snapshot()
	fallback := ""
	if snapshot.Provider == "" {
		fallback = manifestRunner(state.Root())
	}
	return agentTree(snapshot, fallback, state.Root())
}

func manifestRunner(root string) string {
	if root == "" {
		return ""
	}
	m, err := session.Load(root)
	if err != nil {
		return ""
	}
	return m.Runner
}

// overviewWindow opens the session's Overview tab. Adoption may call it again once
// the session is known, which must not leave two.
type overviewWindow struct {
	windows *Windows
}

func (a *overviewWindow) open(owner *SessionState) {
	if owner == nil {
		return
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.overviewWindow != "" && a.windows.exists(owner.overviewWindow) {
		return
	}
	id, err := a.windows.openStructural(owner, workbench.WindowOptions{
		Kind: workbench.KindOverview, Label: overviewWindowLabel, Icon: workbench.IconTree,
	})
	if err != nil {
		return
	}
	owner.overviewWindow = id
}

// shellWindow opens the user shells the session asks for. The numbering is the
// session's, so a second one restarts it.
type shellWindow struct {
	windows  *Windows
	launcher Launcher
}

// another is the tab strip's + button.
func (s *shellWindow) another(owner *SessionState) (string, error) {
	if owner == nil {
		return "", ErrNoShellCommand
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return s.spawn(owner)
}

// spawn is called with the session's lock held, so the numbering matches the
// order the tabs open in.
func (s *shellWindow) spawn(owner *SessionState) (string, error) {
	root := owner.Root()
	if root == "" || s.launcher == nil {
		return "", ErrNoShellCommand
	}
	argv := s.launcher.Shell(root)
	if len(argv) == 0 {
		return "", ErrNoShellCommand
	}
	owner.shells++
	return s.windows.openShell(owner, workbench.WindowOptions{
		Kind:        workbench.KindTerminal,
		Label:       shellLabel(owner.shells),
		Icon:        workbench.IconShell,
		Cwd:         root,
		Command:     argv,
		CloseOnExit: true,
	})
}

func openDocument(windows *Windows, owner *SessionState, launcher Launcher, name string) (string, error) {
	if owner == nil || launcher == nil {
		return "", ErrNoEditorCommand
	}
	opts, err := launcher.Document(owner.Root(), name)
	if err != nil {
		return "", err
	}
	return windows.openStructural(owner, opts)
}

func shellLabel(n int) string {
	if n <= 1 {
		return shellWindowLabel
	}
	return fmt.Sprintf(shellWindowLabelNumbers, n)
}

func agentCommand(launcher Launcher) func(AgentRequest) (AgentCommand, error) {
	if launcher == nil {
		return nil
	}
	return launcher.Agent
}
