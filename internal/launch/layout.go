package launch

import (
	_ "embed"
	"os"
	"runtime"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// notifyScript plays a short cross-platform attention sound when the workbench
// rings a session's chime.
//
//go:embed scripts/notify.sh
var notifyScript string

func writeSupport(dir string) error {
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sessionpaths.NotifyScript(dir), []byte(notifyScript), scriptMode)
}

func superviseArgv(qroutonBin, dir string, r Runner, handle workbench.Handle, editor EditorCommand, resume bool) []string {
	argv := []string{qroutonBin, AgentSubcommand, sessionRootFlag, dir, runnerFlag, r.ID,
		workbenchJSONFlag, handle.Marshal(), editorJSONFlag, editor.Marshal()}
	if resume {
		argv = append(argv, resumeFlag)
	}
	return argv
}

func ShellArgv(qroutonBin, dir string) []string {
	return []string{qroutonBin, ShellSubcommand, sessionRootFlag, dir}
}

// RevealArgv shows a directory in the file manager. On macOS, revealing rather
// than opening leaves Finder on the sessions root with the session selected.
func RevealArgv(dir string) []string { return revealArgv(runtime.GOOS, dir) }

func revealArgv(goos, dir string) []string {
	if goos == darwinOS {
		return []string{openCommand, openRevealFlag, dir}
	}
	return []string{xdgOpenCommand, dir}
}

// OpenTextArgv opens a file in the desktop's default text editor. xdg-open has
// no text-editor mode, so on Linux the file's own type picks the application.
func OpenTextArgv(path string) []string { return openTextArgv(runtime.GOOS, path) }

func openTextArgv(goos, path string) []string {
	if goos == darwinOS {
		return []string{openCommand, openTextFlag, path}
	}
	return []string{xdgOpenCommand, path}
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
