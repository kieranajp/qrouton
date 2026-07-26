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

	agentscmd "github.com/kieranajp/qrouton/cmd/agents"
	mcpcmd "github.com/kieranajp/qrouton/cmd/mcp"
	reposcmd "github.com/kieranajp/qrouton/cmd/repos"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/tui"
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
		},
		Commands: []*cli.Command{mcpcmd.Command, agentscmd.Command, agentscmd.EventCommand, reposcmd.Command},
		Action:   onboard,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, logPrefix, err)
		os.Exit(1)
	}
}

// onboard is the default action. With owner/repo arguments it launches an ad-hoc
// session directly; otherwise it opens the TUI to pick or create one.
func onboard(c *cli.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	sessions, err := session.Scan(cfg.Root)
	if err != nil {
		return err
	}
	if specs := c.Args().Slice(); len(specs) > 0 {
		return launchAdhoc(cfg, sessions, specs, c.String(runnerFlag))
	}
	request, err := tui.Run(cfg, sessions, c.String(runnerFlag), c.Bool(refreshFlag))
	if err != nil || request == nil {
		return err
	}
	return launchRunner(cfg, request.Dir, request.Runner, request.Resume)
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
		if err := session.EnsureWorktrees(cfg, m); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, resumingFormat, slug)
		return launchRunner(cfg, filepath.Join(cfg.Root, slug), runner, true)
	}
	selections := make([]session.RepoSelection, len(repos))
	for i, r := range repos {
		selections[i] = session.RepoSelection{Repo: r, Role: session.RepoRoleActive}
	}
	dir, err := session.Create(cfg, adhocName(repos), "", "", adhocBranchPrefix, session.ModeAssistant, selections,
		func(p session.Progress) {
			if p.Status == session.ProgressCompleted && p.Repo != nil {
				fmt.Fprintf(os.Stderr, progressFormat, p.Repo.ID(), p.Step)
			}
		})
	if err != nil {
		return err
	}
	return launchRunner(cfg, dir, runner, false)
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

func launchRunner(cfg *config.Config, dir string, r launch.Runner, resume bool) error {
	if err := launch.StampAssets(dir); err != nil {
		return err
	}
	editor, err := launch.ResolveEditor(cfg.Editor)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	lp, err := mux.New()
	if err != nil {
		return err
	}
	return launch.Launch(lp, dir, r, bin, editor, resume)
}
