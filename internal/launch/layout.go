package launch

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// Panels are an opinionated multiplexer workspace rather than a bespoke TUI. The
// shell scripts that drive its panes live under scripts/ and are embedded here
// so they read and edit as real scripts; notify.sh is written into .qrouton at
// launch and help.sh under the config dir instead (one global copy).

// notifyScript plays a short cross-platform attention sound; it backs both the notify
// MCP tool and the runner's Notification hook. See scripts/notify.sh for the fallbacks.
//
//go:embed scripts/notify.sh
var notifyScript string

// helpScript is the quick-reference panel, staged once under the config dir
// rather than per session. It reads ./qrouton.json itself for the mode
// tagline; $1, when the caller passes it, is the Codex depth warning.
//
//go:embed scripts/help.sh
var helpScript string

// writeSupport writes .qrouton/notify.sh at launch time (per-session) and
// help.sh under the config dir (one global copy, restaged idempotently so old
// sessions still pick up template changes). Backend layout files are the
// multiplexer adapter's business, staged separately.
func writeSupport(dir string) error {
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(sessionpaths.NotifyScript(dir), []byte(notifyScript), scriptMode); err != nil {
		return err
	}
	helpPath := config.HelpScriptPath()
	if err := os.MkdirAll(filepath.Dir(helpPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(helpPath, []byte(helpScript), scriptMode)
}

// codexWarning is the caveat help.sh shows as $1 when the runner is a Codex
// too shallow to nest subagents; "" means there is nothing to warn about. Only
// the startup panel carries it — Alt-? and the help tool re-summon the panel
// bare, since it is a launch-time concern and not worth repeating.
func codexWarning(argv []string) string {
	if filepath.Base(argv[0]) == codex.Binary && codex.MaxDepth(argv) < codex.RequiredMaxDepth {
		return codexDepthWarning
	}
	return ""
}

// helpGeometry floats the quick-reference panel over the middle of the
// workspace; sized for the full key list, not the old three-line splash. The
// Alt-? binding in zellij-config.kdl mirrors these dimensions exactly, so
// both routes look identical.
var helpGeometry = mux.Geometry{X: "15%", Y: "8%", Width: "70%", Height: "80%"}

// HelpSpawn describes the quick-reference panel as a runtime pane, so the
// startup route, Alt-?, and the help MCP tool all float the same thing. It is
// deliberately not part of the staged layout: Zellij resolves a floating
// pane's percentages against the viewport as it creates the pane, and the
// layout is applied to a session created detached (see mux.Zellij.Start for
// why), whose clientless server reports a ~50x50 default — which is what made
// the startup panel come up squished into a corner of a real terminal.
// Spawning it from inside the session, once someone is attached, sizes it
// against the terminal the user is actually looking at.
func HelpSpawn(dir, warning string) mux.SpawnOptions {
	command := []string{shellBin, config.HelpScriptPath()}
	if warning != "" {
		command = append(command, warning)
	}
	return mux.SpawnOptions{
		Label:       helpPaneName,
		Cwd:         dir,
		Geometry:    helpGeometry,
		CloseOnExit: true,
		Focus:       true,
		Command:     command,
	}
}

// superviseArgv is the agent pane's command: the supervisor that stamps the
// session's assets and launches (and, when signalled, relaunches) the runner.
// The handle and editor cross the exec boundary as flags, the same vocabulary
// the MCP subcommand uses.
func superviseArgv(qroutonBin, dir string, r Runner, handle mux.Handle, editor EditorCommand, resume bool) []string {
	argv := []string{qroutonBin, agentSubcommand, sessionRootFlag, dir, runnerFlag, r.ID,
		muxJSONFlag, handle.Marshal(), editorJSONFlag, editor.Marshal()}
	if resume {
		argv = append(argv, resumeFlag)
	}
	return argv
}

// workspace describes qrouton's session layout in backend-neutral terms: the
// agent beside a shell and the repo/agent status panes, and a full-width
// one-row mode/phase strip along the bottom. The quick-reference panel is not
// here — the supervisor floats it from inside the session instead, for the
// sizing reason HelpSpawn explains.
func workspace(dir, slug string, agentArgv []string, runner, qroutonBin string) mux.Workspace {
	return mux.Workspace{
		Slug:       slug,
		Dir:        dir,
		HelpScript: config.HelpScriptPath(),
		Binary:     qroutonBin,
		Tiled: mux.Node{
			Split: mux.SplitHorizontal,
			Children: []mux.Node{
				{Split: mux.SplitVertical, Children: []mux.Node{
					{Size: agentColumnSize, Pane: &mux.Pane{Name: agentPaneName, Command: agentArgv}},
					{Split: mux.SplitHorizontal, Size: reposColumnSize, Children: []mux.Node{
						{Stacked: true, Children: []mux.Node{
							{Pane: &mux.Pane{Name: shellPaneName, CloseOnExit: true,
								Command: []string{qroutonBin, shellSubcommand, sessionRootFlag, dir}}},
						}},
						{Split: mux.SplitVertical, Size: watchPaneRows, Children: []mux.Node{
							{Pane: &mux.Pane{Name: reposPaneName, Command: []string{qroutonBin, reposSubcommand, sessionRootFlag, dir}}},
							{Pane: &mux.Pane{Name: agentsPaneName, Command: []string{qroutonBin, agentsSubcommand, sessionRootFlag, dir, runnerFlag, runner}}},
						}},
					}},
				}},
				{Size: stripPaneRows, Pane: &mux.Pane{Name: statusPaneName, Borderless: true,
					Command: []string{qroutonBin, statusSubcommand, sessionRootFlag, dir}}},
			},
		},
	}
}

// Launch stamps the session's support files and workspace, then enters it
// through the configured multiplexer: attaching to (or replacing) an existing
// session, or starting a fresh one. On success the process is replaced by the
// multiplexer and Launch never returns. The runner's own argv is not built
// here: the agent pane runs the supervisor, which constructs it from the
// manifest at each (re)launch.
func Launch(lp mux.Launcher, dir string, runner Runner, qroutonBin string, editor EditorCommand, resume bool) error {
	slug := filepath.Base(dir)
	if err := writeSupport(dir); err != nil {
		return err
	}
	argv := superviseArgv(qroutonBin, dir, runner, lp.Handle(slug), editor, resume)
	env := mux.WithEnv(os.Environ(), EditorEnvVar, editor.Marshal())
	ws := workspace(dir, slug, argv, runner.ID, qroutonBin)
	if err := lp.Stage(ws); err != nil {
		return err
	}
	state, err := lp.Lookup(slug)
	if err != nil {
		return err
	}
	switch state {
	case mux.SessionLive:
		attach, err := chooseExistingSession(runner.ID)
		if err != nil {
			return err
		}
		if attach {
			return lp.Attach(ws, env)
		}
		if err := lp.Kill(slug, true); err != nil {
			return err
		}
	case mux.SessionDead:
		// dead session: attach would resurrect the multiplexer's *recorded* state
		// (stale layout, old paths) instead of the freshly-staged one — delete and recreate
		_ = lp.Kill(slug, false)
	}
	return lp.Start(ws, env)
}

func chooseExistingSession(runner string) (bool, error) {
	action := "attach"
	err := huh.NewSelect[string]().Title("Workspace is already running").
		Description("Attach to its current agent, or restart all workspace panes with "+runner+".").
		Options(
			huh.NewOption("Attach existing workspace", "attach"),
			huh.NewOption("Restart workspace", "restart"),
		).Value(&action).Run()
	return action == "attach", err
}
