// qrouton — assemble a multi-repo session (worktrees off local mirrors) and launch an agent runner in it.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	agentcmd "github.com/kieranajp/qrouton/cmd/agent"
	agenteventcmd "github.com/kieranajp/qrouton/cmd/agentevent"
	mcpcmd "github.com/kieranajp/qrouton/cmd/mcp"
	modecmd "github.com/kieranajp/qrouton/cmd/mode"
	shellcmd "github.com/kieranajp/qrouton/cmd/shell"
	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/desktop"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/ticket"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

func main() {
	prepareEnvironment()
	app := &cli.App{
		Name:        appName,
		Usage:       appUsage,
		Description: appDescription,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: runnerFlag, Usage: runnerFlagUsage},
			&cli.StringFlag{Name: linearIssueFlag, Usage: linearIssueFlagUsage},
			&cli.StringFlag{Name: ticketFlag, Usage: ticketFlagUsage},
			&cli.StringFlag{Name: workbenchSpecFlag, Hidden: true},
		},
		Commands: []*cli.Command{mcpcmd.Command, agenteventcmd.EventCommand, agentcmd.Command, modecmd.Command, shellcmd.Command},
		Action:   open,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, logPrefix, err)
		os.Exit(1)
	}
}

// open is the default action: the workbench, on the session last shown or on no
// session at all. Assembling one is the window's own job.
func open(c *cli.Context) error {
	if spec := c.String(workbenchSpecFlag); spec != "" {
		return workbenchProcess(spec)
	}
	if arg := c.Args().First(); arg != "" {
		return fmt.Errorf("%w: %q", errNoSessionArguments, arg)
	}
	reference, prompt, err := offeredTicket(c)
	if err != nil {
		return err
	}
	return workbench.WithLaunchLock(func() error { return openLocked(c, reference, prompt) })
}

// offeredTicket is the ticket this invocation is opening on, canonical and ready
// to dedupe against. --linear-issue is the name Linear Desktop already holds in
// users' coding-tools.json, and it alone carries a free-text prompt.
func offeredTicket(c *cli.Context) (string, string, error) {
	flag, prompt := ticketFlag, ""
	switch {
	case c.IsSet(linearIssueFlag):
		flag, prompt = linearIssueFlag, os.Getenv(linearPromptEnvVar)
	case !c.IsSet(ticketFlag):
		return "", "", nil
	}
	canonical, err := ticket.Canonical(c.String(flag))
	if err != nil {
		return "", "", err
	}
	return canonical, prompt, nil
}

func openLocked(c *cli.Context, linearIssue, linearPrompt string) error {
	discovery := discoverProcess()
	if linearIssue != "" {
		if discovery.Socket != "" {
			_, err := workbench.OpenLinearIssue(
				context.Background(), discovery.Socket, linearIssue, linearPrompt,
			)
			return err
		}
		if discovery.Legacy {
			return errLegacyWorkbench
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		socket, err := workbench.NewSocketPath()
		if err != nil {
			return err
		}
		return detachProcess(launch.WorkbenchSpec{
			Socket: socket, Runner: c.String(runnerFlag), Editor: editorFor(cfg),
			LinearIssue: linearIssue, LinearPrompt: linearPrompt,
		}, os.Environ())
	}
	// There is one workbench, and it opens on a session: two of them would each
	// believe they were the only one holding that session's supervisor.
	if discovery.Socket != "" || discovery.Legacy {
		return errWorkbenchRunning
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	sessions, err := session.Scan(cfg.Root)
	if err != nil {
		return err
	}
	if last, ok := session.Preferred(cfg.Root, sessions); ok {
		runner, err := pickRunner(cfg, c.String(runnerFlag))
		if err != nil {
			return err
		}
		return launchRunner(cfg, filepath.Join(cfg.Root, last.Slug), runner, true)
	}
	socket, err := workbench.NewSocketPath()
	if err != nil {
		return err
	}
	return detach(launch.WorkbenchSpec{
		Socket: socket,
		Runner: c.String(runnerFlag),
		Editor: editorFor(cfg),
	}, os.Environ())
}

var (
	detachProcess   = detach
	discoverProcess = workbench.Discover
)

func pickRunner(cfg *config.Config, id string) (launch.Runner, error) {
	if id != "" {
		return launch.ByID(cfg, id)
	}
	return launch.FirstInstalled(cfg)
}

// launchRunner opens the workbench on the session. The workbench builds the
// agent's command as it boots it, and that supervisor stamps the prompts.
func launchRunner(cfg *config.Config, dir string, r launch.Runner, resume bool) error {
	socket, err := workbench.NewSocketPath()
	if err != nil {
		return err
	}
	return detachProcess(launch.WorkbenchSpec{
		SessionRoot: dir, Socket: socket, Runner: r.ID, Resume: resume,
		Editor: editorFor(cfg),
	}, os.Environ())
}

// editorFor resolves the editor a session's windows open files with. Failing to
// is not fatal: qrouton renders the documents it can itself, so an unresolvable
// editor costs only the files it cannot, and must not keep the window shut.
func editorFor(cfg *config.Config) launch.EditorCommand {
	editor, _ := launch.ResolveEditor(cfg.Editor)
	return editor
}

// detach hands the workbench to a process of its own and returns as soon as it
// is serving, so the terminal comes back with the windows still up. Assembly has
// already narrated to this terminal by the time we get here; the workbench's own
// output has nowhere to go but its log.
func detach(spec launch.WorkbenchSpec, env []string) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	log := workbenchLog(spec)
	if err := launch.Detach(launch.WorkbenchArgv(bin, spec), env, spec.Socket, log); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, openedFormat, subject(spec.SessionRoot), log)
	return nil
}

// workbenchProcess is the detached process's own entry: the event loop, until its
// window closes. It reads the config because it can boot any session under the
// root, each needing a runner and a socket of its own.
func workbenchProcess(marshalled string) error {
	spec, err := launch.ParseWorkbenchSpec(marshalled)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ports := workbenchPorts{cfg: cfg, bin: bin, spec: spec, env: os.Environ()}
	return desktop.Run(desktop.Options{
		Icon:         applicationIcon,
		SessionRoot:  spec.SessionRoot,
		Resume:       spec.Resume,
		Root:         cfg.Root,
		Socket:       spec.Socket,
		LinearIssue:  spec.LinearIssue,
		LinearPrompt: spec.LinearPrompt,
		LinearCommand: []string{
			bin,
			"--" + linearIssueFlag,
		},
		LinearEnvironment: []string{linearPromptEnvVar},
		Env:               os.Environ(),
		Config:            cfg,
		Launcher:          ports,
		Validator:         ports,
		Relauncher:        ports,
	})
}

// workbenchPorts is launch, as the workbench reaches it. desktop names the ports
// and this answers them, so the package linked against a webview never links
// launch.
type workbenchPorts struct {
	cfg  *config.Config
	bin  string
	spec launch.WorkbenchSpec
	env  []string
}

// Agent builds a session's supervisor command when the workbench boots it, so
// the socket it is served on and the manifest it reads are the current ones. A
// session assembled in the overlay names its own agent; anything else takes the
// workbench's.
func (p workbenchPorts) Agent(req desktop.AgentRequest) (desktop.AgentCommand, error) {
	id := req.RunnerID
	if id == "" {
		id = p.spec.Runner
	}
	runner, err := pickRunner(p.cfg.Snapshot(), id)
	if err != nil {
		return desktop.AgentCommand{}, err
	}
	argv, env, err := launch.Launch(req.SessionRoot, runner, p.bin, req.Socket, p.spec.Editor, req.Resume)
	if err != nil {
		return desktop.AgentCommand{}, err
	}
	return desktop.AgentCommand{Argv: argv, Env: env, RunnerID: runner.ID}, nil
}

// Shell and Reveal are built per session, because the landing-list path does not
// know which one the workbench settles on when it opens.
func (p workbenchPorts) Shell(sessionRoot string) []string {
	return launch.ShellArgv(p.bin, sessionRoot)
}

func (p workbenchPorts) Reveal(sessionRoot string) []string { return launch.RevealArgv(sessionRoot) }

// Document reaches the same decision the agent's file tool does.
func (p workbenchPorts) Document(sessionRoot, name string) (workbench.WindowOptions, error) {
	return launch.DocumentWindow(sessionRoot, name, p.spec.Editor, workbench.LineSpan{})
}

func (p workbenchPorts) Runners() ([]assembly.Runner, error) {
	runners, err := launch.Runners(p.cfg.Snapshot())
	if err != nil {
		return nil, err
	}
	out := make([]assembly.Runner, len(runners))
	for i, r := range runners {
		out[i] = assembly.Runner{ID: r.ID, Label: r.Label, Installed: r.Path != ""}
	}
	return out, nil
}

func (p workbenchPorts) Signal(sessionRoot string) { launch.SignalSupervisor(sessionRoot) }

func (p workbenchPorts) ValidateEditor(argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	_, err := launch.ResolveEditor(argv)
	return err
}

func (p workbenchPorts) ValidateLaunch(overrides map[string][]string) error {
	_, err := launch.Runners(&config.Config{Launch: overrides})
	return err
}

// Relaunch replaces this workbench with one that loads the config again, on no
// session. Detach returns only once the child answers, so the two overlap for
// that wait — safe because the successor holds no session, and so claims no
// supervisor the caller might still own.
func (p workbenchPorts) Relaunch(linearIssue func() (string, string)) error {
	return workbench.WithLaunchLock(func() error {
		socket, err := workbench.NewSocketPath()
		if err != nil {
			return err
		}
		next := p.spec
		next.SessionRoot, next.Resume, next.Socket = "", false, socket
		if linearIssue != nil {
			next.LinearIssue, next.LinearPrompt = linearIssue()
		}
		return launch.Detach(launch.WorkbenchArgv(p.bin, next), config.WithoutOverrides(p.env),
			socket, workbenchLog(next))
	})
}

// workbenchLog is where the detached process's stdio lands: inside the session
// when there is one, and beside its control socket on the landing-list path,
// which has not chosen a session yet.
func workbenchLog(spec launch.WorkbenchSpec) string {
	if spec.SessionRoot == "" {
		return workbench.ProcessLog(spec.Socket)
	}
	return sessionpaths.WorkbenchLog(spec.SessionRoot)
}

func subject(sessionRoot string) string {
	if sessionRoot == "" {
		return noSessionSubject
	}
	return filepath.Base(sessionRoot)
}
