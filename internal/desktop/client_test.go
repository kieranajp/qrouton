package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// A workbench with no clipboard command answers the page with the sentinel for
// the copy it was asked for, rather than an empty argv reaching exec.
func TestCopyAssetWithNoClipboardCommandReturnsTheSentinel(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions
	assets := filepath.Join(opts.SessionRoot, "thoughts", "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shot.png", "sim.mov"} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	startWorkbench(t, r, kernel, opts)
	<-r.opened
	shownSession(t, reg)

	slug := filepath.Base(opts.SessionRoot)
	if err := reg.CopyAsset(slug, filepath.Join(assets, "shot.png")); !errors.Is(err, core.ErrNoCopyImageCommand) {
		t.Fatalf("copying an image with nothing to run = %v, want ErrNoCopyImageCommand", err)
	}
	if err := reg.CopyAsset(slug, filepath.Join(assets, "sim.mov")); !errors.Is(err, core.ErrNoCopyFileCommand) {
		t.Fatalf("copying a video with nothing to run = %v, want ErrNoCopyFileCommand", err)
	}
}

// A window left titled for the previous session names the wrong work.
func TestShowingASessionRetitlesTheWindow(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	kraken := sessionDir(t, opts.Root, "kraken")
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions
	startWorkbench(t, r, kernel, opts)
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
	s := &Settings{settings: testWorkbench(t, newFakeRenderer(), &Options{Config: &config.Config{}, Launcher: newStubBoot()}).Settings}
	if err := s.OpenConfig(); !errors.Is(err, core.ErrNoOpenTextCommand) {
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
		core.ErrNoRevealCommand:    "workbench has no command to reveal a session's directory with",
		core.ErrNoOpenTextCommand:  "workbench has no command to open a file in a text editor",
		core.ErrNoCopyImageCommand: "workbench has no command to copy an image to the clipboard with",
		core.ErrNoCopyFileCommand:  "workbench has no command to copy a file to the clipboard with",
		core.ErrNoDirectoryPicker:  "workbench has no directory picker to choose a sessions root with",
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
	boot := newStubBoot()
	boot.openText = func(string) []string {
		return []string{"/bin/sh", "-c", "echo no application knows .json >&2; exit 4"}
	}
	s := &Settings{settings: testWorkbench(t, newFakeRenderer(), &Options{Config: &config.Config{}, Launcher: boot}).Settings}

	err := s.OpenConfig()
	if !errors.Is(err, core.ErrOpenConfig) || err.Error() != "config.json could not be opened: no application knows .json" {
		t.Fatalf("OpenConfig = %v", err)
	}
}
