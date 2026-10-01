package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/lineartools"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
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
	Chime         bool                 `json:"chime"`
	UIScale       int                  `json:"uiScale"`
	UIScaleSteps  []int                `json:"uiScaleSteps"`
	Thoughts      ThoughtsView         `json:"thoughts"`
}

type ThoughtsView struct {
	Default string        `json:"default"`
	Derived string        `json:"derived"`
	Roots   []ThoughtsRow `json:"roots"`
}

type ThoughtsInput struct {
	Default string        `json:"default"`
	Roots   []ThoughtsRow `json:"roots"`
}

// ThoughtsRow carries a shared folder's orgs as the comma-separated text the panel edits.
type ThoughtsRow struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Orgs string `json:"orgs"`
}

type SettingsInput struct {
	Orgs          []string             `json:"orgs"`
	Root          string               `json:"root"`
	Editor        string               `json:"editor"`
	Launch        string               `json:"launch"`
	Linear        string               `json:"linear"`
	StickerLabels config.StickerLabels `json:"stickerLabels"`
	Chime         bool                 `json:"chime"`
	UIScale       int                  `json:"uiScale"`
	Thoughts      ThoughtsInput        `json:"thoughts"`
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
	reconfigure    func(*config.Config)
	linear         lineartools.Tools
}

func newSettings(cfg *config.Config, emit emitter, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, linearCommand, linearEnv []string, quit, wakeChrome func(),
	reconfigure func(*config.Config)) *Settings {
	return &Settings{
		reconfigure:    reconfigure,
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
		Editor:        editorCommand(cfg.Editor),
		Launch:        launch,
		Linear:        linear,
		LinearPath:    lineartools.ConfigPath,
		LinearError:   errorText(linearErr),
		StickerLabels: cfg.EffectiveStickerLabels(),
		Chime:         !cfg.Quiet,
		UIScale:       cfg.EffectiveUIScale(),
		UIScaleSteps:  config.UIScaleSteps(),
		Thoughts:      thoughtsView(cfg),
	}
}

func thoughtsView(cfg *config.Config) ThoughtsView {
	view := ThoughtsView{
		Default: cfg.Thoughts.Default,
		Derived: filepath.Join(cfg.Root, sessionpaths.ThoughtsDirName),
		Roots:   []ThoughtsRow{},
	}
	for _, r := range cfg.Thoughts.Roots {
		view.Roots = append(view.Roots, ThoughtsRow{ID: r.ID, Path: r.Path, Orgs: strings.Join(r.Orgs, thoughtsOrgJoin)})
	}
	return view
}

// The live config routes new sessions from these paths, so ~ is expanded here.
func thoughtsFromInput(in ThoughtsInput) config.Thoughts {
	out := config.Thoughts{Default: config.ExpandHome(strings.TrimSpace(in.Default))}
	for _, row := range in.Roots {
		out.Roots = append(out.Roots, config.ThoughtsRoot{
			ID:   strings.TrimSpace(row.ID),
			Path: config.ExpandHome(strings.TrimSpace(row.Path)),
			Orgs: dedupOrgs(strings.Split(row.Orgs, thoughtsOrgSplit)),
		})
	}
	return out
}

func editorCommand(argv []string) string {
	words := slices.Clone(argv)
	for i, word := range words {
		parsed, err := shlex.Split(word)
		if err != nil || len(parsed) != 1 || parsed[0] != word {
			words[i] = "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
		}
	}
	return strings.Join(words, " ")
}

func (s *Settings) Save(in SettingsInput) (SaveResult, error) {
	orgs, root, expandedRoot, err := validateOwnersAndRoot(in.Orgs, in.Root)
	if err != nil {
		return SaveResult{}, err
	}

	editor, err := shlex.Split(in.Editor)
	if err != nil {
		return SaveResult{}, fmt.Errorf(settingsRefusalFormat, settingsFieldEditor, err)
	}
	if len(editor) > 0 && s.validateEditor != nil {
		if err := s.validateEditor(editor); err != nil {
			return SaveResult{}, fmt.Errorf(settingsRefusalFormat, settingsFieldEditor, err)
		}
	}

	var launch map[string][]string
	if trimmed := strings.TrimSpace(in.Launch); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &launch); err != nil {
			return SaveResult{}, fmt.Errorf(settingsRefusalFormat, settingsFieldLaunch, err)
		}
		if s.validateLaunch != nil {
			if err := s.validateLaunch(launch); err != nil {
				return SaveResult{}, fmt.Errorf(settingsRefusalFormat, settingsFieldLaunch, err)
			}
		}
	}
	stickerLabels, err := validateStickerLabels(in.StickerLabels)
	if err != nil {
		return SaveResult{}, err
	}
	uiScale, err := validateUIScale(in.UIScale)
	if err != nil {
		return SaveResult{}, err
	}

	linear, err := lineartools.Validate(in.Linear)
	if err != nil {
		return SaveResult{}, fmt.Errorf(settingsWrappedFormat, settingsFieldLinear, err)
	}
	thoughts := thoughtsFromInput(in.Thoughts)
	candidate := s.cfg.Snapshot()
	candidate.Root, candidate.Thoughts = root, thoughts
	if err := config.CheckThoughts(candidate); err != nil {
		return SaveResult{}, fmt.Errorf(settingsWrappedFormat, settingsFieldThoughts, err)
	}
	result := SaveResult{}
	err = saveConfig(s.cfg, func(next *config.Config) {
		next.Orgs, next.Root, next.Editor, next.Launch = orgs, root, editor, launch
		next.StickerLabels = &stickerLabels
		next.Quiet = !in.Chime
		next.UIScale = storedUIScale(uiScale)
		next.Thoughts = thoughts
	}, func() error {
		if err := s.linear.Save(linear); err != nil {
			return fmt.Errorf(settingsWrappedFormat, settingsFieldLinear, err)
		}
		return nil
	}, func(current, live *config.Config) {
		if !reflect.DeepEqual(current.Thoughts, live.Thoughts) && s.reconfigure != nil {
			s.reconfigure(live)
		}
		changed := !slices.Equal(current.Orgs, orgs)
		labelsChanged := current.EffectiveStickerLabels() != stickerLabels
		result.RestartRequired = expandedRoot != filepath.Clean(current.Root)
		if changed {
			s.emit(orgsChangedEvent, orgs)
		}
		if labelsChanged && s.wakeChrome != nil {
			s.wakeChrome()
		}
		if current.EffectiveUIScale() != uiScale {
			s.emit(uiScaleEvent, uiScale)
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
		{name: settingsFieldStar, value: labels.Star},
		{name: settingsFieldBookmark, value: labels.Bookmark},
		{name: settingsFieldQuestion, value: labels.Question},
		{name: settingsFieldExclamation, value: labels.Exclamation},
	} {
		if field.value == "" {
			return config.StickerLabels{}, fmt.Errorf(settingsRefusalFormat, field.name, settingsEmpty)
		}
	}
	return labels, nil
}

// validateUIScale reads an unset scale as the default.
func validateUIScale(percent int) (int, error) {
	if percent == 0 {
		return config.UIScaleDefault, nil
	}
	if !config.ValidUIScale(percent) {
		return 0, fmt.Errorf(settingsWrappedFormat, settingsFieldUIScale, ErrUIScale)
	}
	return percent, nil
}

func storedUIScale(percent int) int {
	if percent == config.UIScaleDefault {
		return 0
	}
	return percent
}

// AdjustUIScale steps the scale in or out within its bounds, or resets it,
// saving and announcing only a change.
func (s *Settings) AdjustUIScale(action string) (int, error) {
	var scale int
	err := s.cfg.Transact(func(snapshot *config.Config) error {
		current := snapshot.EffectiveUIScale()
		switch action {
		case uiScaleActionIn:
			scale = min(current+config.UIScaleStep, config.UIScaleMax)
		case uiScaleActionOut:
			scale = max(current-config.UIScaleStep, config.UIScaleMin)
		case uiScaleActionReset:
			scale = config.UIScaleDefault
		default:
			return fmt.Errorf("%w: %q", ErrUIScaleAction, action)
		}
		if scale == current {
			return nil
		}
		onDisk, err := config.ReadFile()
		if err != nil {
			return err
		}
		onDisk.UIScale = storedUIScale(scale)
		if err := config.Save(onDisk); err != nil {
			return err
		}
		live := snapshot.Snapshot()
		live.UIScale = onDisk.UIScale
		s.cfg.Replace(live)
		s.emit(uiScaleEvent, scale)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return scale, nil
}

// Quit tears down every session supervisor and PTY before exiting.
func (s *Settings) Quit() { s.quit() }

// saveConfig persists Root but keeps the live boot value used by the rail scanner.
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

// validateOwnersAndRoot refuses empty owners before validateRoot can create a directory.
func validateOwnersAndRoot(owners []string, rawRoot string) (orgs []string, root, expanded string, err error) {
	orgs = dedupOrgs(owners)
	if len(orgs) == 0 {
		return nil, "", "", fmt.Errorf(settingsWrappedFormat, settingsFieldOrgs, ErrNoOwners)
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
		return "", "", fmt.Errorf(settingsRefusalFormat, settingsFieldRoot, settingsEmpty)
	}
	expanded = filepath.Clean(config.ExpandHome(stored))
	if err := os.MkdirAll(expanded, 0o755); err != nil {
		return "", "", fmt.Errorf(settingsRefusalFormat, settingsFieldRoot, err)
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
