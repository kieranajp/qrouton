package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// recordingClient is a client whose chime is recorded rather than rung.
type recordingClient struct {
	core.Client
	mu   sync.Mutex
	rung []string
}

func (c *recordingClient) Chime(sessionRoot string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rung = append(c.rung, sessionRoot)
}

func (c *recordingClient) chimes() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.rung...)
}

// The page names the path, so a reveal outside the session would hand the file
// manager any directory on the machine.
func TestRevealPathRevealsOnlyAPathInsideItsSession(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	var revealed []string
	reg.boot.reveal = func(path string) error {
		revealed = append(revealed, path)
		return nil
	}
	kraken := sessionDir(t, root, "kraken")
	inside := filepath.Join(kraken, "thoughts")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := reg.RevealPath("kraken", inside); err != nil {
		t.Fatal(err)
	}
	if len(revealed) != 1 || revealed[0] != inside {
		t.Fatalf("revealed %v, want [%q]", revealed, inside)
	}
	if err := reg.RevealPath("kraken", root); !errors.Is(err, ErrPathOutsideSession) {
		t.Fatalf("a path outside the session = %v, want ErrPathOutsideSession", err)
	}
	if err := reg.RevealPath("ghost", inside); err == nil {
		t.Fatal("a slug with no session was revealed anyway")
	}
	if len(revealed) != 1 {
		t.Fatalf("a refused path reached the file manager: %v", revealed)
	}
}

// A workbench with no clipboard command answers the page with the sentinel for
// the copy it was asked for, rather than an empty argv reaching exec.
func TestCopyAssetWithNoClipboardCommandReturnsTheSentinel(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)
	assets := filepath.Join(opts.SessionRoot, "thoughts", "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shot.png", "sim.mov"} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	slug := filepath.Base(opts.SessionRoot)
	if err := reg.CopyAsset(slug, filepath.Join(assets, "shot.png")); !errors.Is(err, ErrNoCopyImageCommand) {
		t.Fatalf("copying an image with nothing to run = %v, want ErrNoCopyImageCommand", err)
	}
	if err := reg.CopyAsset(slug, filepath.Join(assets, "sim.mov")); !errors.Is(err, ErrNoCopyFileCommand) {
		t.Fatalf("copying a video with nothing to run = %v, want ErrNoCopyFileCommand", err)
	}
}

// The chime is for a wait the user is not looking at: the session on screen in
// a focused window stays silent, and it rings once the window is unfocused or
// shows another session.
func TestTheWorkbenchChimesOnlyWhenTheWindowIsNotFocusedOnTheSession(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	opts.Config = &config.Config{}
	sessionDir(t, opts.Root, "kraken")
	reg, term, windows := testWorkbench(t, r, r.Emit)
	client := &recordingClient{Client: newLocalClient(r, opts.Launcher, nil)}
	startWorkbenchWith(t, r, client, term, windows, opts)
	<-r.opened
	octopus := shownSession(t, reg)
	handle := workbench.Handle{Socket: boot.socket(t, opts.SessionRoot), SessionRoot: opts.SessionRoot}
	attention := func() {
		t.Helper()
		if _, err := handle.WindowHost().Open(context.Background(), workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "qrouton", Content: "done", Attention: true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	r.inFront.Store(true)
	attention()
	if got := client.chimes(); len(got) != 0 {
		t.Fatalf("the watched session chimed %v", got)
	}
	r.inFront.Store(false)
	attention()
	if got, want := client.chimes(), opts.SessionRoot; len(got) != 1 || got[0] != want {
		t.Fatalf("chimed %v, want %s once", got, want)
	}

	_ = octopus.write([]byte("y"))
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	r.inFront.Store(true)
	attention()
	if got := client.chimes(); len(got) != 2 || got[1] != opts.SessionRoot {
		t.Fatalf("a focused window showing another session chimed %v, want %s a second time", got, opts.SessionRoot)
	}
}

// A window left titled for the previous session names the wrong work.
func TestShowingASessionRetitlesTheWindow(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	kraken := sessionDir(t, opts.Root, "kraken")
	reg, term, windows := testWorkbench(t, r, r.Emit)
	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)
	title := func() string {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.titles[mainWindowName]
	}

	waitFor(t, "the window titled for the first session", func() bool { return title() == windowTitle(opts.SessionRoot) })
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the window titled for the session switched to", func() bool { return title() == windowTitle(kraken) })
}

// Refusing after writing would leave a fresh config.json behind for a button
// that did nothing.
func TestOpenConfigWithNoOpenTextCommandWritesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	if err := s.OpenConfig(); !errors.Is(err, ErrNoOpenTextCommand) {
		t.Fatalf("OpenConfig without a command = %v, want ErrNoOpenTextCommand", err)
	}
	if _, err := os.Stat(config.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused OpenConfig left config.json behind: %v", err)
	}
}

// The local chime is the session's own notify script; ringing anything else
// is silence the user never hears about.
func TestTheLocalChimeRunsTheSessionsNotifyScript(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "rung")
	script := sessionpaths.NotifyScript(root)
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	newLocalClient(newFakeRenderer(), newStubBoot(), nil).Chime(root)
	waitFor(t, "the notify script to run", func() bool {
		_, err := os.Stat(marker)
		return err == nil
	})
}

// The page shows a refusal's text and the core tests it against ErrUnavailable,
// so each capability sentinel must keep its words and gain the match.
func TestEveryCapabilitySentinelKeepsItsTextAndIsUnavailable(t *testing.T) {
	for err, text := range map[error]string{
		ErrNoRevealCommand:    "workbench has no command to reveal a session's directory with",
		ErrNoOpenTextCommand:  "workbench has no command to open a file in a text editor",
		ErrNoCopyImageCommand: "workbench has no command to copy an image to the clipboard with",
		ErrNoCopyFileCommand:  "workbench has no command to copy a file to the clipboard with",
		ErrNoDirectoryPicker:  "workbench has no directory picker to choose a sessions root with",
	} {
		if err.Error() != text || !errors.Is(err, core.ErrUnavailable) {
			t.Errorf("%q: text %q, unavailable %v", text, err.Error(), errors.Is(err, core.ErrUnavailable))
		}
	}
	client := newLocalClient(newFakeRenderer(), newStubBoot(), nil)
	for name, call := range map[string]func() error{
		"reveal":     func() error { return client.Reveal("/") },
		"open text":  func() error { return client.OpenText("/") },
		"copy image": func() error { return client.CopyImage("/") },
		"copy file":  func() error { return client.CopyFile("/") },
		"choose":     func() error { _, err := client.ChooseDirectory(); return err },
	} {
		if err := call(); !errors.Is(err, core.ErrUnavailable) {
			t.Errorf("%s with nothing to run = %v, want ErrUnavailable", name, err)
		}
	}
	for _, capability := range []core.Capability{core.CapabilityReveal, core.CapabilityOpenText,
		core.CapabilityCopyImage, core.CapabilityCopyFile, core.CapabilityChooseDirectory} {
		if client.Supports(capability) {
			t.Errorf("a client with nothing to run claims %q", capability)
		}
	}
}

// The tool's own words are the only account of a failed open the page gets.
func TestAFailedOpenConfigCarriesWhatTheToolSaid(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	s.client = newLocalClient(nil, &stubBoot{openText: func(string) []string {
		return []string{"/bin/sh", "-c", "echo no application knows .json >&2; exit 4"}
	}}, nil)

	err := s.OpenConfig()
	if !errors.Is(err, ErrOpenConfig) || err.Error() != "config.json could not be opened: no application knows .json" {
		t.Fatalf("OpenConfig = %v", err)
	}
}
