package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/shlex"
	"github.com/kieranajp/qrouton/internal/config"
)

// SettingsView is config.Config on the wire: Editor and Launch collapse to the
// one text field each side of the panel edits.
type SettingsView struct {
	Orgs   []string `json:"orgs"`
	Root   string   `json:"root"`
	Editor string   `json:"editor"`
	Launch string   `json:"launch"`
}

// SettingsInput is what Save receives back: the same shape SettingsView hands
// out, filled in by the user.
type SettingsInput struct {
	Orgs   []string `json:"orgs"`
	Root   string   `json:"root"`
	Editor string   `json:"editor"`
	Launch string   `json:"launch"`
}

// SaveResult reports whether the process needs to end for a changed Root to
// take effect. Nothing else Settings saves needs a restart.
type SaveResult struct {
	RestartRequired bool `json:"restartRequired"`
}

// Settings is the panel's service: config.Config's four fields, and the
// process teardown its Root banner offers instead of a relaunch it cannot do.
type Settings struct {
	cfg            *config.Config
	validateEditor func([]string) error
	validateLaunch func(map[string][]string) error
	quit           func()
}

func newSettings(cfg *config.Config, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, quit func()) *Settings {
	return &Settings{cfg: cfg, validateEditor: validateEditor, validateLaunch: validateLaunch, quit: quit}
}

// Load reads the live config as the panel draws it.
func (s *Settings) Load() SettingsView {
	launch := ""
	if len(s.cfg.Launch) > 0 {
		if b, err := json.MarshalIndent(s.cfg.Launch, "", "  "); err == nil {
			launch = string(b)
		}
	}
	return SettingsView{
		Orgs:   append([]string(nil), s.cfg.Orgs...),
		Root:   s.cfg.Root,
		Editor: strings.Join(s.cfg.Editor, " "),
		Launch: launch,
	}
}

// Save validates Root, then Editor, then Launch, refusing on the first
// problem and writing nothing if any field fails. On success it writes the
// whole config to disk and updates every live field except Root — see Save's
// own Root handling below for why.
func (s *Settings) Save(in SettingsInput) (SaveResult, error) {
	root := strings.TrimSpace(in.Root)
	if root == "" {
		return SaveResult{}, fmt.Errorf("root: cannot be empty")
	}
	expandedRoot := filepath.Clean(config.ExpandHome(root))
	if err := os.MkdirAll(expandedRoot, 0o755); err != nil {
		return SaveResult{}, fmt.Errorf("root: %s", err)
	}

	editor, err := shlex.Split(in.Editor)
	if err != nil {
		return SaveResult{}, fmt.Errorf("editor: %s", err)
	}
	if len(editor) > 0 && s.validateEditor != nil {
		if err := s.validateEditor(editor); err != nil {
			return SaveResult{}, fmt.Errorf("editor: %s", err)
		}
	}

	var launch map[string][]string
	if trimmed := strings.TrimSpace(in.Launch); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &launch); err != nil {
			return SaveResult{}, fmt.Errorf("launch: %s", err)
		}
		if s.validateLaunch != nil {
			if err := s.validateLaunch(launch); err != nil {
				return SaveResult{}, fmt.Errorf("launch: %s", err)
			}
		}
	}

	orgs := dedupOrgs(in.Orgs)

	next := *s.cfg
	next.Orgs, next.Root, next.Editor, next.Launch = orgs, root, editor, launch
	if err := config.Save(&next); err != nil {
		return SaveResult{}, err
	}

	// Root is deliberately not written here: the rail's scanner, boot path and
	// cleanup all closed over the live root at process start, and mutating it
	// now would make a session created after this Save invisible to a rail
	// still scanning the old directory.
	s.cfg.Orgs, s.cfg.Editor, s.cfg.Launch = orgs, editor, launch

	return SaveResult{RestartRequired: expandedRoot != filepath.Clean(s.cfg.Root)}, nil
}

// Quit runs the same teardown closing the conversation window runs, not a bare
// renderer Quit: every open session's supervisor and PTY end cleanly before
// the process does.
func (s *Settings) Quit() { s.quit() }

// dedupOrgs trims and deduplicates the way config's own QROUTON_ORGS parsing
// does, over a slice rather than a CSV string.
func dedupOrgs(orgs []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, org := range orgs {
		org = strings.TrimSpace(org)
		if org != "" && !seen[org] {
			seen[org] = true
			out = append(out, org)
		}
	}
	return out
}
