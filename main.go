// qrouton — assemble a multi-repo session (worktrees off local mirrors) and launch an agent runner in it.
// Spec: thoughts/shared/specs/S001-2026-07-15-workspace-harness.md
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		die(runMCP(os.Args[2:]))
		return
	}
	refresh := flag.Bool("refresh", false, "refresh the cached org repo list")
	runner := flag.String("runner", "", "coding agent to launch (claude, codex, or opencode)")
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
	die(launchRunner(cfg, request.dir, request.runner, request.resume))
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
	return launchRunner(cfg, dir, r, false)
}

func launchRunner(cfg *Config, dir string, r Runner, resume bool) error {
	if err := stampAssets(dir); err != nil {
		return err
	}
	editor, err := resolveEditor(cfg.Editor)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	return launchZellij(dir, r, bin, editor, resume)
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "qrouton:", err)
		os.Exit(1)
	}
}
