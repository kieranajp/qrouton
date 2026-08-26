package session

import (
	"os"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func TestCreateCarriesAnInitialPromptPrivately(t *testing.T) {
	root := t.TempDir()
	dir, err := CreateWithInitialPrompt(&config.Config{Root: root}, "Linear task", "",
		"https://linear.app/issue/LIF-2841", "Fix the login regression.", "", ModeAssistant, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := os.ReadFile(sessionpaths.InitialPrompt(dir))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(prompt); got != "Fix the login regression." {
		t.Fatalf("initial prompt = %q", got)
	}
	info, err := os.Stat(sessionpaths.InitialPrompt(dir))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("initial prompt mode = %o, want 600", got)
	}
}

func TestCreateOmitsABlankInitialPrompt(t *testing.T) {
	root := t.TempDir()
	dir, err := CreateWithInitialPrompt(&config.Config{Root: root}, "Blank task", "", "", " \n ", "", ModeAssistant, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionpaths.InitialPrompt(dir)); !os.IsNotExist(err) {
		t.Fatalf("blank initial prompt was persisted: %v", err)
	}
}
