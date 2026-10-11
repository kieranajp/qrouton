package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/lineartools"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// SettingsView carries the editable config and Linear document on the wire.
type SettingsView struct {
	Orgs         []string         `json:"orgs"`
	Root         string           `json:"root"`
	Editor       string           `json:"editor"`
	Launch       string           `json:"launch"`
	Linear       string           `json:"linear"`
	LinearPath   string           `json:"linearPath"`
	LinearError  string           `json:"linearError,omitempty"`
	Stickers     []config.Sticker `json:"stickers"`
	Chime        bool             `json:"chime"`
	UIScale      int              `json:"uiScale"`
	UIScaleSteps []int            `json:"uiScaleSteps"`
	Thoughts     ThoughtsView     `json:"thoughts"`
	Pinned       SettingsPinned   `json:"pinned"`
	Runners      []RunnerLegend   `json:"runners"`
}

// RunnerLegend names a runner id the launch overrides may key on.
type RunnerLegend struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SettingsPinned names the fields an environment variable overrides for this run.
type SettingsPinned struct {
	Root bool `json:"root"`
	Orgs bool `json:"orgs"`
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

// ThoughtsRow carries a shared folder's orgs as the comma-separated text the
// panel edits. Missing reports on Load only, and Save ignores it.
type ThoughtsRow struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Orgs    string `json:"orgs"`
	Missing bool   `json:"missing,omitempty"`
}

// GitHubStatus is the credential the app would use. Login names the account
// when one is signed in and GitHub answered; Variable names the env fallback.
type GitHubStatus struct {
	State    github.Credential `json:"state"`
	Login    string            `json:"login"`
	Variable string            `json:"variable"`
}

type SettingsInput struct {
	Orgs     []string         `json:"orgs"`
	Root     string           `json:"root"`
	Editor   string           `json:"editor"`
	Launch   string           `json:"launch"`
	Linear   string           `json:"linear"`
	Stickers []config.Sticker `json:"stickers"`
	Chime    bool             `json:"chime"`
	UIScale  int              `json:"uiScale"`
	Thoughts ThoughtsInput    `json:"thoughts"`
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
	reconfigure    func(*config.Config) error
	runners        func() ([]assembly.Runner, error)
	login          func(ctx context.Context, token string) (string, error)
	linear         lineartools.Tools
}

func newSettings(cfg *config.Config, emit emitter, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, linearCommand, linearEnv []string, quit, wakeChrome func(),
	reconfigure func(*config.Config) error, runners func() ([]assembly.Runner, error)) *Settings {
	return &Settings{
		runners: runners,
		login: func(ctx context.Context, token string) (string, error) {
			return github.AuthenticatedLogin(ctx, &http.Client{Timeout: loginTimeout}, token)
		},
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

// Load shows root, orgs and thoughts as config.json holds them, so a Save
// never writes back an environment override or an expanded ~.
func (s *Settings) Load() SettingsView {
	cfg := s.cfg.Snapshot()
	file, err := config.ReadFile()
	if err != nil {
		file = cfg
	}
	root := file.Root
	if strings.TrimSpace(root) == "" {
		root = config.DefaultRoot()
	}
	launch := ""
	if len(cfg.Launch) > 0 {
		if b, err := json.MarshalIndent(cfg.Launch, "", "  "); err == nil {
			launch = string(b)
		}
	}
	linear, linearErr := s.linear.Load()
	return SettingsView{
		Orgs:         file.Orgs,
		Root:         root,
		Editor:       editorCommand(cfg.Editor),
		Launch:       launch,
		Linear:       linear,
		LinearPath:   lineartools.ConfigPath,
		LinearError:  errorText(linearErr),
		Stickers:     cfg.EffectiveStickers(),
		Chime:        !cfg.Quiet,
		UIScale:      cfg.EffectiveUIScale(),
		UIScaleSteps: config.UIScaleSteps(),
		Thoughts:     thoughtsView(file.Thoughts, root),
		Pinned:       SettingsPinned{Root: config.RootOverridden(), Orgs: config.OrgsOverridden()},
		Runners:      s.runnerLegend(),
	}
}

func (s *Settings) runnerLegend() []RunnerLegend {
	legend := []RunnerLegend{}
	if s.runners == nil {
		return legend
	}
	runners, err := s.runners()
	if err != nil {
		return legend
	}
	for _, r := range runners {
		legend = append(legend, RunnerLegend{ID: r.ID, Label: r.Label})
	}
	return legend
}

func thoughtsView(thoughts config.Thoughts, root string) ThoughtsView {
	view := ThoughtsView{
		Default: thoughts.Default,
		Derived: filepath.Join(root, sessionpaths.ThoughtsDirName),
		Roots:   []ThoughtsRow{},
	}
	for _, r := range thoughts.Roots {
		_, err := os.Stat(config.ExpandHome(r.Path))
		view.Roots = append(view.Roots, ThoughtsRow{
			ID: r.ID, Path: r.Path, Orgs: strings.Join(r.Orgs, thoughtsOrgJoin), Missing: errors.Is(err, fs.ErrNotExist),
		})
	}
	return view
}

func thoughtsFromInput(in ThoughtsInput) config.Thoughts {
	out := config.Thoughts{Default: strings.TrimSpace(in.Default)}
	for _, row := range in.Roots {
		out.Roots = append(out.Roots, config.ThoughtsRoot{
			ID:   strings.TrimSpace(row.ID),
			Path: strings.TrimSpace(row.Path),
			Orgs: dedupOrgs(strings.Split(row.Orgs, thoughtsOrgSplit)),
		})
	}
	return out
}

// expandThoughts resolves ~ for the live config, which routes new sessions by these paths.
func expandThoughts(in config.Thoughts) config.Thoughts {
	out := config.Thoughts{Default: config.ExpandHome(in.Default)}
	for _, r := range in.Roots {
		out.Roots = append(out.Roots, config.ThoughtsRoot{ID: r.ID, Path: config.ExpandHome(r.Path), Orgs: slices.Clone(r.Orgs)})
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
	orgs, root, _, err := validateOwnersAndRoot(in.Orgs, in.Root)
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
	stickers, err := validateStickers(in.Stickers)
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
	err = saveConfig(s.cfg, func(next, live *config.Config) {
		next.Orgs, next.Root, next.Thoughts = orgs, root, thoughts
		if !config.OrgsOverridden() {
			live.Orgs = orgs
		}
		live.Thoughts = expandThoughts(thoughts)
		for _, c := range []*config.Config{next, live} {
			c.Editor, c.Launch = editor, launch
			c.Stickers, c.StickerLabels = &stickers, nil
			c.Quiet = !in.Chime
			c.UIScale = storedUIScale(uiScale)
		}
	}, func() error {
		if err := s.linear.Save(linear); err != nil {
			return fmt.Errorf(settingsWrappedFormat, settingsFieldLinear, err)
		}
		return nil
	}, s.reconfigure, func(current, live *config.Config, restart bool) {
		changed := !slices.Equal(current.Orgs, live.Orgs)
		labelsChanged := !slices.Equal(current.EffectiveStickers(), stickers)
		result.RestartRequired = restart
		if changed {
			s.emit(orgsChangedEvent, live.Orgs)
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

// validateStickers trims labels and gives each new sticker an id. Ids are
// random so a removed sticker's id never comes back onto a session.
func validateStickers(in []config.Sticker) ([]config.Sticker, error) {
	out := make([]config.Sticker, 0, len(in))
	seen := map[[2]string]bool{}
	for i, s := range in {
		field := fmt.Sprintf(stickerFieldFormat, settingsFieldStickers, i)
		s.Label = strings.TrimSpace(s.Label)
		if s.Label == "" {
			return nil, fmt.Errorf(settingsRefusalFormat, field, settingsEmpty)
		}
		if err := config.CheckSticker(s); err != nil {
			return nil, fmt.Errorf(settingsWrappedFormat, field, err)
		}
		pair := [2]string{s.Colour, s.Symbol}
		if seen[pair] {
			return nil, fmt.Errorf(settingsWrappedFormat, field, ErrStickerDuplicate)
		}
		seen[pair] = true
		if s.ID == "" {
			id := make([]byte, stickerIDBytes)
			if _, err := rand.Read(id); err != nil {
				return nil, err
			}
			s.ID = hex.EncodeToString(id)
		}
		out = append(out, s)
	}
	return out, nil
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

// GitHubStatus checks the credential each time it is asked, so Check again sees
// a gh install or sign-in that happened since.
func (s *Settings) GitHubStatus() GitHubStatus {
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()
	status := GitHubStatus{State: github.CredentialSource(ctx), Variable: github.TokenEnvVar()}
	if status.State != github.CredentialCLI && status.State != github.CredentialEnv {
		return status
	}
	token, err := github.TokenContext(ctx)
	if err != nil || s.login == nil {
		return status
	}
	status.Login, _ = s.login(ctx, token)
	return status
}

// Quit tears down every session supervisor and PTY before exiting.
func (s *Settings) Quit() { s.quit() }

// saveConfig writes next over config.json as written and publishes live over
// the running config. Live keeps the boot Root used by the rail scanner, and a
// Root that needs a restart holds back the thoughts too, since the live default
// still derives from the old one.
func saveConfig(cfg *config.Config, mutate func(next, live *config.Config), persist func() error,
	reconfigure func(*config.Config) error, publish func(current, live *config.Config, restart bool)) error {
	return cfg.Transact(func(snapshot *config.Config) error {
		onDisk, err := config.ReadFile()
		if err != nil {
			return err
		}
		next, live := onDisk.Snapshot(), snapshot.Snapshot()
		mutate(next, live)
		live.Root = snapshot.Root
		restart := restartRequired(snapshot, onDisk, next)
		if restart {
			live.Thoughts = snapshot.Thoughts
		}
		if err := config.CheckThoughts(live); err != nil {
			return fmt.Errorf(settingsWrappedFormat, settingsFieldThoughts, err)
		}
		if err := commitLive(cfg, snapshot, next, live, reconfigure, persist); err != nil {
			return err
		}
		if publish != nil {
			publish(snapshot, live, restart)
		}
		return nil
	})
}

// commitLive reindexes before it writes or publishes anything, so routing and
// search never disagree. A failed write puts the old index back.
func commitLive(cfg, current, next, live *config.Config, reconfigure func(*config.Config) error, persist func() error) error {
	reindex := reconfigure != nil && !slices.Equal(thoughtsProfiles(current), thoughtsProfiles(live))
	if reindex {
		if err := reconfigure(live); err != nil {
			return fmt.Errorf(settingsWrappedFormat, settingsFieldThoughts, err)
		}
	}
	var err error
	if persist != nil {
		err = persist()
	}
	if err == nil {
		err = config.Save(next)
	}
	if err != nil {
		if reindex {
			err = errors.Join(err, reconfigure(current))
		}
		return err
	}
	cfg.Replace(live)
	return nil
}

// restartRequired compares the saved root with the one a restart would replace:
// the live root, or config.json's previous root while QROUTON_ROOT pins the live one.
func restartRequired(live, onDisk, next *config.Config) bool {
	saved := resolvedRoot(next.Root)
	if config.RootOverridden() {
		return saved != resolvedRoot(onDisk.Root)
	}
	return saved != filepath.Clean(live.Root)
}

func resolvedRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		root = config.DefaultRoot()
	}
	return filepath.Clean(config.ExpandHome(strings.TrimSpace(root)))
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
