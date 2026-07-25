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
	// The repos pane used to be a generated status.sh; drop stale copies so
	// resumed sessions don't keep an orphaned script around.
	_ = os.Remove(sessionpaths.LegacyStatusScript(dir))
	if err := os.WriteFile(sessionpaths.NotifyScript(dir), []byte(notifyScript), 0o755); err != nil {
		return err
	}
	warning := ""
	if filepath.Base(argv[0]) == codex.Binary && codex.MaxDepth(argv) < codex.RequiredMaxDepth {
		warning = strings.TrimRight(codexDepthWarning, "\n")
	}
	tagline := "Coordinate here; delegate work to subagents."
	if sessionMode(dir) == modeAssistant {
		tagline = "Open-ended session; ask to switch to RPI anytime."
	}
	help := strings.ReplaceAll(helpScript, "@@WARNING@@", warning)
	help = strings.ReplaceAll(help, "@@TAGLINE@@", tagline)
	return os.WriteFile(sessionpaths.HelpScript(dir), []byte(help), 0o755)
}

// workspace describes qrouton's session layout in backend-neutral terms: the
// agent beside a shell and the repo/agent status panes, with the quick-start
// help floating on top.
func workspace(dir, slug string, argv []string, qroutonBin string) mux.Workspace {
	runner := filepath.Base(argv[0])
	return mux.Workspace{
		Slug: slug,
		Dir:  dir,
		Tiled: mux.Node{
			Split: "vertical",
			Children: []mux.Node{
				{Size: "65%", Pane: &mux.Pane{Name: "agent", Command: argv}},
				{Split: "horizontal", Size: "35%", Children: []mux.Node{
					{Pane: &mux.Pane{Name: "shell", Command: []string{"sh", "-lc", strings.TrimSpace(shellIntro)}}},
					{Split: "vertical", Size: "6", Children: []mux.Node{
						{Pane: &mux.Pane{Name: "repos", Command: []string{qroutonBin, "repos", "--session-root", dir}}},
						{Pane: &mux.Pane{Name: "agents", Command: []string{qroutonBin, "agents", "--session-root", dir, "--runner", runner}}},
					}},
				}},
			},
		},
		Floating: []mux.Floating{{
			Pane:     mux.Pane{Name: "qrouton · quick start", Command: []string{shellBin, sessionpaths.HelpScript(dir)}, CloseOnExit: true, Focus: true},
			Geometry: mux.Geometry{X: "27%", Y: "25%", Width: "46%", Height: "35%"},
		}},
	}
}

// Launch stamps the session's support files and workspace, then enters it
// through the configured multiplexer: attaching to (or replacing) an existing
// session, or starting a fresh one. On success the process is replaced by the
// multiplexer and Launch never returns.
func Launch(lp mux.Launcher, dir string, runner Runner, qroutonBin string, editor EditorCommand, resume bool) error {
	slug := filepath.Base(dir)
	argv, env, err := runnerLaunch(runner, qroutonBin, dir, editor, lp.Handle(slug), resume)
	if err != nil {
		return err
	}
	env = mux.WithEnv(env, EditorEnvVar, editor.Marshal())
	if err := writeSupport(dir, argv); err != nil {
		return err
	}
	ws := workspace(dir, slug, argv, qroutonBin)
	if err := lp.Stage(ws); err != nil {
		return err
	}
	state, err := lp.Lookup(slug)
	if err != nil {
		return err
	}
	switch state {
	case mux.SessionLive:
		attach, err := chooseExistingSession(filepath.Base(argv[0]))
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
