package launch

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// Panels are an opinionated multiplexer workspace rather than a bespoke TUI. The
// shell scripts that drive its panes live under scripts/ and are embedded here so
// they read and edit as real scripts; each is written into .qrouton at launch (or,
// for shellIntro and codexDepthWarning, spliced into the generated layout).

// shellIntro greets the shell pane with a shallow tree, then execs an interactive login shell.
//
//go:embed scripts/shell-intro.sh
var shellIntro string

// notifyScript plays a short cross-platform attention sound; it backs both the notify
// MCP tool and the runner's Notification hook. See scripts/notify.sh for the fallbacks.
//
//go:embed scripts/notify.sh
var notifyScript string

// helpScript is the quick-start panel; @@WARNING@@ is replaced with codexDepthWarning or "".
//
//go:embed scripts/help.sh
var helpScript string

// codexDepthWarning warns when Codex's subagent nesting is too shallow.
//
//go:embed scripts/codex-warning.sh
var codexDepthWarning string

// writeSupport writes .qrouton/{help.sh,notify.sh} at launch time, so old
// sessions pick up template changes on resume. Backend layout files are the
// multiplexer adapter's business, staged separately.
func writeSupport(dir string, argv []string) error {
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(sessionpaths.NotifyScript(dir), []byte(notifyScript), scriptMode); err != nil {
		return err
	}
	warning := ""
	if filepath.Base(argv[0]) == codex.Binary && codex.MaxDepth(argv) < codex.RequiredMaxDepth {
		warning = strings.TrimRight(codexDepthWarning, "\n")
	}
	tagline := rpiTagline
	if sessionMode(dir) == modeAssistant {
		tagline = assistantTagline
	}
	help := strings.ReplaceAll(helpScript, warningPlaceholder, warning)
	help = strings.ReplaceAll(help, taglinePlaceholder, tagline)
	return os.WriteFile(sessionpaths.HelpScript(dir), []byte(help), scriptMode)
}

// helpGeometry floats the quick-start panel over the middle of the workspace.
var helpGeometry = mux.Geometry{X: "27%", Y: "25%", Width: "46%", Height: "35%"}

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
// agent beside a shell and the repo/agent status panes, with the quick-start
// help floating on top.
func workspace(dir, slug string, agentArgv []string, runner, qroutonBin string) mux.Workspace {
	return mux.Workspace{
		Slug: slug,
		Dir:  dir,
		Tiled: mux.Node{
			Split: mux.SplitVertical,
			Children: []mux.Node{
				{Size: agentColumnSize, Pane: &mux.Pane{Name: agentPaneName, Command: agentArgv}},
				{Split: mux.SplitHorizontal, Size: reposColumnSize, Children: []mux.Node{
					{Pane: &mux.Pane{Name: shellPaneName, Command: []string{shellBin, shellLoginFlag, strings.TrimSpace(shellIntro)}}},
					{Split: mux.SplitVertical, Size: watchPaneRows, Children: []mux.Node{
						{Pane: &mux.Pane{Name: reposPaneName, Command: []string{qroutonBin, reposSubcommand, sessionRootFlag, dir}}},
						{Pane: &mux.Pane{Name: agentsPaneName, Command: []string{qroutonBin, agentsSubcommand, sessionRootFlag, dir, runnerFlag, runner}}},
					}},
				}},
			},
		},
		Floating: []mux.Floating{{
			Pane:     mux.Pane{Name: helpPaneName, Command: []string{shellBin, sessionpaths.HelpScript(dir)}, CloseOnExit: true, Focus: true},
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
	if err := writeSupport(dir, runner.Command); err != nil {
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
