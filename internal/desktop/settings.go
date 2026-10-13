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
	"os/exec"
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

// SettingsView carries the editable config, and what Install would write for Linear.
type SettingsView struct {
	Orgs         []string         `json:"orgs"`
	Root         string           `json:"root"`
	Editor       string           `json:"editor"`
	Launch       string           `json:"launch"`
	Linear       LinearView       `json:"linear"`
	Stickers     []config.Sticker `json:"stickers"`
	Chime        bool             `json:"chime"`
	UIScale      int              `json:"uiScale"`
	UIScaleSteps []int            `json:"uiScaleSteps"`
	Thoughts     ThoughtsView     `json:"thoughts"`
	Pinned       SettingsPinned   `json:"pinned"`
	Runners      []RunnerLegend   `json:"runners"`
}

// LinearView is the coding-tools.json entry Install writes. Replaces means the
// file already holds a different one; Error says why neither could be read.
type LinearView struct {
	Path     string `json:"path"`
	Snippet  string `json:"snippet"`
	Replaces bool   `json:"replaces"`
	Error    string `json:"error,omitempty"`
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

// GitHubStatus is the credential the app would use and what GitHub said about
// it. Check is empty when there is no token to ask about.
type GitHubStatus struct {
	State    github.Credential `json:"state"`
	Check    github.TokenCheck `json:"check"`
	Login    string            `json:"login"`
	Variable string            `json:"variable"`
}

type SettingsInput struct {
	Orgs     []string         `json:"orgs"`
	Root     string           `json:"root"`
	Editor   string           `json:"editor"`
	Launch   string           `json:"launch"`
	Stickers []config.Sticker `json:"stickers"`
	Chime    bool             `json:"chime"`
	UIScale  int              `json:"uiScale"`
	Thoughts ThoughtsInput    `json:"thoughts"`
}

// SaveResult reports whether the process needs to end for a changed Root or
// changed thoughts folders to take effect. Nothing else Settings saves needs a
// restart.
type SaveResult struct {
	RestartRequired bool `json:"restartRequired"`
}

type Settings struct {
	cfg            *config.Config
	emit           emitter
	validateEditor func([]string) error
	validateLaunch func(map[string][]string) error
	relaunch       func() error
	quit           func()
	pinnedRoot     string
	openText       func(path string) []string
	wakeChrome     func()
	reconfigure    func(*config.Config) error
	runners        func() ([]assembly.Runner, error)
	checkToken     func(ctx context.Context, token string) (github.TokenCheck, string)
	linear         lineartools.Tools
}

func newSettings(cfg *config.Config, emit emitter, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, linearCommand, linearEnv []string, relaunch func() error,
	quit, wakeChrome func(), reconfigure func(*config.Config) error, runners func() ([]assembly.Runner, error),
	openText func(path string) []string) *Settings {
	return &Settings{
		pinnedRoot: pinnedFileRoot(),
		relaunch:   relaunch,
		openText:   openText,
		runners:    runners,
		checkToken: func(ctx context.Context, token string) (github.TokenCheck, string) {
			return github.CheckToken(ctx, &http.Client{Timeout: githubCheckTimeout}, token)
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
	return SettingsView{
		Orgs:         file.Orgs,
		Root:         root,
		Editor:       editorCommand(cfg.Editor),
		Launch:       launch,
		Linear:       s.linearView(),
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
	var orgs []string
	var root string
	var err error
	if config.OrgsOverridden() && len(dedupOrgs(in.Orgs)) == 0 {
		root, _, err = validateRoot(in.Root)
	} else {
		orgs, root, _, err = validateOwnersAndRoot(in.Orgs, in.Root)
	}
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

	thoughts := thoughtsFromInput(in.Thoughts)
	candidate := s.cfg.Snapshot()
	candidate.Root, candidate.Thoughts = root, thoughts
	if err := config.CheckThoughts(candidate); err != nil {
		return SaveResult{}, fmt.Errorf(settingsWrappedFormat, settingsFieldThoughts, err)
	}
	result := SaveResult{}
	err = saveConfig(s.cfg, s.pinnedRoot, func(next, live *config.Config) {
		if next.Root != "" || root != config.DefaultRoot() {
			next.Root = root
		}
		next.Orgs, next.Thoughts = orgs, thoughts
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
	ctx, cancel := context.WithTimeout(context.Background(), githubCheckTimeout)
	defer cancel()
	state, token := github.ResolveCredential(ctx)
	status := GitHubStatus{State: state, Variable: github.TokenEnvVar()}
	if token == "" || s.checkToken == nil {
		return status
	}
	status.Check, status.Login = s.checkToken(ctx, token)
	return status
}

// Relaunch starts a successor that reads config.json afresh, on no session, then
// tears this workbench down. Holding the config writer keeps a save from
// landing between the successor's read and this process's exit.
func (s *Settings) Relaunch() error {
	if s.relaunch == nil || s.quit == nil {
		return ErrNoRelaunch
	}
	if err := s.cfg.Transact(func(*config.Config) error { return s.relaunch() }); err != nil {
		return err
	}
	s.quit()
	return nil
}

// OpenConfig writes config.json first when it does not exist yet, so the editor
// has a file to open.
func (s *Settings) OpenConfig() error {
	var argv []string
	if s.openText != nil {
		argv = s.openText(config.Path())
	}
	if len(argv) == 0 {
		return ErrNoOpenTextCommand
	}
	if _, err := os.Stat(config.Path()); errors.Is(err, fs.ErrNotExist) {
		if err := s.cfg.Transact(func(*config.Config) error {
			onDisk, err := config.ReadFile()
			if err != nil {
				return err
			}
			return config.Save(onDisk)
		}); err != nil {
			return err
		}
	}
	if out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", ErrOpenConfig, strings.TrimSpace(string(out)))
	}
	return nil
}

// InstallLinear writes qrouton's openIssue entry at once; Cancel does not undo it.
func (s *Settings) InstallLinear() (LinearView, error) {
	if err := s.linear.Install(); err != nil {
		return s.linearView(), err
	}
	return s.linearView(), nil
}

func (s *Settings) linearView() LinearView {
	view := LinearView{Path: lineartools.ConfigPath}
	snippet, err := s.linear.Snippet()
	if err != nil {
		view.Error = err.Error()
		return view
	}
	view.Snippet = snippet
	replaces, err := s.linear.Replaces()
	if err != nil {
		view.Error = err.Error()
	}
	view.Replaces = replaces
	return view
}

// saveConfig writes next over config.json as written and publishes live over
// the running config. Live keeps the boot Root used by the rail scanner, and a
// Root that needs a restart holds back the thoughts too, since the live default
// still derives from the old one.
func saveConfig(cfg *config.Config, pinnedRoot string, mutate func(next, live *config.Config),
	reconfigure func(*config.Config) error, publish func(current, live *config.Config, restart bool)) error {
	return cfg.Transact(func(snapshot *config.Config) error {
		next, live, restart, err := stageConfig(snapshot, pinnedRoot, true, mutate)
		if err != nil {
			return err
		}
		if err := checkLiveThoughts(live); err != nil {
			return err
		}
		if err := commitLive(cfg, snapshot, next, live, reconfigure); err != nil {
			return err
		}
		if publish != nil {
			publish(snapshot, live, restart)
		}
		return nil
	})
}

func checkLiveThoughts(live *config.Config) error {
	if err := config.CheckThoughts(live); err != nil {
		return fmt.Errorf(settingsWrappedFormat, settingsFieldThoughts, err)
	}
	return nil
}

// stageConfig builds the file copy from config.json as written and the live copy
// from the running config. Until a restart, live keeps the root and thoughts it booted with.
func stageConfig(snapshot *config.Config, pinnedRoot string, holdThoughts bool, mutate func(next, live *config.Config)) (next, live *config.Config, restart bool, err error) {
	onDisk, err := config.ReadFile()
	if err != nil {
		return nil, nil, false, err
	}
	next, live = onDisk.Snapshot(), snapshot.Snapshot()
	mutate(next, live)
	live.Root = snapshot.Root
	restart = restartRequired(snapshot, pinnedRoot, next) || (holdThoughts && thoughtsRoutingChanged(snapshot, next))
	if restart {
		live.Thoughts = snapshot.Thoughts
	}
	return next, live, restart, nil
}

// commitLive reindexes before it writes or publishes anything, so routing and
// search never disagree. A failed write puts the old index back.
func commitLive(cfg, current, next, live *config.Config, reconfigure func(*config.Config) error) error {
	reindex := reconfigure != nil && !slices.Equal(thoughtsProfiles(current), thoughtsProfiles(live))
	if reindex {
		if err := reconfigure(live); err != nil {
			return fmt.Errorf(settingsWrappedFormat, settingsFieldThoughts, err)
		}
	}
	if err := config.Save(next); err != nil {
		if reindex {
			err = errors.Join(err, reconfigure(current))
		}
		return err
	}
	cfg.Replace(live)
	return nil
}

// restartRequired compares the saved root with the one this process runs on.
// While QROUTON_ROOT pins the live root, that is the file's root at boot.
func restartRequired(live *config.Config, pinnedRoot string, next *config.Config) bool {
	saved := resolvedRoot(next.Root)
	if pinnedRoot != "" {
		return saved != pinnedRoot
	}
	return saved != filepath.Clean(live.Root)
}

// thoughtsRoutingChanged compares the saved thoughts folders with the ones the
// running process routes by. Routing changes apply at the next launch.
func thoughtsRoutingChanged(live, next *config.Config) bool {
	saved := next.Snapshot()
	saved.Root = live.Root
	return saved.ThoughtsFingerprint() != live.ThoughtsFingerprint()
}

func pinnedFileRoot() string {
	if !config.RootOverridden() {
		return ""
	}
	file, err := config.ReadFile()
	if err != nil {
		return ""
	}
	return resolvedRoot(file.Root)
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
