package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/launch"
)

func TestAgentChildLaunchesOnTheNamedProfile(t *testing.T) {
	configHome, dir := t.TempDir(), filepath.Join(t.TempDir(), ".claude-work")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("QROUTON_ROOT", t.TempDir())
	if err := os.MkdirAll(filepath.Join(configHome, "qrouton"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"launch":{"claude":["/bin/echo"]},"profiles":{"claude":[{"id":"work","label":"Work","dir":"` + dir + `"}]}}`
	if err := os.WriteFile(filepath.Join(configHome, "qrouton", "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	runner, err := resolveRunner("claude", "work")
	if err != nil {
		t.Fatal(err)
	}
	if runner.Profile.ID != "work" || runner.Profile.Dir != dir {
		t.Fatalf("child runner profile = %+v, want work at %q", runner.Profile, dir)
	}

	_, err = resolveRunner("claude", "gone")
	if !errors.Is(err, launch.ErrProfileUnavailable) || !strings.Contains(err.Error(), "gone") {
		t.Fatalf("missing profile = %v, want %v naming it", err, launch.ErrProfileUnavailable)
	}
}
