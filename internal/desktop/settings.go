package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/kieranajp/qrouton/internal/config"
)

// SettingsView is config.Config plus Linear's coding-tools.json on the wire:
// each structured value collapses to the text field the panel edits.
type SettingsView struct {
	Orgs        []string `json:"orgs"`
	Root        string   `json:"root"`
	Editor      string   `json:"editor"`
	Launch      string   `json:"launch"`
	Linear      string   `json:"linear"`
	LinearPath  string   `json:"linearPath"`
	LinearError string   `json:"linearError,omitempty"`
}

// SettingsInput is what Save receives back: the same shape SettingsView hands
// out, filled in by the user.
type SettingsInput struct {
	Orgs   []string `json:"orgs"`
	Root   string   `json:"root"`
	Editor string   `json:"editor"`
	Launch string   `json:"launch"`
	Linear string   `json:"linear"`
}

// SaveResult reports whether the process needs to end for a changed Root to
// take effect. Nothing else Settings saves needs a restart.
type SaveResult struct {
	RestartRequired bool `json:"restartRequired"`
}

// Settings is the panel's service: config.Config, Linear's custom-script file,
// and the process teardown its Root banner offers instead of a relaunch.
type Settings struct {
	cfg            *config.Config
	emit           emitter
	validateEditor func([]string) error
	validateLaunch func(map[string][]string) error
	quit           func()
	linearFile     string
	linearCommand  []string
	linearEnv      []string
}

func newSettings(cfg *config.Config, emit emitter, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, linearCommand, linearEnv []string, quit func()) *Settings {
	return &Settings{
		cfg:            cfg,
		emit:           emit,
		validateEditor: validateEditor,
		validateLaunch: validateLaunch,
		quit:           quit,
		linearFile:     filepath.Clean(config.ExpandHome(linearConfigPath)),
		linearCommand:  append([]string(nil), linearCommand...),
		linearEnv:      append([]string(nil), linearEnv...),
	}
}

// Load reads the live config as the panel draws it.
func (s *Settings) Load() SettingsView {
	launch := ""
	if len(s.cfg.Launch) > 0 {
		if b, err := json.MarshalIndent(s.cfg.Launch, "", "  "); err == nil {
			launch = string(b)
		}
	}
	linear, linearErr := s.loadLinear()
	return SettingsView{
		Orgs:        append([]string(nil), s.cfg.Orgs...),
		Root:        s.cfg.Root,
		Editor:      strings.Join(s.cfg.Editor, " "),
		Launch:      launch,
		Linear:      linear,
		LinearPath:  linearConfigPath,
		LinearError: errorText(linearErr),
	}
}

// Save validates Orgs, Root, Editor, Launch, then Linear, refusing on the first
// problem and writing nothing if any field fails. On success it writes both
// files and updates every live config field except Root.
func (s *Settings) Save(in SettingsInput) (SaveResult, error) {
	orgs, root, expandedRoot, err := validateOwnersAndRoot(in.Orgs, in.Root)
	if err != nil {
		return SaveResult{}, err
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

	linear, err := validateLinear(in.Linear)
	if err != nil {
		return SaveResult{}, err
	}

	next := *s.cfg
	next.Orgs, next.Root, next.Editor, next.Launch = orgs, root, editor, launch
	if err := s.saveLinear(linear); err != nil {
		return SaveResult{}, err
	}
	if err := config.Save(&next); err != nil {
		return SaveResult{}, err
	}

	// Root stays off the live pointer: the rail's scanner and boot path closed
	// over it at process start, so a session created against a live-mutated
	// root would not appear in a rail still scanning the old one.
	changed := !slices.Equal(s.cfg.Orgs, orgs)
	s.cfg.Orgs, s.cfg.Editor, s.cfg.Launch = orgs, editor, launch
	if changed {
		s.emit(orgsChangedEvent, orgs)
	}

	return SaveResult{RestartRequired: expandedRoot != filepath.Clean(s.cfg.Root)}, nil
}

// Quit runs the same teardown closing the conversation window runs, not a bare
// renderer Quit: every open session's supervisor and PTY end cleanly before
// the process does.
func (s *Settings) Quit() { s.quit() }

func (s *Settings) loadLinear() (string, error) {
	b, err := os.ReadFile(s.linearFile)
	if err == nil {
		return string(b), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if len(s.linearCommand) == 0 {
		return "", ErrNoLinearCommand
	}
	args := append(append([]string(nil), s.linearCommand[1:]...), linearIssueTemplate)
	b, err = json.MarshalIndent(linearConfig{
		OpenIssue: linearCommand{
			Path: s.linearCommand[0],
			Args: args,
			Env:  append([]string(nil), s.linearEnv...),
		},
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func validateLinear(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &document); err != nil {
		return nil, fmt.Errorf("linear: %s", err)
	}
	if document == nil {
		return nil, fmt.Errorf("linear: %w", ErrLinearConfigObject)
	}
	return append([]byte(trimmed), '\n'), nil
}

func (s *Settings) saveLinear(body []byte) error {
	current, err := os.ReadFile(s.linearFile)
	if err == nil && bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(body)) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("linear: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.linearFile), linearConfigDirMode); err != nil {
		return fmt.Errorf("linear: %w", err)
	}
	if err := os.WriteFile(s.linearFile, body, linearConfigFileMode); err != nil {
		return fmt.Errorf("linear: %w", err)
	}
	return nil
}

type linearConfig struct {
	OpenIssue linearCommand `json:"openIssue"`
}

type linearCommand struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
	Env  []string `json:"env,omitempty"`
}

// validateOwnersAndRoot is the two answers both the settings panel and the
// first-run gate collect, checked in the order that matters: validateRoot
// creates the directory it is given, so an empty owner list refused behind it
// would leave that directory behind. Keeping the pair here is what stops a
// later edit to either caller from reordering them.
func validateOwnersAndRoot(owners []string, rawRoot string) (orgs []string, root, expanded string, err error) {
	orgs = dedupOrgs(owners)
	if len(orgs) == 0 {
		return nil, "", "", fmt.Errorf("orgs: %w", ErrNoOwners)
	}
	root, expanded, err = validateRoot(rawRoot)
	return orgs, root, expanded, err
}

// validateRoot creates a typed sessions root, answering the form to store — a
// leading ~ survives, so the config stays portable — and the cleaned absolute
// path to compare against the live one.
func validateRoot(raw string) (stored, expanded string, err error) {
	stored = strings.TrimSpace(raw)
	if stored == "" {
		return "", "", fmt.Errorf("root: cannot be empty")
	}
	expanded = filepath.Clean(config.ExpandHome(stored))
	if err := os.MkdirAll(expanded, 0o755); err != nil {
		return "", "", fmt.Errorf("root: %s", err)
	}
	return stored, expanded, nil
}

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
