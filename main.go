// qrouton — assemble a multi-repo session (worktrees off local mirrors) and launch an agent runner in it.
// Spec: thoughts/shared/specs/S001-2026-07-15-workspace-harness.md
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	refresh := flag.Bool("refresh", false, "refresh the cached org repo list")
	runner := flag.String("runner", "", "coding agent to launch (claude, codex, opencode, agy, pi, or configured command)")
	flag.Parse()

	cfg, err := loadConfig()
	die(err)

	sessions, err := scanSessions(cfg.Root)
	die(err)

	request, err := runOnboarding(cfg, sessions, *runner, *refresh)
	die(err)
	if request == nil {
		return
	}
	die(launchRunner(cfg, request.dir, request.runner))
}

func repoID(r Repo) string { return r.Org + "/" + r.Name }

// launch selects and execs a detected runner with cwd = session dir. No return on success.
func launch(cfg *Config, dir, requestedRunner string) error {
	if err := stampAssets(dir); err != nil {
		return err
	}
	r, err := chooseRunner(cfg, requestedRunner)
	if err != nil {
		return err
	}
	argv := runnerArgv(r)
	if os.Getenv("QROUTON_PLAIN") == "" {
		if p, err := exec.LookPath("zellij"); err == nil {
			return launchZellij(p, dir, argv)
		}
		if p, err := exec.LookPath("tmux"); err == nil {
			return launchTmux(p, dir, argv)
		}
	}
	return execv(r.Path, argv, dir)
}

func launchRunner(cfg *Config, dir string, r Runner) error {
	if err := stampAssets(dir); err != nil {
		return err
	}
	argv := runnerArgv(r)
	if os.Getenv("QROUTON_PLAIN") == "" {
		if p, err := exec.LookPath("zellij"); err == nil {
			return launchZellij(p, dir, argv)
		}
		if p, err := exec.LookPath("tmux"); err == nil {
			return launchTmux(p, dir, argv)
		}
	}
	return execv(r.Path, argv, dir)
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "qrouton:", err)
		os.Exit(1)
	}
}
