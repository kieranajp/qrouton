package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/lineartools"
	"github.com/kieranajp/qrouton/internal/vault"
)

func testSettings(t *testing.T, cfg *config.Config, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, quit func()) *Settings {
	t.Helper()
	s := newSettings(cfg, func(string, any) {}, validateEditor, validateLaunch, []string{
		"/Applications/qrouton.app/Contents/MacOS/qrouton",
		"--linear-issue",
	}, []string{"LINEAR_PROMPT"}, nil, quit, nil, nil, nil, nil)
	s.linear.File = filepath.Join(t.TempDir(), "coding-tools.json")
	return s
}

// writeConfigFile gives the test a config.json holding cfg, as a booted app has.
func writeConfigFile(t *testing.T, cfg *config.Config) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(config.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireBlocked(t *testing.T, done <-chan error, what string) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("%s completed before the transaction boundary opened: %v", what, err)
	case <-time.After(250 * time.Millisecond):
	}
}

func TestSettingsLoadRoundTripsEditorAndLaunch(t *testing.T) {
	cfg := &config.Config{
		Orgs:   []string{"acme", "second-org"},
		Root:   "/sessions",
		Editor: []string{"code", "--wait", "{}"},
		Launch: map[string][]string{"claude": {"claude", "--verbose"}},
	}
	writeConfigFile(t, cfg)
	s := testSettings(t, cfg, nil, nil, nil)

	got := s.Load()
	if !reflect.DeepEqual(got.Orgs, cfg.Orgs) {
		t.Fatalf("orgs = %#v, want %#v", got.Orgs, cfg.Orgs)
	}
	if got.Root != cfg.Root {
		t.Fatalf("root = %q, want %q", got.Root, cfg.Root)
	}
	if got.Editor != "code --wait {}" {
		t.Fatalf("editor = %q", got.Editor)
	}
	var launch map[string][]string
	if err := json.Unmarshal([]byte(got.Launch), &launch); err != nil {
		t.Fatalf("launch is not valid JSON: %v (%q)", err, got.Launch)
	}
	if !reflect.DeepEqual(launch, cfg.Launch) {
		t.Fatalf("launch = %#v, want %#v", launch, cfg.Launch)
	}
}

func TestSettingsPreserveEditorArgumentsWhenSaving(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := []string{"/Applications/Visual Studio Code.app/bin/code", "--wait", "{}", "a'b", `a"b`, `a\b`, "", "#profile", "line\nbreak"}
	cfg := &config.Config{Root: t.TempDir(), Orgs: []string{"acme"}, Editor: want}
	writeConfigFile(t, cfg)
	s := testSettings(t, cfg, nil, nil, nil)
	loaded := s.Load()
	_, err := s.Save(SettingsInput{
		Orgs: loaded.Orgs, Root: loaded.Root, Editor: loaded.Editor,
		Launch: loaded.Launch, Stickers: loaded.Stickers,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Snapshot().Editor; !reflect.DeepEqual(got, want) {
		t.Fatalf("editor arguments changed: %q, want %q", got, want)
	}
}

func defaultStickers() []config.Sticker { return (&config.Config{}).EffectiveStickers() }

func TestSettingsLoadAnswersTheEffectiveStickers(t *testing.T) {
	if got := testSettings(t, &config.Config{}, nil, nil, nil).Load().Stickers; !reflect.DeepEqual(got, defaultStickers()) {
		t.Fatalf("default stickers = %#v", got)
	}
	listed := []config.Sticker{{ID: "a1b2c3d4", Colour: "pink", Symbol: "heart", Label: "Ship it"}}
	if got := testSettings(t, &config.Config{Stickers: &listed}, nil, nil, nil).Load().Stickers; !reflect.DeepEqual(got, listed) {
		t.Fatalf("listed stickers = %#v", got)
	}
}

func TestSettingsLoadListsTheRunnerIdsLaunchKnows(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	if got := s.Load().Runners; got == nil || len(got) != 0 {
		t.Fatalf("no runner source = %#v, want an empty list", got)
	}
	s.runners = func() ([]assembly.Runner, error) {
		return []assembly.Runner{{ID: "claude", Label: "Claude Code", Installed: true}, {ID: "agy", Label: "Antigravity"}}, nil
	}
	want := []RunnerLegend{{ID: "claude", Label: "Claude Code"}, {ID: "agy", Label: "Antigravity"}}
	if got := s.Load().Runners; !reflect.DeepEqual(got, want) {
		t.Fatalf("runners = %#v, want %#v", got, want)
	}
}

func TestSettingsLoadAnswersEmptyStringForNoLaunchOverrides(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	if got := s.Load().Launch; got != "" {
		t.Fatalf("launch = %q, want empty", got)
	}
}

func TestSettingsLoadOffersTheLinearEntryAndWhetherInstallWouldReplaceOne(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	got := s.Load().Linear
	if got.Path != lineartools.ConfigPath || got.Replaces || got.Error != "" || !strings.Contains(got.Snippet, `"openIssue"`) {
		t.Fatalf("linear = %+v", got)
	}
	if err := os.WriteFile(s.linear.File, []byte(`{"openIssue": {"path": "/bin/other"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.Load().Linear; !got.Replaces {
		t.Fatalf("an existing different entry did not report a replacement: %+v", got)
	}
}

func TestSettingsLoadReportsWhyTheLinearEntryIsUnavailable(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	s.linear.Command = nil
	if got := s.Load().Linear; got.Snippet != "" || got.Error != lineartools.ErrNoCommand.Error() {
		t.Fatalf("linear = %+v", got)
	}
}

func TestSettingsInstallLinearKeepsOtherKeysAndClearsTheReplacement(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	if err := os.WriteFile(s.linear.File, []byte(`{"openIssue": {"path": "/bin/other"}, "keep": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	view, err := s.InstallLinear()
	if err != nil {
		t.Fatal(err)
	}
	if view.Replaces {
		t.Fatalf("after install = %+v", view)
	}
	raw, _ := os.ReadFile(s.linear.File)
	if !strings.Contains(string(raw), `"keep": true`) || !strings.Contains(string(raw), "--linear-issue") {
		t.Fatalf("installed %s", raw)
	}
}

func TestSettingsSaveLeavesTheLinearFileAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	s := testSettings(t, cfg, nil, nil, nil)
	if _, err := s.Save(SettingsInput{Orgs: cfg.Orgs, Root: cfg.Root, Stickers: defaultStickers()}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.linear.File); !os.IsNotExist(err) {
		t.Fatal("Save wrote coding-tools.json")
	}
}

func TestSettingsRelaunchNeedsARelauncher(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, func() { t.Fatal("quit without a successor") })
	if err := s.Relaunch(); !errors.Is(err, ErrNoRelaunch) {
		t.Fatalf("Relaunch = %v, want ErrNoRelaunch", err)
	}
	s.relaunch = func() error { return errors.New("successor never answered") }
	if err := s.Relaunch(); err == nil {
		t.Fatal("a failed relaunch reported success")
	}
}

func TestSettingsOpenConfigRunsTheOpenTextArgvOnConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	if err := s.OpenConfig(); !errors.Is(err, ErrNoOpenTextCommand) {
		t.Fatalf("OpenConfig without a command = %v", err)
	}
	record := filepath.Join(t.TempDir(), "opened")
	var asked string
	s.openText = func(path string) []string {
		asked = path
		return []string{"/bin/sh", "-c", `printf %s "$1" > "$2"`, "sh", path, record}
	}
	if err := s.OpenConfig(); err != nil {
		t.Fatal(err)
	}
	if asked != config.Path() {
		t.Fatalf("opened %q, want %q", asked, config.Path())
	}
	if b, _ := os.ReadFile(record); string(b) != config.Path() {
		t.Fatalf("the command saw %q", b)
	}
}

// Save refuses Orgs, Root, Editor, then Launch in that order, and never
// writes config.json for an input that fails any of them.
func TestSettingsSaveRefusesTheFirstInvalidFieldAndWritesNothing(t *testing.T) {
	unwritable := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(unwritable, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name           string
		in             SettingsInput
		validateEditor func([]string) error
		validateLaunch func(map[string][]string) error
		wantField      string
	}{
		{name: "no orgs", in: SettingsInput{Root: t.TempDir()}, wantField: "orgs"},
		{
			name:      "orgs are all blank",
			in:        SettingsInput{Orgs: []string{"", "   "}, Root: t.TempDir()},
			wantField: "orgs",
		},
		{name: "empty root", in: SettingsInput{Orgs: []string{"acme"}, Root: "   "}, wantField: "root"},
		{
			name:      "root cannot be created",
			in:        SettingsInput{Orgs: []string{"acme"}, Root: filepath.Join(unwritable, "sub")},
			wantField: "root",
		},
		{
			name:      "editor does not shlex-split",
			in:        SettingsInput{Orgs: []string{"acme"}, Root: t.TempDir(), Editor: `vim "`},
			wantField: "editor",
		},
		{
			name:           "editor refused by validateEditor",
			in:             SettingsInput{Orgs: []string{"acme"}, Root: t.TempDir(), Editor: "vim {}"},
			validateEditor: func([]string) error { return errors.New("vim is not installed") },
			wantField:      "editor",
		},
		{
			name:      "launch is not valid json",
			in:        SettingsInput{Orgs: []string{"acme"}, Root: t.TempDir(), Launch: "{not json"},
			wantField: "launch",
		},
		{
			name:           "launch refused by validateLaunch",
			in:             SettingsInput{Orgs: []string{"acme"}, Root: t.TempDir(), Launch: `{"claude": ["claude"]}`},
			validateLaunch: func(map[string][]string) error { return errors.New("nope") },
			wantField:      "launch",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := &config.Config{Root: t.TempDir()}
			s := testSettings(t, cfg, tc.validateEditor, tc.validateLaunch, nil)

			_, err := s.Save(tc.in)
			if err == nil {
				t.Fatal("Save did not refuse an invalid field")
			}
			if !strings.HasPrefix(err.Error(), tc.wantField+": ") {
				t.Fatalf("error = %q, want it to start with %q", err.Error(), tc.wantField+": ")
			}
			if _, statErr := os.Stat(config.Path()); !os.IsNotExist(statErr) {
				t.Fatal("Save wrote config.json despite refusing the input")
			}
		})
	}
}

// Never zero owners holds at both write points, not just first run: clearing
// the list here would write an ownerless config that stays welcomed, so the
// question never comes back.
func TestSettingsSaveRefusesAnEmptyOwnerListAndWritesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir(), Welcomed: true}
	s := testSettings(t, cfg, nil, nil, nil)

	root := filepath.Join(t.TempDir(), "sessions")
	for _, orgs := range [][]string{nil, {}, {"", "   "}} {
		_, err := s.Save(SettingsInput{Orgs: orgs, Root: root})
		if !errors.Is(err, ErrNoOwners) {
			t.Fatalf("Save(%q) error = %v, want ErrNoOwners", orgs, err)
		}
		if !strings.HasPrefix(err.Error(), "orgs: ") {
			t.Fatalf("refusal %q does not name the field the panel puts it on", err)
		}
	}

	if !reflect.DeepEqual(cfg.Orgs, []string{"acme"}) {
		t.Fatalf("live orgs = %#v; a refused save cleared them", cfg.Orgs)
	}
	if _, err := os.Stat(config.Path()); !os.IsNotExist(err) {
		t.Fatal("a refused save wrote config.json")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("a refused save created the sessions root it was given")
	}
	if _, err := os.Stat(s.linear.File); !os.IsNotExist(err) {
		t.Fatal("a refused save wrote coding-tools.json")
	}
}

// A JSON syntax error in Launch is reported with Go's own message intact,
// rather than a rewritten one.
func TestSettingsSaveKeepsTheJSONErrorMessageVerbatim(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Root: t.TempDir()}
	s := testSettings(t, cfg, nil, nil, nil)

	var want map[string][]string
	wantErr := json.Unmarshal([]byte("{not json"), &want)
	if wantErr == nil {
		t.Fatal("test fixture is valid JSON; pick a genuinely malformed one")
	}

	_, err := s.Save(SettingsInput{Orgs: []string{"acme"}, Root: t.TempDir(), Launch: "{not json"})
	if err == nil {
		t.Fatal("Save accepted malformed JSON")
	}
	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("Save error %q does not carry Go's own message %q", err, wantErr)
	}
}

func TestSettingsSaveRefusesAnInvalidStickerBeforeWritingEitherFile(t *testing.T) {
	cases := []struct {
		name  string
		edit  func([]config.Sticker) []config.Sticker
		field string
	}{
		{"blank label", func(s []config.Sticker) []config.Sticker { s[1].Label = " \t"; return s }, "stickers.1: cannot be empty"},
		{"unknown colour", func(s []config.Sticker) []config.Sticker { s[2].Colour = "beige"; return s }, "stickers.2: "},
		{"unknown symbol", func(s []config.Sticker) []config.Sticker { s[0].Symbol = "moon"; return s }, "stickers.0: "},
		{"duplicate pair", func(s []config.Sticker) []config.Sticker {
			return append(s, config.Sticker{Colour: "blue", Symbol: "star", Label: "Again"})
		}, "stickers.4: " + stickerDuplicate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
			s := testSettings(t, cfg, nil, nil, nil)

			_, err := s.Save(SettingsInput{
				Orgs: cfg.Orgs, Root: cfg.Root, Stickers: tc.edit(defaultStickers()),
			})
			if err == nil || !strings.HasPrefix(err.Error(), tc.field) {
				t.Fatalf("Save error = %v, want %q", err, tc.field)
			}
			if _, statErr := os.Stat(config.Path()); !os.IsNotExist(statErr) {
				t.Fatal("Save wrote config.json despite refusing a sticker")
			}
			if _, statErr := os.Stat(s.linear.File); !os.IsNotExist(statErr) {
				t.Fatal("Save wrote coding-tools.json despite refusing a sticker")
			}
		})
	}
}

func TestSettingsSaveGivesNewStickersAnIdAndKeepsExistingOnes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	s := testSettings(t, cfg, nil, nil, nil)
	in := append(defaultStickers(), config.Sticker{Colour: "pink", Symbol: "heart", Label: " Ship it "})
	if _, err := s.Save(SettingsInput{Orgs: cfg.Orgs, Root: cfg.Root, Stickers: in}); err != nil {
		t.Fatal(err)
	}
	onDisk, err := config.ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Stickers == nil || onDisk.StickerLabels != nil {
		t.Fatalf("on disk = stickers %v, labels %v", onDisk.Stickers, onDisk.StickerLabels)
	}
	got := *onDisk.Stickers
	if len(got) != 5 || got[0].ID != "star" || got[3].ID != "exclamation" {
		t.Fatalf("stickers = %#v", got)
	}
	if added := got[4]; len(added.ID) != 8 || added.Label != "Ship it" || slices.Contains([]string{"star", "bookmark", "question", "exclamation"}, added.ID) {
		t.Fatalf("added sticker = %#v", added)
	}
	if live := cfg.EffectiveStickers(); !reflect.DeepEqual(live, got) {
		t.Fatalf("live stickers = %#v, want %#v", live, got)
	}
}

func TestSettingsSaveKeepsAnEmptyStickerList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	s := testSettings(t, cfg, nil, nil, nil)
	if _, err := s.Save(SettingsInput{Orgs: cfg.Orgs, Root: cfg.Root, Stickers: []config.Sticker{}}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := config.ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.EffectiveStickers(); len(got) != 0 {
		t.Fatalf("reloaded stickers = %#v", got)
	}
}

// A successful Save writes the whole config to disk, dedupes Orgs, and
// updates every live field except Root.
func TestSettingsSaveWritesTheFileAndUpdatesEveryLiveFieldExceptRoot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	liveRoot := t.TempDir()
	cfg := &config.Config{Orgs: []string{"old-org"}, Root: liveRoot, Editor: []string{"old-editor"}}
	s := testSettings(t, cfg, nil, nil, nil)

	newRoot := t.TempDir()
	result, err := s.Save(SettingsInput{
		Orgs:     []string{" acme ", "acme", "", "second-org"},
		Root:     newRoot,
		Editor:   "code --wait {}",
		Launch:   `{"claude": ["claude", "--verbose"]}`,
		Stickers: []config.Sticker{{ID: "star", Colour: "blue", Symbol: "star", Label: " Priority "}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired {
		t.Fatal("changing root did not ask for a restart")
	}

	wantOrgs := []string{"acme", "second-org"}
	if !reflect.DeepEqual(cfg.Orgs, wantOrgs) {
		t.Fatalf("live orgs = %#v, want %#v", cfg.Orgs, wantOrgs)
	}
	if !reflect.DeepEqual(cfg.Editor, []string{"code", "--wait", "{}"}) {
		t.Fatalf("live editor = %#v", cfg.Editor)
	}
	wantLaunch := map[string][]string{"claude": {"claude", "--verbose"}}
	if !reflect.DeepEqual(cfg.Launch, wantLaunch) {
		t.Fatalf("live launch = %#v, want %#v", cfg.Launch, wantLaunch)
	}
	if cfg.Root != liveRoot {
		t.Fatalf("live root changed to %q; Root must stay the boot value", cfg.Root)
	}
	wantStickers := []config.Sticker{{ID: "star", Colour: "blue", Symbol: "star", Label: "Priority"}}
	if got := cfg.EffectiveStickers(); !reflect.DeepEqual(got, wantStickers) {
		t.Fatalf("live stickers = %#v, want %#v", got, wantStickers)
	}

	raw, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var onDisk config.Config
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Root != newRoot {
		t.Fatalf("on-disk root = %q, want %q", onDisk.Root, newRoot)
	}
	if !reflect.DeepEqual(onDisk.Orgs, wantOrgs) {
		t.Fatalf("on-disk orgs = %#v, want %#v", onDisk.Orgs, wantOrgs)
	}
	if !reflect.DeepEqual(onDisk.Editor, []string{"code", "--wait", "{}"}) {
		t.Fatalf("on-disk editor = %#v", onDisk.Editor)
	}
	if !reflect.DeepEqual(onDisk.Launch, wantLaunch) {
		t.Fatalf("on-disk launch = %#v, want %#v", onDisk.Launch, wantLaunch)
	}
	if got := onDisk.EffectiveStickers(); !reflect.DeepEqual(got, wantStickers) {
		t.Fatalf("on-disk stickers = %#v, want %#v", got, wantStickers)
	}
}

func TestSettingsSaveWakesChromeOnceOnlyWhenNormalisedStickersChange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Orgs: []string{"acme"}, Root: root}
	s := testSettings(t, cfg, nil, nil, nil)
	wakes := 0
	s.wakeChrome = func() { wakes++ }

	save := func(stickers []config.Sticker) SaveResult {
		t.Helper()
		result, err := s.Save(SettingsInput{
			Orgs: cfg.Orgs, Root: root, Stickers: stickers,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	if result := save(defaultStickers()); result.RestartRequired {
		t.Fatal("persisting effective defaults asked for a restart")
	}
	if wakes != 0 {
		t.Fatalf("persisting unchanged defaults woke chrome %d times", wakes)
	}

	custom := defaultStickers()
	custom[0].Label = "Priority"
	if result := save(custom); result.RestartRequired {
		t.Fatal("changing a sticker asked for a restart")
	}
	if wakes != 1 {
		t.Fatalf("changing a sticker woke chrome %d times, want once", wakes)
	}

	padded := defaultStickers()
	padded[0].Label = " Priority\t"
	save(padded)
	if wakes != 1 {
		t.Fatalf("re-saving normalised stickers woke chrome %d times, want once total", wakes)
	}
}

func TestSettingsSaveDoesNotWakeChromeWhenConfigWriteFails(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(configHome, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", configHome)
	root := t.TempDir()
	cfg := &config.Config{Orgs: []string{"acme"}, Root: root}
	s := testSettings(t, cfg, nil, nil, nil)
	wakes := 0
	orgEvents := 0
	s.wakeChrome = func() { wakes++ }
	s.emit = func(event string, _ any) {
		if event == orgsChangedEvent {
			orgEvents++
		}
	}

	stickers := defaultStickers()
	stickers[0].Label = "Priority"
	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme", "other"}, Root: root, Stickers: stickers,
	}); err == nil {
		t.Fatal("Save succeeded despite an unwritable config path")
	}
	if wakes != 0 {
		t.Fatalf("failed config write woke chrome %d times", wakes)
	}
	if orgEvents != 0 {
		t.Fatalf("failed config write emitted %d org events", orgEvents)
	}
	if cfg.Stickers != nil {
		t.Fatalf("failed config write changed live stickers to %#v", cfg.Stickers)
	}
}

func TestSaveConfigSerializesOverlappingWritersThroughDiskAndLiveState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Orgs: []string{"initial"}, Root: root}
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})
	errs := make(chan error, 2)

	go func() {
		err := saveConfig(cfg, "", func(next, live *config.Config) {
			close(firstEntered)
			<-releaseFirst
			next.Orgs, live.Orgs = []string{"first"}, []string{"first"}
		}, nil, nil)
		errs <- err
	}()
	<-firstEntered
	go func() {
		close(secondStarted)
		err := saveConfig(cfg, "", func(next, live *config.Config) {
			next.Orgs, live.Orgs = []string{"second"}, []string{"second"}
		}, nil, nil)
		errs <- err
	}()
	<-secondStarted
	requireBlocked(t, errs, "second config save")
	close(releaseFirst)

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	live := cfg.Snapshot()
	if !reflect.DeepEqual(live.Orgs, []string{"second"}) {
		t.Fatalf("live orgs = %#v, want final serialized save", live.Orgs)
	}
	raw, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var disk config.Config
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(disk.Orgs, live.Orgs) {
		t.Fatalf("disk orgs = %#v, live orgs = %#v", disk.Orgs, live.Orgs)
	}
}

// Save rewrites the whole file, so a marker it does not carry forward re-arms
// first run the moment anyone opens the panel.
func TestSettingsSaveKeepsTheWelcomedMarker(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Root: root, Welcomed: true}
	writeConfigFile(t, cfg)
	s := testSettings(t, cfg, nil, nil, nil)

	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme"}, Root: root, Stickers: defaultStickers(),
	}); err != nil {
		t.Fatal(err)
	}
	if !cfg.Welcomed {
		t.Fatal("a save cleared the live welcomed marker")
	}
	raw, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var onDisk config.Config
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if !onDisk.Welcomed {
		t.Fatalf("on-disk config lost the welcomed marker: %s", raw)
	}
}

// RestartRequired compares the expanded, cleaned input Root against the live
// one: a no-op retype (leading ~, trailing slash) never claims a restart, an
// unrelated field change never claims one either, a genuinely different root
// does, and retyping the live value again clears it.
func TestSettingsSaveRestartRequiredComparesNormalisedRootsAndClearsOnRevert(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)

	liveRoot := filepath.Join(home, "work")
	if err := os.MkdirAll(liveRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: liveRoot}
	s := testSettings(t, cfg, nil, nil, nil)

	result, err := s.Save(SettingsInput{
		Orgs: []string{"acme"}, Root: "~/work/", Stickers: defaultStickers(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("retyping the live root with ~ and a trailing slash asked for a restart")
	}

	result, err = s.Save(SettingsInput{
		Root: liveRoot, Orgs: []string{"acme"}, Stickers: defaultStickers(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("changing only Orgs asked for a restart")
	}

	otherRoot := t.TempDir()
	result, err = s.Save(SettingsInput{
		Orgs: []string{"acme"}, Root: otherRoot, Stickers: defaultStickers(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired {
		t.Fatal("a genuinely different root did not ask for a restart")
	}
	if cfg.Root != liveRoot {
		t.Fatalf("live root mutated to %q", cfg.Root)
	}

	result, err = s.Save(SettingsInput{
		Orgs: []string{"acme"}, Root: liveRoot, Stickers: defaultStickers(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("typing the live root back did not clear the restart flag")
	}
}

// Settings.Relaunch must run the exact teardown closing the conversation window
// runs, not a bare renderer Quit: every open session ends and the renderer's
// own Quit is reached only after that teardown completes.
func TestSettingsRelaunchRunsTheSameTeardownClosingTheWindowRuns(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	quit := sync.OnceFunc(func() {
		windows.stopAll()
		reg.stopAll()
		r.Quit()
	})
	settings := testSettings(t, &config.Config{}, nil, nil, quit)
	relaunched := false
	settings.relaunch = func() error { relaunched = true; return nil }

	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		done <- run(r, term, windows, opts, quit)
	}()
	t.Cleanup(func() {
		r.Quit()
		<-stopped
	})

	<-r.opened
	state := shownSession(t, reg)

	if err := settings.Relaunch(); err != nil {
		t.Fatal(err)
	}
	if !relaunched {
		t.Fatal("Settings.Relaunch quit without starting a successor")
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	quitReached := r.quit
	r.mu.Unlock()
	if !quitReached {
		t.Fatal("Settings.Relaunch did not reach the renderer's own Quit")
	}
	state.mu.Lock()
	stoppedSession := state.stopped
	state.mu.Unlock()
	if !stoppedSession {
		t.Fatal("Settings.Relaunch did not stop the session registry")
	}
}

func TestSettingsSaveAnnouncesOrgsThatChanged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	var announced [][]string
	s := testSettings(t, cfg, nil, nil, nil)
	s.emit = func(event string, payload any) {
		if event == orgsChangedEvent {
			announced = append(announced, payload.([]string))
		}
	}

	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme", "other"}, Root: cfg.Root, Stickers: defaultStickers(),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme", "other"}, Root: cfg.Root, Stickers: defaultStickers(),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	want := [][]string{{"acme", "other"}}
	if !reflect.DeepEqual(announced, want) {
		t.Fatalf("announced = %#v, want %#v — once, for the save that changed them", announced, want)
	}
}

func TestOverlappingSettingsSavesAnnounceOrgsInCommitOrder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Orgs: []string{"initial"}, Root: root}
	var mu sync.Mutex
	var announced [][]string
	firstEmitting := make(chan struct{})
	releaseFirst := make(chan struct{})
	first := testSettings(t, cfg, nil, nil, nil)
	first.emit = func(event string, payload any) {
		if event != orgsChangedEvent {
			return
		}
		mu.Lock()
		announced = append(announced, append([]string(nil), payload.([]string)...))
		mu.Unlock()
		close(firstEmitting)
		<-releaseFirst
	}
	secondValidated := make(chan struct{})
	second := testSettings(t, cfg, func([]string) error {
		close(secondValidated)
		return nil
	}, nil, nil)
	second.emit = func(event string, payload any) {
		if event != orgsChangedEvent {
			return
		}
		mu.Lock()
		announced = append(announced, append([]string(nil), payload.([]string)...))
		mu.Unlock()
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := first.Save(SettingsInput{
			Orgs: []string{"first"}, Root: root,
			Stickers: defaultStickers(),
		})
		firstDone <- err
	}()
	<-firstEmitting
	secondDone := make(chan error, 1)
	go func() {
		_, err := second.Save(SettingsInput{
			Orgs: []string{"second"}, Root: root, Editor: "code",
			Stickers: defaultStickers(),
		})
		secondDone <- err
	}()
	<-secondValidated
	requireBlocked(t, secondDone, "second Settings save")
	mu.Lock()
	beforeRelease := append([][]string(nil), announced...)
	mu.Unlock()
	if !reflect.DeepEqual(beforeRelease, [][]string{{"first"}}) {
		t.Fatalf("announced before first notification completed = %#v", beforeRelease)
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := append([][]string(nil), announced...)
	mu.Unlock()
	want := [][]string{{"first"}, {"second"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("announced = %#v, want commit order %#v", got, want)
	}
	live := cfg.Snapshot()
	raw, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var disk config.Config
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(live.Orgs, []string{"second"}) || !reflect.DeepEqual(disk.Orgs, live.Orgs) || !reflect.DeepEqual(got[len(got)-1], live.Orgs) {
		t.Fatalf("last event/disk/live orgs = %#v / %#v / %#v", got[len(got)-1], disk.Orgs, live.Orgs)
	}
	if !reflect.DeepEqual(disk.Editor, []string{"code"}) || !reflect.DeepEqual(live.Editor, disk.Editor) {
		t.Fatalf("final disk/live editor = %#v / %#v", disk.Editor, live.Editor)
	}
}

func TestSettingsChimeReadsOnForAnUntouchedConfigAndSavesAsQuiet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	s := testSettings(t, cfg, nil, nil, nil)
	if !s.Load().Chime {
		t.Fatal("a config that never mentioned the chime loads it as off")
	}

	input := SettingsInput{
		Orgs: cfg.Orgs, Root: cfg.Root, Stickers: defaultStickers(),
	}
	if _, err := s.Save(input); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Snapshot().Quiet || !loaded.Quiet || s.Load().Chime {
		t.Fatalf("unticking the chime left live quiet %v, on-disk quiet %v", cfg.Snapshot().Quiet, loaded.Quiet)
	}

	input.Chime = true
	if _, err := s.Save(input); err != nil {
		t.Fatal(err)
	}
	if cfg.Snapshot().Quiet || !s.Load().Chime {
		t.Fatal("ticking the chime again left the config quiet")
	}
}

func TestSettingsLoadAnswersTheEffectiveUIScaleAndItsSteps(t *testing.T) {
	for _, tc := range []struct{ stored, want int }{{0, 100}, {130, 130}, {85, 100}} {
		view := testSettings(t, &config.Config{UIScale: tc.stored}, nil, nil, nil).Load()
		if view.UIScale != tc.want {
			t.Fatalf("stored %d loads as %d, want %d", tc.stored, view.UIScale, tc.want)
		}
		if !reflect.DeepEqual(view.UIScaleSteps, config.UIScaleSteps()) {
			t.Fatalf("steps = %v", view.UIScaleSteps)
		}
	}
}

func TestSettingsSaveRefusesAnOffGridUIScaleAndWritesNothing(t *testing.T) {
	for _, scale := range []int{70, 105, 160} {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
		s := testSettings(t, cfg, nil, nil, nil)
		_, err := s.Save(SettingsInput{
			Orgs: cfg.Orgs, Root: cfg.Root, Stickers: defaultStickers(), UIScale: scale,
		})
		if !errors.Is(err, ErrUIScale) || !strings.HasPrefix(err.Error(), settingsFieldUIScale+": ") {
			t.Fatalf("scale %d: error = %v", scale, err)
		}
		if _, statErr := os.Stat(config.Path()); !os.IsNotExist(statErr) {
			t.Fatalf("scale %d: Save wrote config.json despite refusing it", scale)
		}
	}
}

func TestSettingsSaveAnnouncesTheUIScaleOnlyWhenItChanges(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	var announced []int
	s := testSettings(t, cfg, nil, nil, nil)
	s.emit = func(event string, payload any) {
		if event == uiScaleEvent {
			announced = append(announced, payload.(int))
		}
	}
	input := SettingsInput{Orgs: cfg.Orgs, Root: cfg.Root, Stickers: defaultStickers()}
	for _, scale := range []int{100, 150, 150, 0} {
		input.UIScale = scale
		if _, err := s.Save(input); err != nil {
			t.Fatalf("Save(%d): %v", scale, err)
		}
	}
	if want := []int{150, 100}; !reflect.DeepEqual(announced, want) {
		t.Fatalf("announced = %v, want %v", announced, want)
	}
}

func TestSettingsSaveStoresTheUIScaleAndDropsItAtTheDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	s := testSettings(t, cfg, nil, nil, nil)
	input := SettingsInput{
		Orgs: cfg.Orgs, Root: cfg.Root, Stickers: defaultStickers(), UIScale: 150,
	}
	if _, err := s.Save(input); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"uiScale": 150`) || cfg.EffectiveUIScale() != 150 {
		t.Fatalf("150 left live %d and wrote %s", cfg.EffectiveUIScale(), b)
	}
	input.UIScale = 100
	if _, err := s.Save(input); err != nil {
		t.Fatal(err)
	}
	if b, err = os.ReadFile(config.Path()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "uiScale") || cfg.EffectiveUIScale() != 100 {
		t.Fatalf("100 left live %d and wrote %s", cfg.EffectiveUIScale(), b)
	}
}

func TestAdjustUIScaleStepsClampsAndResets(t *testing.T) {
	cases := []struct {
		name          string
		stored        int
		action        string
		want, written int
		announced     bool
	}{
		{"in from 100", 0, uiScaleActionIn, 110, 110, true},
		{"out from 100", 0, uiScaleActionOut, 90, 90, true},
		{"in at the top", 150, uiScaleActionIn, 150, -1, false},
		{"out at the bottom", 80, uiScaleActionOut, 80, -1, false},
		{"out from the top", 150, uiScaleActionOut, 140, 140, true},
		{"in from the bottom", 80, uiScaleActionIn, 90, 90, true},
		{"reset from 150", 150, uiScaleActionReset, 100, 0, true},
		{"reset at 100", 0, uiScaleActionReset, 100, -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir(), Editor: []string{"vim"}, UIScale: tc.stored}
			s := testSettings(t, cfg, nil, nil, nil)
			if tc.written >= 0 {
				if err := config.Save(cfg); err != nil {
					t.Fatal(err)
				}
			}
			var announced []int
			s.emit = func(event string, payload any) {
				if event == uiScaleEvent {
					announced = append(announced, payload.(int))
				}
			}
			got, err := s.AdjustUIScale(tc.action)
			if err != nil || got != tc.want {
				t.Fatalf("AdjustUIScale = %d, %v; want %d", got, err, tc.want)
			}
			if tc.announced != (len(announced) == 1 && announced[0] == tc.want) {
				t.Fatalf("announced %v", announced)
			}
			b, err := os.ReadFile(config.Path())
			if tc.written < 0 {
				if !os.IsNotExist(err) {
					t.Fatalf("a scale at its bound wrote config.json (%v)", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var onDisk config.Config
			if err := json.Unmarshal(b, &onDisk); err != nil {
				t.Fatal(err)
			}
			if onDisk.UIScale != tc.written || !reflect.DeepEqual(onDisk.Editor, []string{"vim"}) || cfg.EffectiveUIScale() != tc.want {
				t.Fatalf("wrote %s, live scale %d", b, cfg.EffectiveUIScale())
			}
		})
	}
}

func thoughtsInput(rows ...ThoughtsRow) SettingsInput {
	return SettingsInput{Orgs: []string{"acme"}, Stickers: defaultStickers(),
		Thoughts: ThoughtsInput{Roots: rows}}
}

func TestSettingsLoadAnswersTheThoughtsFoldersAndTheDerivedDefault(t *testing.T) {
	cfg := &config.Config{Root: "/sessions", Thoughts: config.Thoughts{Roots: []config.ThoughtsRoot{
		{ID: "work", Path: "/sync/work", Orgs: []string{"acme", "acme-labs"}},
	}}}
	writeConfigFile(t, cfg)
	got := testSettings(t, cfg, nil, nil, nil).Load().Thoughts
	want := ThoughtsView{Derived: "/sessions/thoughts", Roots: []ThoughtsRow{{ID: "work", Path: "/sync/work", Orgs: "acme, acme-labs", Missing: true}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("thoughts = %+v, want %+v", got, want)
	}
}

func TestSettingsSaveRefusesThoughtsFoldersThatCouldMixMaterialBeforeWritingEitherFile(t *testing.T) {
	shared := t.TempDir()
	for name, rows := range map[string][]ThoughtsRow{
		"org on two folders":  {{ID: "a", Path: shared + "/a", Orgs: "acme"}, {ID: "b", Path: shared + "/b", Orgs: "other, ACME"}},
		"overlapping folders": {{ID: "a", Path: shared, Orgs: "acme"}, {ID: "b", Path: shared + "/b", Orgs: "other"}},
		"reserved id":         {{ID: "default", Path: shared, Orgs: "acme"}},
		"bad id":              {{ID: "no spaces", Path: shared, Orgs: "acme"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			root := t.TempDir()
			reconfigures := 0
			s := testSettings(t, &config.Config{Root: root}, nil, nil, nil)
			s.reconfigure = func(*config.Config) error { reconfigures++; return nil }
			in := thoughtsInput(rows...)
			in.Root = root
			_, err := s.Save(in)
			if err == nil || !strings.HasPrefix(err.Error(), "thoughts: ") {
				t.Fatalf("Save = %v, want a thoughts field refusal", err)
			}
			for _, path := range []string{config.Path(), s.linear.File} {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("a refused save wrote %s", path)
				}
			}
			if reconfigures != 0 {
				t.Fatal("a refused save reconfigured search")
			}
		})
	}
}

func TestAdjustUIScaleRefusesAnUnknownAction(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := testSettings(t, &config.Config{Root: t.TempDir()}, nil, nil, nil)
	if _, err := s.AdjustUIScale("sideways"); !errors.Is(err, ErrUIScaleAction) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(config.Path()); !os.IsNotExist(err) {
		t.Fatal("an unknown action wrote config.json")
	}
}

func TestAdjustUIScaleWritesTheFileAsWrittenNotTheRuntimeOverrides(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(config.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(), []byte(`{"orgs": ["acme"], "root": "~/work-from-file"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QROUTON_ROOT", t.TempDir())
	t.Setenv("QROUTON_ORGS", "override")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := testSettings(t, cfg, nil, nil, nil)

	if _, err := s.AdjustUIScale(uiScaleActionIn); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var onDisk config.Config
	if err := json.Unmarshal(b, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Root != "~/work-from-file" || !reflect.DeepEqual(onDisk.Orgs, []string{"acme"}) || onDisk.UIScale != 110 {
		t.Fatalf("wrote %s", b)
	}
	if live := cfg.Snapshot(); live.UIScale != 110 || !reflect.DeepEqual(live.Orgs, []string{"override"}) {
		t.Fatalf("live config = %+v", live)
	}
}

func overriddenSettings(t *testing.T) (*Settings, *config.Config, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	shared := filepath.Join(home, "shared")
	for _, dir := range []string{filepath.Join(home, "work"), shared} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(config.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	file := `{"orgs": ["acme"], "root": "~/work", "thoughts": {"roots": [{"id": "work", "path": "~/shared", "orgs": ["acme"]}]}}`
	if err := os.WriteFile(config.Path(), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	override := t.TempDir()
	t.Setenv("QROUTON_ROOT", override)
	t.Setenv("QROUTON_ORGS", "pinned-org")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return testSettings(t, cfg, nil, nil, nil), cfg, override
}

func TestSettingsLoadShowsTheFileValuesAndNamesTheOverrides(t *testing.T) {
	s, _, _ := overriddenSettings(t)

	got := s.Load()
	if got.Root != "~/work" || !reflect.DeepEqual(got.Orgs, []string{"acme"}) || got.Thoughts.Roots[0].Path != "~/shared" {
		t.Fatalf("view = root %q, orgs %v, thoughts %+v", got.Root, got.Orgs, got.Thoughts)
	}
	if !got.Pinned.Root || !got.Pinned.Orgs {
		t.Fatalf("pinned = %+v", got.Pinned)
	}
}

func TestSettingsLoadShowsTheUnexpandedDefaultRootForAnEmptyFile(t *testing.T) {
	writeConfigFile(t, &config.Config{})
	t.Setenv("QROUTON_ROOT", "")
	t.Setenv("QROUTON_ORGS", "")
	got := testSettings(t, &config.Config{Root: "/expanded/work"}, nil, nil, nil).Load()
	if got.Root != config.DefaultRoot() || got.Pinned.Root || got.Pinned.Orgs {
		t.Fatalf("root = %q, pinned = %+v", got.Root, got.Pinned)
	}
}

func TestSettingsSaveUnderOverridesWritesTheFileAsTypedAndKeepsTheOverridesLive(t *testing.T) {
	s, cfg, override := overriddenSettings(t)
	view := s.Load()

	res, err := s.Save(SettingsInput{
		Orgs: view.Orgs, Root: view.Root, Stickers: view.Stickers, Chime: false,
		Thoughts: ThoughtsInput{Default: view.Thoughts.Default, Roots: view.Thoughts.Roots},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RestartRequired {
		t.Fatal("an unchanged root asked for a restart under QROUTON_ROOT")
	}
	onDisk, err := config.ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Root != "~/work" || !reflect.DeepEqual(onDisk.Orgs, []string{"acme"}) || onDisk.Thoughts.Roots[0].Path != "~/shared" {
		t.Fatalf("on disk = root %q, orgs %v, thoughts %+v", onDisk.Root, onDisk.Orgs, onDisk.Thoughts)
	}
	if !onDisk.Quiet {
		t.Fatal("the typed chime did not reach the file")
	}
	live := cfg.Snapshot()
	if live.Root != override || !reflect.DeepEqual(live.Orgs, []string{"pinned-org"}) {
		t.Fatalf("live = root %q, orgs %v", live.Root, live.Orgs)
	}
	if want := filepath.Join(os.Getenv("HOME"), "shared"); live.Thoughts.Roots[0].Path != want {
		t.Fatalf("live thoughts path = %q, want %q", live.Thoughts.Roots[0].Path, want)
	}
}

func TestSettingsSaveUnderAnOverrideAsksForARestartWhenTheFileRootChanges(t *testing.T) {
	s, _, _ := overriddenSettings(t)
	view := s.Load()
	res, err := s.Save(SettingsInput{
		Orgs: view.Orgs, Root: t.TempDir(), Stickers: view.Stickers,
		Thoughts: ThoughtsInput{Default: view.Thoughts.Default, Roots: view.Thoughts.Roots},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.RestartRequired {
		t.Fatal("a changed root did not ask for a restart")
	}
}

func TestSettingsSaveUnderAnOverrideKeepsTheRestartUntilTheRootIsTypedBack(t *testing.T) {
	s, _, _ := overriddenSettings(t)
	view := s.Load()
	in := SettingsInput{Orgs: view.Orgs, Root: t.TempDir(), Stickers: view.Stickers,
		Thoughts: ThoughtsInput{Default: view.Thoughts.Default, Roots: view.Thoughts.Roots}}
	for _, step := range []struct {
		root string
		want bool
	}{{in.Root, true}, {in.Root, true}, {view.Root, false}} {
		in.Root = step.root
		res, err := s.Save(in)
		if err != nil {
			t.Fatal(err)
		}
		if res.RestartRequired != step.want {
			t.Fatalf("Save(root %q) restart = %v, want %v", step.root, res.RestartRequired, step.want)
		}
	}
}

func TestSettingsSaveAcceptsNoOrgsWhileQROUTONORGSSuppliesThem(t *testing.T) {
	writeConfigFile(t, &config.Config{Root: t.TempDir()})
	t.Setenv("QROUTON_ORGS", "pinned-org")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := testSettings(t, cfg, nil, nil, nil)
	view := s.Load()
	if _, err := s.Save(SettingsInput{Orgs: view.Orgs, Root: view.Root, Stickers: view.Stickers}); err != nil {
		t.Fatalf("Save with orgs only from the env = %v", err)
	}
	if onDisk, _ := config.ReadFile(); len(onDisk.Orgs) != 0 {
		t.Fatalf("file orgs = %v", onDisk.Orgs)
	}
	if live := cfg.Snapshot().Orgs; !reflect.DeepEqual(live, []string{"pinned-org"}) {
		t.Fatalf("live orgs = %v", live)
	}
}

func TestSettingsSaveLeavesAnUnsetRootUnsetWhenTheDefaultIsKept(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfigFile(t, &config.Config{Orgs: []string{"acme"}})
	t.Setenv("QROUTON_ROOT", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := testSettings(t, cfg, nil, nil, nil)
	view := s.Load()
	res, err := s.Save(SettingsInput{Orgs: view.Orgs, Root: view.Root, Stickers: view.Stickers})
	if err != nil || res.RestartRequired {
		t.Fatalf("Save = %+v, %v", res, err)
	}
	if onDisk, _ := config.ReadFile(); onDisk.Root != "" {
		t.Fatalf("file root = %q, want unset", onDisk.Root)
	}
}

func TestSettingsSaveThatAddsAThoughtsFolderReachesSearchWithoutARestart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root, shared := t.TempDir(), t.TempDir()
	cfg := &config.Config{Root: root}
	set := vault.NewSet(vault.NewOllama(), t.TempDir())
	if err := set.Configure(thoughtsProfiles(cfg)); err != nil {
		t.Fatal(err)
	}
	s := testSettings(t, cfg, nil, nil, nil)
	s.reconfigure = reconfigureVault(set)
	link := filepath.Join(shared, "new-session")
	if set.ForPath(link) != nil {
		t.Fatal("the folder was searchable before it was added")
	}

	in := thoughtsInput(ThoughtsRow{ID: "work", Path: shared, Orgs: "acme"})
	in.Root = root
	if _, err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	if set.ForPath(link) == nil {
		t.Fatal("a session in the added folder has no thoughts root until a restart")
	}
	if got := cfg.RouteThoughts([]string{"acme"}).ID; got != "work" {
		t.Fatalf("new acme sessions route to %s", got)
	}
}

func TestSettingsSaveThatLeavesThoughtsAloneDoesNotReconfigureSearch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root, shared := t.TempDir(), t.TempDir()
	cfg := &config.Config{Root: root, Thoughts: config.Thoughts{Roots: []config.ThoughtsRoot{
		{ID: "work", Path: shared, Orgs: []string{"acme"}},
	}}}
	writeConfigFile(t, cfg)
	reconfigures := 0
	s := testSettings(t, cfg, nil, nil, nil)
	s.reconfigure = func(*config.Config) error { reconfigures++; return nil }
	view := s.Load()
	in := SettingsInput{Orgs: []string{"other"}, Root: root, Stickers: defaultStickers(),
		Thoughts: ThoughtsInput{Default: view.Thoughts.Default, Roots: view.Thoughts.Roots}}
	if _, err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	if reconfigures != 0 {
		t.Fatalf("an untouched thoughts section reconfigured search %d times", reconfigures)
	}
}

func TestSettingsSaveThatChangesTheRootHoldsBackItsThoughtsFoldersUntilTheRelaunch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("QROUTON_ROOT", "")
	t.Setenv("QROUTON_ORGS", "")
	oldRoot, newRoot := t.TempDir(), t.TempDir()
	shared := filepath.Join(oldRoot, "thoughts", "shared")
	if err := os.MkdirAll(filepath.Join(shared, "s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "thoughts", "private.md"), []byte("# Private\n\nNot for work.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: oldRoot}
	set := vault.NewSet(offlineEmbedder{}, t.TempDir())
	if err := set.Configure(thoughtsProfiles(cfg)); err != nil {
		t.Fatal(err)
	}
	reconfigures := 0
	s := testSettings(t, cfg, nil, nil, nil)
	s.reconfigure = func(c *config.Config) error { reconfigures++; return reconfigureVault(set)(c) }

	in := thoughtsInput(ThoughtsRow{ID: "work", Path: shared, Orgs: "acme"})
	in.Root = newRoot
	res, err := s.Save(in)
	if err != nil || !res.RestartRequired {
		t.Fatalf("Save = %+v, %v", res, err)
	}
	if reconfigures != 0 {
		t.Fatal("a save waiting on a relaunch reconfigured search")
	}
	if got := cfg.RouteThoughts([]string{"acme"}).ID; got != config.DefaultThoughtsID {
		t.Fatalf("acme sessions route to %s before the relaunch", got)
	}

	relaunched, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	sessionDir := t.TempDir()
	if err := os.Symlink(filepath.Join(shared, "s1"), filepath.Join(sessionDir, "thoughts")); err != nil {
		t.Fatal(err)
	}
	thoughts, err := sessionThoughts(openVault(relaunched), relaunched.ThoughtsRoots(), sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := thoughts.Read(vault.ReadRequest{Ref: "default:private.md"}); err == nil {
		t.Fatal("a shared session read the default folder")
	}
}

func TestSettingsSaveThatCannotReindexLeavesRoutingAndSearchAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root, shared := t.TempDir(), t.TempDir()
	cfg := &config.Config{Root: root}
	set := vault.NewSet(offlineEmbedder{}, t.TempDir())
	if err := set.Configure(thoughtsProfiles(cfg)); err != nil {
		t.Fatal(err)
	}
	s := testSettings(t, cfg, nil, nil, nil)
	s.reconfigure = func(*config.Config) error { return errOffline }

	in := thoughtsInput(ThoughtsRow{ID: "work", Path: shared, Orgs: "acme"})
	in.Root = root
	if _, err := s.Save(in); !errors.Is(err, errOffline) {
		t.Fatalf("Save = %v, want the reindex failure", err)
	}
	for _, path := range []string{config.Path(), s.linear.File} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("a refused save wrote %s", path)
		}
	}
	if got := cfg.RouteThoughts([]string{"acme"}).ID; got != config.DefaultThoughtsID {
		t.Fatalf("acme sessions route to %s after a refused save", got)
	}
	if set.ForProfile(vault.Profile{ID: "work", Root: shared}) != nil {
		t.Fatal("a refused save indexed the folder")
	}
}

func TestSessionThoughtsRefusesAFolderWithNoIndexInsideAWiderOne(t *testing.T) {
	base := t.TempDir()
	def, shared := filepath.Join(base, "thoughts"), filepath.Join(base, "thoughts", "shared")
	if err := os.MkdirAll(filepath.Join(shared, "s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	set := vault.NewSet(offlineEmbedder{}, t.TempDir())
	if err := set.Configure([]vault.Profile{{ID: config.DefaultThoughtsID, Root: def}}); err != nil {
		t.Fatal(err)
	}
	roots := []config.ThoughtsRoot{{ID: config.DefaultThoughtsID, Path: def}, {ID: "work", Path: shared, Orgs: []string{"acme"}}}
	sessionDir := t.TempDir()
	if err := os.Symlink(filepath.Join(shared, "s1"), filepath.Join(sessionDir, "thoughts")); err != nil {
		t.Fatal(err)
	}
	if _, err := sessionThoughts(set, roots, sessionDir); !errors.Is(err, ErrNoThoughtsRoot) {
		t.Fatalf("sessionThoughts = %v, want %v", err, ErrNoThoughtsRoot)
	}
}

func fakeGitHubCLI(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if script != "" {
		if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestSettingsGitHubStatusReportsEachCredentialStateAndTheLogin(t *testing.T) {
	cases := []struct {
		name, script, env string
		want              GitHubStatus
	}{
		{"cli", "echo gho_cli", "", GitHubStatus{State: github.CredentialCLI, Login: "gho_cli-user", Variable: "GITHUB_TOKEN"}},
		{"env", "exit 1", "ghp_env", GitHubStatus{State: github.CredentialEnv, Login: "ghp_env-user", Variable: "GITHUB_TOKEN"}},
		{"missing", "", "", GitHubStatus{State: github.CredentialMissing, Variable: "GITHUB_TOKEN"}},
		{"signed out", "exit 1", "", GitHubStatus{State: github.CredentialSignedOut, Variable: "GITHUB_TOKEN"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeGitHubCLI(t, tc.script)
			t.Setenv("GITHUB_TOKEN", tc.env)
			s := testSettings(t, &config.Config{}, nil, nil, nil)
			asked := 0
			s.login = func(_ context.Context, token string) (string, error) {
				asked++
				return token + "-user", nil
			}
			if got := s.GitHubStatus(); got != tc.want {
				t.Fatalf("GitHubStatus() = %+v, want %+v", got, tc.want)
			}
			if signedIn := tc.want.Login != ""; signedIn != (asked == 1) {
				t.Fatalf("login asked %d times for %s", asked, tc.name)
			}
		})
	}
}

func TestSettingsLoadFlagsASharedFolderMissingOnDisk(t *testing.T) {
	present := t.TempDir()
	cfg := &config.Config{Root: t.TempDir(), Thoughts: config.Thoughts{Roots: []config.ThoughtsRoot{
		{ID: "here", Path: present, Orgs: []string{"acme"}},
		{ID: "gone", Path: filepath.Join(present, "not-synced-yet"), Orgs: []string{"club"}},
	}}}
	writeConfigFile(t, cfg)
	roots := testSettings(t, cfg, nil, nil, nil).Load().Thoughts.Roots
	if roots[0].Missing || !roots[1].Missing {
		t.Fatalf("missing flags = %v, %v", roots[0].Missing, roots[1].Missing)
	}
}
