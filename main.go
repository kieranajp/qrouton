// qrouton — assemble a multi-repo session (worktrees off local mirrors) and launch an agent runner in it.
// See AGENTS.md for the package layout and the invariants a change must hold.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	agentcmd "github.com/kieranajp/qrouton/cmd/agent"
	agentscmd "github.com/kieranajp/qrouton/cmd/agents"
	mcpcmd "github.com/kieranajp/qrouton/cmd/mcp"
	modecmd "github.com/kieranajp/qrouton/cmd/mode"
	onboardcmd "github.com/kieranajp/qrouton/cmd/onboard"
	pickcmd "github.com/kieranajp/qrouton/cmd/pick"
	reposcmd "github.com/kieranajp/qrouton/cmd/repos"
	shellcmd "github.com/kieranajp/qrouton/cmd/shell"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/desktop"
	"github.com/kieranajp/qrouton/internal/github"
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
		ArgsUsage:   appArgsUsage,
		Description: appDescription,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: refreshFlag, Usage: refreshFlagUsage},
			&cli.StringFlag{Name: runnerFlag, Usage: runnerFlagUsage},
			&cli.StringFlag{Name: workbenchSpecFlag, Hidden: true},
		},
		Commands: []*cli.Command{mcpcmd.Command, agentscmd.EventCommand, reposcmd.Command, pickcmd.Command, agentcmd.Command, modecmd.Command, shellcmd.Command, onboardcmd.Command},
		Action:   onboard,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, logPrefix, err)
		os.Exit(1)
	}
}

// onboard is the default action. With no arguments it opens the landing list.
// A single argument naming an existing directory drops into a fresh zero-repo
// scratch session named after it; owner/repo arguments launch an ad-hoc
// session directly. Each of those assembles in the terminal the user ran and
// then hands the workbench to a process of its own.
func onboard(c *cli.Context) error {
	if spec := c.String(workbenchSpecFlag); spec != "" {
		return workbenchProcess(spec)
	}
	args := c.Args().Slice()
	if len(args) == 0 {
		return list(c.String(runnerFlag), c.Bool(refreshFlag))
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(args) == 1 {
		if info, err := os.Stat(args[0]); err == nil && info.IsDir() {
			return launchScratch(cfg, args[0], c.String(runnerFlag))
		}
	}
	sessions, err := session.Scan(cfg.Root)
	if err != nil {
		return err
	}
	return launchAdhoc(cfg, sessions, args, c.String(runnerFlag))
}

// list opens the workbench on the landing list: the window comes up first and
// onboarding draws in its terminal, then hands that same terminal to the agent.
func list(runnerID string, refresh bool) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	socket, err := workbench.NewSocketPath()
	if err != nil {
		return err
	}
	return detach(launch.WorkbenchSpec{
		Socket: socket,
		Argv:   launch.OnboardArgv(bin, socket, runnerID, refresh),
	}, os.Environ())
}

// launchScratch is the directory-argument path: a zero-repo Assistant session
// named after the given directory, with no picker and no network.
func launchScratch(cfg *config.Config, target, runnerID string) error {
	runner, err := pickRunner(cfg, runnerID)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	dir, err := session.Create(cfg, session.ScratchName(abs), "", "", "", session.ModeAssistant, nil, nil)
	if err != nil {
		return err
	}
	return launchRunner(cfg, dir, runner, false)
}

// launchAdhoc skips the picker: it launches an Assistant-mode session with the
// given owner/repo specs active, resuming an existing session of the same name.
func launchAdhoc(cfg *config.Config, sessions []session.Manifest, specs []string, runnerID string) error {
	runner, err := pickRunner(cfg, runnerID)
	if err != nil {
		return err
	}
	repos, err := resolveRepos(cfg, specs)
	if err != nil {
		return err
	}
	slug := session.Slugify(adhocName(repos))
	if m, ok := findSession(sessions, slug); ok {
		if err := session.EnsureWorktrees(cfg, m, printProgress); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, resumingFormat, slug)
		return launchRunner(cfg, filepath.Join(cfg.Root, slug), runner, true)
	}
	selections := make([]session.RepoSelection, len(repos))
	for i, r := range repos {
		selections[i] = session.RepoSelection{Repo: r, Role: session.RepoRoleActive}
	}
	dir, err := session.Create(cfg, adhocName(repos), "", "", adhocBranchPrefix, session.ModeAssistant, selections, printProgress)
	if err != nil {
		return err
	}
	return launchRunner(cfg, dir, runner, false)
}

// printProgress reports assembly on the paths that have no TUI to draw into.
// Outcomes only: git's own clone and fetch progress arrives as ProgressAdvanced
// many times a second, which is a bar in the TUI and a wall of text here.
func printProgress(p session.Progress) {
	if p.Status == session.ProgressCompleted && p.Repo != nil {
		fmt.Fprintf(os.Stderr, progressFormat, p.Repo.ID(), p.Step)
	}
}

// pickRunner resolves the runner headlessly: the requested one if given and
// installed, otherwise the first installed built-in.
func pickRunner(cfg *config.Config, id string) (launch.Runner, error) {
	if id != "" {
		return launch.ByID(cfg, id)
	}
	return launch.FirstInstalled(cfg)
}

// resolveRepos turns owner/repo specs into repositories, preferring the local
// cache and falling back to a direct GitHub lookup for anything not cached.
func resolveRepos(cfg *config.Config, specs []string) ([]github.Repo, error) {
	cached, _, _ := github.CachedRepos(cfg.Orgs)
	var token string
	var repos []github.Repo
	seen := make(map[string]bool)
	for _, spec := range specs {
		owner, name, err := parseRepoSpec(spec)
		if err != nil {
			return nil, err
		}
		id := strings.ToLower(owner + repoSpecSeparator + name)
		if seen[id] {
			continue
		}
		seen[id] = true
		repo, ok := findCachedRepo(cached, owner, name)
		if !ok {
			if token == "" {
				if token, err = github.Token(); err != nil {
					return nil, err
				}
			}
			if repo, err = github.FetchRepo(context.Background(), http.DefaultClient, token, owner, name); err != nil {
				return nil, err
			}
		}
		repos = append(repos, repo)
	}
	if len(repos) == 0 {
		return nil, errNoRepositories
	}
	return repos, nil
}

func findCachedRepo(cached []github.Repo, owner, name string) (github.Repo, bool) {
	for _, r := range cached {
		if strings.EqualFold(r.Org, owner) && strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return github.Repo{}, false
}

func findSession(sessions []session.Manifest, slug string) (session.Manifest, bool) {
	for _, m := range sessions {
		if m.Slug == slug {
			return m, true
		}
	}
	return session.Manifest{}, false
}

// parseRepoSpec accepts "owner/repo" (tolerating a trailing slash or ".git").
func parseRepoSpec(spec string) (string, string, error) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(spec), repoSpecSeparator), repoSpecSeparator)
	if len(parts) != repoSpecParts || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w, got %q", errRepoSpecShape, spec)
	}
	return parts[0], strings.TrimSuffix(parts[1], gitDirSuffix), nil
}

// adhocName derives a session name from its repositories: the repo name for a
// single repo, or the names joined for several.
func adhocName(repos []github.Repo) string {
	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.Name
	}
	return strings.Join(names, adhocNameSeparator)
}

// launchRunner opens the workbench on the session. Prompt stamping is not done
// here: the supervisor running in the conversation terminal stamps on every
// (re)launch, from the manifest as it then stands.
func launchRunner(cfg *config.Config, dir string, r launch.Runner, resume bool) error {
	editor, err := launch.ResolveEditor(cfg.Editor)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	socket, err := workbench.NewSocketPath()
	if err != nil {
		return err
	}
	argv, env, err := launch.Launch(dir, r, bin, socket, editor, resume)
	if err != nil {
		return err
	}
	return detach(launch.WorkbenchSpec{SessionRoot: dir, Socket: socket, Argv: argv}, env)
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

// workbenchProcess is the detached process's own entry: it runs the event loop
// and lives until the session ends. Nothing is re-derived — the parent assembled
// the session and put everything this needs in the spec.
func workbenchProcess(marshalled string) error {
	spec, err := launch.ParseWorkbenchSpec(marshalled)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	return desktop.Run(desktop.Options{
		SessionRoot: spec.SessionRoot,
		Socket:      spec.Socket,
		Argv:        spec.Argv,
		Env:         os.Environ(),
		Shell:       shellArgv(bin),
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

// subject names what was opened in the one line the user gets back.
func subject(sessionRoot string) string {
	if sessionRoot == "" {
		return sessionListSubject
	}
	return filepath.Base(sessionRoot)
}

// shellArgv builds the user shell's command for whichever session the workbench
// settles on, which the landing-list path does not know when it opens.
func shellArgv(bin string) func(string) []string {
	return func(dir string) []string { return launch.ShellArgv(bin, dir) }
}
