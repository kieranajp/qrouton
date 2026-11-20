package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type headlessClient struct{}

func (headlessClient) Emit(string, any)                     {}
func (headlessClient) Focused() bool                        { return false }
func (headlessClient) Raise()                               {}
func (headlessClient) SessionShown(string)                  {}
func (headlessClient) Supports(core.Capability) bool        { return false }
func (headlessClient) Reveal(string) error                  { return core.ErrUnavailable }
func (headlessClient) OpenText(string) error                { return core.ErrUnavailable }
func (headlessClient) CopyImage(string) error               { return core.ErrUnavailable }
func (headlessClient) CopyFile(string) error                { return core.ErrUnavailable }
func (headlessClient) ChooseDirectory(bool) (string, error) { return "", core.ErrUnavailable }
func (headlessClient) Chime(string)                         {}

// recordingRunner writes a runner that saves its environment once per launch.
// The first launch waits for the supervisor to stop it; later ones exit.
func recordingRunner(t *testing.T, records string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "runner")
	body := fmt.Sprintf(`#!/bin/sh
n=$(ls %[1]q | wc -l | tr -d ' ')
env > %[1]q/launch-$n.tmp && mv %[1]q/launch-$n.tmp %[1]q/launch-$n.env
[ "$n" -eq 0 ] && exec sleep 60
exit 0
`, records)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func launchedEnv(t *testing.T, records string, n int) []string {
	t.Helper()
	path := filepath.Join(records, fmt.Sprintf("launch-%d.env", n))
	deadline := time.Now().Add(30 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.Split(string(data), "\n")
		}
		if time.Now().After(deadline) {
			t.Fatalf("the runner never made launch %d", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Breaks if a session's profile is lost anywhere between its manifest and the
// runner's environment: the boot reading the manifest, the supervisor's argv,
// the child re-resolving the profile, or a SIGUSR1 relaunch.
func TestTheManifestsProfileReachesTheRunnerThroughBootAndRestart(t *testing.T) {
	for _, tc := range []struct{ runner, envVar string }{
		{"claude", "CLAUDE_CONFIG_DIR"},
		{"codex", "CODEX_HOME"},
	} {
		t.Run(tc.runner, func(t *testing.T) {
			configHome, root, records := t.TempDir(), t.TempDir(), t.TempDir()
			profileDir := filepath.Join(t.TempDir(), "work")
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", configHome)
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			t.Setenv("QROUTON_ROOT", root)
			t.Setenv(tc.envVar, "")
			if err := os.Unsetenv(tc.envVar); err != nil {
				t.Fatal(err)
			}
			t.Setenv(runMainEnvVar, "1")

			if err := config.Save(&config.Config{
				Root:     root,
				Launch:   map[string][]string{tc.runner: {recordingRunner(t, records)}},
				Profiles: map[string][]config.RunnerProfile{tc.runner: {{ID: "work", Label: "Work", Dir: profileDir}}},
			}); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "octopus")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := session.WriteManifest(dir, session.Manifest{Slug: "octopus", Mode: session.ModeAssistant,
				Runner: tc.runner, Profile: "work"}); err != nil {
				t.Fatal(err)
			}
			bin, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}

			ports := workbenchPorts{cfg: cfg, bin: bin, spec: launch.WorkbenchSpec{Runner: "opencode"}}
			socket, err := workbench.NewSocketPath()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(socket) })
			kernel := core.New(core.Options{Root: root, Config: cfg, Launcher: ports}, headlessClient{})
			t.Cleanup(kernel.Shutdown)
			if err := kernel.Start(socket); err != nil {
				t.Fatal(err)
			}
			if err := kernel.OpenSession(dir, true); err != nil {
				t.Fatal(err)
			}
			var terminal string
			deadline := time.Now().Add(30 * time.Second)
			for terminal == "" {
				terminal = kernel.Chrome.Snapshot().Terminal
				if time.Now().After(deadline) {
					t.Fatal("the session's terminal never reached the chrome")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := kernel.Term.Start(terminal, 80, 24); err != nil {
				t.Fatal(err)
			}

			want := tc.envVar + "=" + profileDir
			if env := launchedEnv(t, records, 0); !slices.Contains(env, want) {
				t.Fatalf("the first launch ran without %s", want)
			}
			launch.SignalSupervisor(dir)
			if env := launchedEnv(t, records, 1); !slices.Contains(env, want) {
				t.Fatalf("the relaunch after SIGUSR1 ran without %s", want)
			}
		})
	}
}
