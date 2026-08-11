package launch

import (
	_ "embed"
	"os"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// notifyScript plays a short cross-platform attention sound; it backs both the
// notify MCP tool and the runner's Notification hook.
//
//go:embed scripts/notify.sh
var notifyScript string

func writeSupport(dir string) error {
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sessionpaths.NotifyScript(dir), []byte(notifyScript), scriptMode)
}

// superviseArgv is the conversation terminal's command: the supervisor that
// stamps the session's assets and launches (and, when signalled, relaunches)
// the runner.
func superviseArgv(qroutonBin, dir string, r Runner, handle workbench.Handle, editor EditorCommand, resume bool) []string {
	argv := []string{qroutonBin, agentSubcommand, sessionRootFlag, dir, runnerFlag, r.ID,
		workbenchJSONFlag, handle.Marshal(), editorJSONFlag, editor.Marshal()}
	if resume {
		argv = append(argv, resumeFlag)
	}
	return argv
}

// OnboardArgv is the conversation terminal's first command when no session has
// been chosen yet: the landing list runs in the PTY and replaces itself with the
// supervisor there, so a session has one long-lived terminal rather than two.
func OnboardArgv(qroutonBin, socket, runnerID string, refresh bool) []string {
	argv := []string{qroutonBin, onboardSubcommand, socketFlag, socket}
	if runnerID != "" {
		argv = append(argv, runnerFlag, runnerID)
	}
	if refresh {
		argv = append(argv, refreshFlag)
	}
	return argv
}

// ShellArgv is a user shell window's command, rooted in the session.
func ShellArgv(qroutonBin, dir string) []string {
	return []string{qroutonBin, shellSubcommand, sessionRootFlag, dir}
}

// PickerArgv is the repository picker's command. No --escalate: the workbench's
// button adds repositories to the session as it stands, whatever mode that is.
func PickerArgv(qroutonBin, dir string) []string {
	return []string{qroutonBin, pickSubcommand, sessionRootFlag, dir}
}

// Launch stamps the session's support files and returns what the workbench runs
// in its conversation terminal: the supervisor's argv, and the environment it
// inherits. socket is the control socket the desktop process will serve, and
// reaches the MCP child inside the handle.
func Launch(dir string, runner Runner, qroutonBin, socket string, editor EditorCommand, resume bool) (argv, env []string, err error) {
	if err := writeSupport(dir); err != nil {
		return nil, nil, err
	}
	handle := workbench.Handle{Socket: socket, SessionRoot: dir}
	return superviseArgv(qroutonBin, dir, runner, handle, editor, resume),
		workbench.WithEnv(os.Environ(), EditorEnvVar, editor.Marshal()), nil
}
