package session

import (
	"os"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func TestCreateCarriesAnInitialPromptPrivately(t *testing.T) {
	root := t.TempDir()
	dir, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Linear task", Ticket: "https://linear.app/issue/LIF-2841",
		InitialPrompt: "Fix the login regression.", Mode: ModeAssistant,
	}, nil)
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
	dir, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Blank task", InitialPrompt: " \n ", Mode: ModeAssistant,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionpaths.InitialPrompt(dir)); !os.IsNotExist(err) {
		t.Fatalf("blank initial prompt was persisted: %v", err)
	}
}
