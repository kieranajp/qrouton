package launch

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// Panels are an opinionated multiplexer workspace rather than a bespoke TUI. The
// shell scripts that drive its panes live under scripts/ and are embedded here so
// they read and edit as real scripts; notify.sh is written into .qrouton at
// launch, help.sh under the config dir instead (one global copy), and
// shellIntro is spliced straight into the generated layout.

// shellIntro greets the shell pane with a shallow tree, then execs an interactive login shell.
//
//go:embed scripts/shell-intro.sh
var shellIntro string

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
// sessions still pick up template changes). It returns the Codex depth
// warning text for the startup pane to pass along as help.sh's $1; "" means
// no warning. Backend layout files are the multiplexer adapter's business,
// staged separately.
func writeSupport(dir string, argv []string) (string, error) {
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(sessionpaths.NotifyScript(dir), []byte(notifyScript), scriptMode); err != nil {
		return "", err
	}
	helpPath := config.HelpScriptPath()
	if err := os.MkdirAll(filepath.Dir(helpPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(helpPath, []byte(helpScript), scriptMode); err != nil {
		return "", err
	}
	warning := ""
	if filepath.Base(argv[0]) == codex.Binary && codex.MaxDepth(argv) < codex.RequiredMaxDepth {
		warning = codexDepthWarning
	}
	return warning, nil
}

// helpGeometry floats the quick-reference panel over the middle of the
// workspace; sized for the full key list, not the old three-line splash. The
// Alt-? binding in zellij-config.kdl mirrors these dimensions exactly, so
// both routes look identical.
var helpGeometry = mux.Geometry{X: "15%", Y: "8%", Width: "70%", Height: "80%"}

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
// agent beside a shell and the repo/agent status panes, a full-width one-row
// mode/phase strip along the bottom, and the quick-reference panel floating
// on top. warning, when non-empty, is the Codex depth warning passed to
// help.sh as $1 — only the startup route carries one; Alt-? re-summons the
// same script bare.
func workspace(dir, slug string, agentArgv []string, runner, qroutonBin, warning string) mux.Workspace {
	helpPath := config.HelpScriptPath()
	helpCommand := []string{shellBin, helpPath}
	if warning != "" {
		helpCommand = append(helpCommand, warning)
	}
	return mux.Workspace{
		Slug:       slug,
		Dir:        dir,
		HelpScript: helpPath,
		Tiled: mux.Node{
			Split: mux.SplitHorizontal,
			Children: []mux.Node{
				{Split: mux.SplitVertical, Children: []mux.Node{
					{Size: agentColumnSize, Pane: &mux.Pane{Name: agentPaneName, Command: agentArgv}},
					{Split: mux.SplitHorizontal, Size: reposColumnSize, Children: []mux.Node{
						{Pane: &mux.Pane{Name: shellPaneName, Command: []string{shellBin, shellLoginFlag, strings.TrimSpace(shellIntro)}}},
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
		Floating: []mux.Floating{{
			Pane:     mux.Pane{Name: helpPaneName, Command: helpCommand, CloseOnExit: true, Focus: true},
			Geometry: helpGeometry,
		}},
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
	warning, err := writeSupport(dir, runner.Command)
	if err != nil {
		return err
	}
	argv := superviseArgv(qroutonBin, dir, runner, lp.Handle(slug), editor, resume)
	env := mux.WithEnv(os.Environ(), EditorEnvVar, editor.Marshal())
	ws := workspace(dir, slug, argv, runner.ID, qroutonBin, warning)
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
