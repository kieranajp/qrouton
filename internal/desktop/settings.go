package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/lineartools"
)

// SettingsView carries the editable config and Linear document on the wire.
type SettingsView struct {
	Orgs          []string             `json:"orgs"`
	Root          string               `json:"root"`
	Editor        string               `json:"editor"`
	Launch        string               `json:"launch"`
	Linear        string               `json:"linear"`
	LinearPath    string               `json:"linearPath"`
	LinearError   string               `json:"linearError,omitempty"`
	StickerLabels config.StickerLabels `json:"stickerLabels"`
}

type SettingsInput struct {
	Orgs          []string             `json:"orgs"`
	Root          string               `json:"root"`
	Editor        string               `json:"editor"`
	Launch        string               `json:"launch"`
	Linear        string               `json:"linear"`
	StickerLabels config.StickerLabels `json:"stickerLabels"`
}

// SaveResult reports whether the process needs to end for a changed Root to
// take effect. Nothing else Settings saves needs a restart.
type SaveResult struct {
	RestartRequired bool `json:"restartRequired"`
}

type Settings struct {
	cfg            *config.Config
	emit           emitter
	validateEditor func([]string) error
	validateLaunch func(map[string][]string) error
	quit           func()
	wakeChrome     func()
	linear         lineartools.Tools
}

func newSettings(cfg *config.Config, emit emitter, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, linearCommand, linearEnv []string, quit, wakeChrome func()) *Settings {
	return &Settings{
		cfg:            cfg,
		emit:           emit,
		validateEditor: validateEditor,
		validateLaunch: validateLaunch,
		quit:           quit,
		wakeChrome:     wakeChrome,
		linear:         lineartools.New(linearCommand, linearEnv),
	}
}

func (s *Settings) Load() SettingsView {
	cfg := s.cfg.Snapshot()
	launch := ""
	if len(cfg.Launch) > 0 {
		if b, err := json.MarshalIndent(cfg.Launch, "", "  "); err == nil {
			launch = string(b)
		}
	}
	linear, linearErr := s.linear.Load()
	return SettingsView{
		Orgs:          cfg.Orgs,
		Root:          cfg.Root,
		Editor:        strings.Join(cfg.Editor, " "),
		Launch:        launch,
		Linear:        linear,
		LinearPath:    lineartools.ConfigPath,
		LinearError:   errorText(linearErr),
		StickerLabels: cfg.EffectiveStickerLabels(),
	}
}

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
	stickerLabels, err := validateStickerLabels(in.StickerLabels)
	if err != nil {
		return SaveResult{}, err
	}

	linear, err := lineartools.Validate(in.Linear)
	if err != nil {
		return SaveResult{}, fmt.Errorf("linear: %w", err)
	}
	result := SaveResult{}
	err = saveConfig(s.cfg, func(next *config.Config) {
		next.Orgs, next.Root, next.Editor, next.Launch = orgs, root, editor, launch
		next.StickerLabels = &stickerLabels
	}, func() error {
		if err := s.linear.Save(linear); err != nil {
			return fmt.Errorf("linear: %w", err)
		}
		return nil
	}, func(current, _ *config.Config) {
		changed := !slices.Equal(current.Orgs, orgs)
		labelsChanged := current.EffectiveStickerLabels() != stickerLabels
		result.RestartRequired = expandedRoot != filepath.Clean(current.Root)
		if changed {
			s.emit(orgsChangedEvent, orgs)
		}
		if labelsChanged && s.wakeChrome != nil {
			s.wakeChrome()
		}
	})
	if err != nil {
		return SaveResult{}, err
	}
	return result, nil
}

func validateStickerLabels(labels config.StickerLabels) (config.StickerLabels, error) {
	labels.Star = strings.TrimSpace(labels.Star)
	labels.Bookmark = strings.TrimSpace(labels.Bookmark)
	labels.Question = strings.TrimSpace(labels.Question)
	labels.Exclamation = strings.TrimSpace(labels.Exclamation)
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "star", value: labels.Star},
		{name: "bookmark", value: labels.Bookmark},
		{name: "question", value: labels.Question},
		{name: "exclamation", value: labels.Exclamation},
	} {
		if field.value == "" {
			return config.StickerLabels{}, fmt.Errorf("%s: cannot be empty", field.name)
		}
	}
	return labels, nil
}

// Quit runs the same teardown closing the conversation window runs, not a bare
// renderer Quit: every open session's supervisor and PTY end cleanly before
// the process does.
func (s *Settings) Quit() { s.quit() }

// saveConfig writes cfg with mutate applied, and answers the mirror of that
// write onto the live pointer. Root is written but never applied: the rail's
// scanner and boot path closed over the boot value, so a session created
// against a live-mutated root would not appear in a rail still scanning the old
// one.
func saveConfig(cfg *config.Config, mutate func(*config.Config), persist func() error,
	publish func(current, live *config.Config)) error {
	return cfg.Transact(func(snapshot *config.Config) error {
		next := snapshot.Snapshot()
		mutate(next)
		if persist != nil {
			if err := persist(); err != nil {
				return err
			}
		}
		if err := config.Save(next); err != nil {
			return err
		}
		live := next.Snapshot()
		live.Root = snapshot.Root
		cfg.Replace(live)
		if publish != nil {
			publish(snapshot, live)
		}
		return nil
	})
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
