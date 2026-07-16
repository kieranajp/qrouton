// qrouton — assemble a multi-repo session (worktrees off local mirrors) and launch an agent runner in it.
// Spec: thoughts/shared/specs/S001-2026-07-15-workspace-harness.md
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/huh"
)

func main() {
	refresh := flag.Bool("refresh", false, "refresh the cached org repo list")
	flag.Parse()

	cfg, err := loadConfig()
	die(err)

	sessions, err := scanSessions(cfg.Root)
	die(err)

	action := "new"
	if len(sessions) > 0 {
		die(huh.NewSelect[string]().
			Title("qrouton").
			Options(
				huh.NewOption("New session", "new"),
				huh.NewOption(fmt.Sprintf("Resume session (%d)", len(sessions)), "resume"),
			).
			Value(&action).Run())
	}

	var dir string
	if action == "resume" {
		dir, err = resumeSession(cfg, sessions)
	} else {
		dir, err = newSession(cfg, *refresh)
	}
	die(err)

	die(launch(cfg, dir))
}

func newSession(cfg *Config, refresh bool) (string, error) {
	repos, err := listRepos(cfg.Org, refresh)
	if err != nil {
		return "", err
	}

	var name, desc, ticket, prefix string
	var picked []string
	opts := make([]huh.Option[string], len(repos))
	for i, r := range repos {
		opts[i] = huh.NewOption(r.Name, r.Name)
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Session name").Value(&name).Validate(func(s string) error {
			if slugify(s) == "" {
				return fmt.Errorf("need a name")
			}
			if _, err := os.Stat(filepath.Join(cfg.Root, slugify(s))); err == nil {
				return fmt.Errorf("session %q already exists", slugify(s))
			}
			return nil
		}),
		huh.NewMultiSelect[string]().Title("Repos").Options(opts...).Filterable(true).Value(&picked).
			Validate(func(v []string) error {
				if len(v) == 0 {
					return fmt.Errorf("pick at least one repo")
				}
				return nil
			}),
		huh.NewInput().Title("Description").Value(&desc),
		huh.NewInput().Title("Ticket URL (optional)").Value(&ticket),
		huh.NewSelect[string]().Title("Branch prefix").
			Options(huh.NewOptions("feat", "fix", "chore", "refactor", "docs", "test")...).
			Value(&prefix),
	))
	if err := form.Run(); err != nil {
		return "", err
	}

	byName := make(map[string]Repo, len(repos))
	for _, r := range repos {
		byName[r.Name] = r
	}
	var sel []Repo
	for _, n := range picked {
		sel = append(sel, byName[n])
	}
	return createSession(cfg, name, desc, ticket, prefix, sel)
}

func resumeSession(cfg *Config, sessions []Manifest) (string, error) {
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].CreatedAt.After(sessions[j].CreatedAt) })
	opts := make([]huh.Option[string], len(sessions))
	for i, s := range sessions {
		opts[i] = huh.NewOption(fmt.Sprintf("%s — %s", s.Slug, s.Description), s.Slug)
	}
	var slug string
	if err := huh.NewSelect[string]().Title("Resume").Options(opts...).Value(&slug).Run(); err != nil {
		return "", err
	}
	for _, s := range sessions {
		if s.Slug == slug {
			return filepath.Join(cfg.Root, slug), ensureWorktrees(cfg, s)
		}
	}
	return "", fmt.Errorf("session %q not found", slug)
}

// launch execs the configured runner (default claude) with cwd = session dir. No return on success.
func launch(cfg *Config, dir string) error {
	if err := stampAssets(dir); err != nil {
		return err
	}
	argv := cfg.Launch[0]
	if len(cfg.Launch) > 1 {
		labels := make([]huh.Option[int], len(cfg.Launch))
		for i, c := range cfg.Launch {
			labels[i] = huh.NewOption(fmt.Sprint(c), i)
		}
		var idx int
		if err := huh.NewSelect[int]().Title("Launch").Options(labels...).Value(&idx).Run(); err != nil {
			return err
		}
		argv = cfg.Launch[idx]
	}
	if os.Getenv("QROUTON_PLAIN") == "" {
		if p, err := exec.LookPath("zellij"); err == nil {
			return launchZellij(p, dir, argv)
		}
		if p, err := exec.LookPath("tmux"); err == nil {
			return launchTmux(p, dir, argv)
		}
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	return execv(path, argv, dir)
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "qrouton:", err)
		os.Exit(1)
	}
}
