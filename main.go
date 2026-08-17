// qrouton — assemble a multi-repo session (worktrees off local mirrors) and launch an agent runner in it.
// See AGENTS.md for the package layout and the invariants a change must hold.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	agentcmd "github.com/kieranajp/qrouton/cmd/agent"
	agentscmd "github.com/kieranajp/qrouton/cmd/agents"
	mcpcmd "github.com/kieranajp/qrouton/cmd/mcp"
	modecmd "github.com/kieranajp/qrouton/cmd/mode"
	reposcmd "github.com/kieranajp/qrouton/cmd/repos"
	shellcmd "github.com/kieranajp/qrouton/cmd/shell"
	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/desktop"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        appName,
		Usage:       appUsage,
		Description: appDescription,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: runnerFlag, Usage: runnerFlagUsage},
			&cli.StringFlag{Name: workbenchSpecFlag, Hidden: true},
		},
		Commands: []*cli.Command{mcpcmd.Command, agentscmd.EventCommand, reposcmd.Command, agentcmd.Command, modecmd.Command, shellcmd.Command},
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
	// There is one workbench, and it opens on a session: two of them would each
	// believe they were the only one holding that session's supervisor.
	if workbench.Running() {
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
	if last, ok := lastShown(cfg.Root, sessions); ok {
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
	// A missing editor costs the document chip, and must not keep the window shut.
	editor, _ := launch.ResolveEditor(cfg.Editor)
	return detach(launch.WorkbenchSpec{
		Socket: socket,
		Runner: c.String(runnerFlag),
		Editor: editor,
	}, os.Environ())
}

// lastShown is the session to come back to: the one the workbench showed most
// recently, or the newest when none of them carries a stamp.
func lastShown(root string, sessions []session.Manifest) (session.Manifest, bool) {
	var best session.Manifest
	var stamp time.Time
	for _, m := range sessions {
		at, ok := session.LastOpened(filepath.Join(root, m.Slug))
		if ok && (best.Slug == "" || at.After(stamp)) {
			best, stamp = m, at
		}
	}
	if best.Slug != "" {
		return best, true
	}
	for _, m := range sessions {
		if best.Slug == "" || m.CreatedAt.After(best.CreatedAt) {
			best = m
		}
	}
	return best, best.Slug != ""
}

// pickRunner resolves the runner headlessly: the requested one if given and
// installed, otherwise the first installed built-in.
func pickRunner(cfg *config.Config, id string) (launch.Runner, error) {
	if id != "" {
		return launch.ByID(cfg, id)
	}
	return launch.FirstInstalled(cfg)
}

// launchRunner opens the workbench on the session. The workbench builds the
// agent's command as it boots it, and that supervisor stamps the prompts.
func launchRunner(cfg *config.Config, dir string, r launch.Runner, resume bool) error {
	editor, err := launch.ResolveEditor(cfg.Editor)
	if err != nil {
		return err
	}
	socket, err := workbench.NewSocketPath()
	if err != nil {
		return err
	}
	return detach(launch.WorkbenchSpec{
		SessionRoot: dir, Socket: socket, Runner: r.ID, Resume: resume,
		Editor: editor,
	}, os.Environ())
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
	return desktop.Run(desktop.Options{
		SessionRoot: spec.SessionRoot,
		Resume:      spec.Resume,
		Root:        cfg.Root,
		Socket:      spec.Socket,
		Env:         os.Environ(),
		Agent:       agentCommand(cfg, bin, spec.Runner, spec.Editor),
		Shell:       shellArgv(bin),
		Reveal:      launch.RevealArgv,
		Document:    documentWindow(spec.Editor),
		Config:      cfg,
		Runners:     assemblyRunners(cfg),
		Signal:      launch.SignalSupervisor,
		Relaunch:    relaunchWorkbench(bin, spec, os.Environ()),
		ValidateEditor: func(argv []string) error {
			if len(argv) == 0 {
				return nil
			}
			_, err := launch.ResolveEditor(argv)
			return err
		},
		ValidateLaunch: func(overrides map[string][]string) error {
			_, err := launch.Runners(&config.Config{Launch: overrides})
			return err
		},
	})
}

// relaunchWorkbench replaces this workbench with one that loads the config
// again, on no session. Detach returns only once the child answers, so the two
// overlap for that wait — safe because the successor holds no session, and so
// claims no supervisor the caller might still own.
func relaunchWorkbench(bin string, spec launch.WorkbenchSpec, env []string) func() error {
	return func() error {
		socket, err := workbench.NewSocketPath()
		if err != nil {
			return err
		}
		next := spec
		next.SessionRoot, next.Resume, next.Socket = "", false, socket
		return launch.Detach(launch.WorkbenchArgv(bin, next), config.WithoutOverrides(env),
			socket, workbenchLog(next))
	}
}

// agentCommand builds a session's supervisor command when the workbench boots it,
// so the socket it is served on and the manifest it reads are the current ones.
// A session assembled in the overlay names its own agent; anything else takes the
// workbench's.
func agentCommand(cfg *config.Config, bin, workbenchRunner string, editor launch.EditorCommand) func(string, string, string, bool) ([]string, []string, error) {
	return func(sessionRoot, socket, runnerID string, resume bool) ([]string, []string, error) {
		if runnerID == "" {
			runnerID = workbenchRunner
		}
		runner, err := pickRunner(cfg, runnerID)
		if err != nil {
			return nil, nil, err
		}
		return launch.Launch(sessionRoot, runner, bin, socket, editor, resume)
	}
}

// assemblyRunners maps launch's runners onto the row the overlay draws, which is
// how desktop offers agents without importing launch.
func assemblyRunners(cfg *config.Config) func() ([]assembly.Runner, error) {
	return func() ([]assembly.Runner, error) {
		runners, err := launch.Runners(cfg)
		if err != nil {
			return nil, err
		}
		out := make([]assembly.Runner, len(runners))
		for i, r := range runners {
			out[i] = assembly.Runner{ID: r.ID, Label: r.Label, Installed: r.Path != ""}
		}
		return out, nil
	}
}

// documentWindow reaches the same decision the agent's file tool does.
func documentWindow(editor launch.EditorCommand) func(string, string) (workbench.WindowOptions, error) {
	return func(sessionRoot, name string) (workbench.WindowOptions, error) {
		return launch.DocumentWindow(sessionRoot, name, editor, workbench.LineSpan{})
	}
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

// subject names what was opened in the one line the user gets back.
func subject(sessionRoot string) string {
	if sessionRoot == "" {
		return noSessionSubject
	}
	return filepath.Base(sessionRoot)
}

// shellArgv builds the user shell's command for whichever session the workbench
// settles on, which the landing-list path does not know when it opens.
func shellArgv(bin string) func(string) []string {
	return func(dir string) []string { return launch.ShellArgv(bin, dir) }
}
