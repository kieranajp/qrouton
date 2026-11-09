package desktop

import (
	"os/exec"
	"strings"

	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// localClient is the machine the workbench runs on: the conversation window
// and the desktop's own tools.
type localClient struct {
	r        renderer
	launcher Launcher
	choose   func() (string, error)
}

var _ core.Client = (*localClient)(nil)

func newLocalClient(r renderer, launcher Launcher, choose func() (string, error)) *localClient {
	return &localClient{r: r, launcher: launcher, choose: choose}
}

func (c *localClient) Emit(event string, payload any) { c.r.Emit(event, payload) }

func (c *localClient) Focused() bool { return c.r.Focused(mainWindowName) }

func (c *localClient) Raise() { c.r.Focus(mainWindowName) }

func (c *localClient) SessionShown(root string) { c.r.Retitle(mainWindowName, windowTitle(root)) }

// Supports asks the launcher with no path, since an argv builder that can do
// the thing builds one whatever the path.
func (c *localClient) Supports(capability core.Capability) bool {
	switch capability {
	case core.CapabilityReveal:
		return len(c.launcher.Reveal("")) > 0
	case core.CapabilityOpenText:
		return len(c.launcher.OpenText("")) > 0
	case core.CapabilityCopyImage:
		return len(c.launcher.CopyImage("")) > 0
	case core.CapabilityCopyFile:
		return len(c.launcher.CopyFile("")) > 0
	case core.CapabilityChooseDirectory:
		return c.choose != nil
	}
	return false
}

// Reveal shows a directory in the file manager. open(1) hands the request to
// the desktop and exits, so waiting on it costs nothing and turns a bad path
// into an error the page can show.
func (c *localClient) Reveal(path string) error {
	return runLocal(c.launcher.Reveal(path), ErrNoRevealCommand)
}

// OpenText answers a failure with what the tool printed, which is the only
// account of it the page gets.
func (c *localClient) OpenText(path string) error {
	argv := c.launcher.OpenText(path)
	if len(argv) == 0 {
		return ErrNoOpenTextCommand
	}
	if out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil {
		return &toolFailed{output: strings.TrimSpace(string(out)), err: err}
	}
	return nil
}

// CopyImage puts a picture on the clipboard. The conversion runs while the
// command does, so waiting on it is what says the clipboard now holds the
// image.
func (c *localClient) CopyImage(path string) error {
	return runLocal(c.launcher.CopyImage(path), ErrNoCopyImageCommand)
}

func (c *localClient) CopyFile(path string) error {
	return runLocal(c.launcher.CopyFile(path), ErrNoCopyFileCommand)
}

// ChooseDirectory answers "" on a cancel.
func (c *localClient) ChooseDirectory() (string, error) {
	if c.choose == nil {
		return "", ErrNoDirectoryPicker
	}
	return c.choose()
}

func (c *localClient) Chime(sessionRoot string) {
	cmd := exec.Command(sessionpaths.NotifyScript(sessionRoot))
	if err := cmd.Start(); err != nil {
		return
	}
	go func() { _ = cmd.Wait() }()
}

func runLocal(argv []string, unavailable error) error {
	if len(argv) == 0 {
		return unavailable
	}
	return exec.Command(argv[0], argv[1:]...).Run()
}

type toolFailed struct {
	output string
	err    error
}

func (e *toolFailed) Error() string { return e.output }

func (e *toolFailed) Unwrap() error { return e.err }
