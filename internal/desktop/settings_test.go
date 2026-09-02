package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/lineartools"
)

func testSettings(t *testing.T, cfg *config.Config, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, quit func()) *Settings {
	t.Helper()
	s := newSettings(cfg, func(string, any) {}, validateEditor, validateLaunch, []string{
		"/Applications/qrouton.app/Contents/MacOS/qrouton",
		"--linear-issue",
	}, []string{"LINEAR_PROMPT"}, quit, nil)
	s.linear.File = filepath.Join(t.TempDir(), "coding-tools.json")
	return s
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

func TestSettingsLoadAnswersEffectiveStickerLabels(t *testing.T) {
	defaults := testSettings(t, &config.Config{}, nil, nil, nil).Load()
	if defaults.StickerLabels != config.DefaultStickerLabels {
		t.Fatalf("default sticker labels = %#v, want %#v", defaults.StickerLabels, config.DefaultStickerLabels)
	}

	cfg := &config.Config{StickerLabels: &config.StickerLabels{
		Star: "Priority", Bookmark: "Later", Question: "Clarify", Exclamation: "Broken",
	}}
	custom := testSettings(t, cfg, nil, nil, nil).Load()
	if custom.StickerLabels != *cfg.StickerLabels {
		t.Fatalf("custom sticker labels = %#v, want %#v", custom.StickerLabels, *cfg.StickerLabels)
	}
}

func TestSettingsLoadAnswersEmptyStringForNoLaunchOverrides(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	if got := s.Load().Launch; got != "" {
		t.Fatalf("launch = %q, want empty", got)
	}
}

func TestSettingsLoadCarriesTheLinearDocumentAndWhereItLives(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	document := "{\n  \"openIssue\": {\"path\": \"/other/tool\"}\n}\n"
	if err := os.WriteFile(s.linear.File, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}

	got := s.Load()
	if got.Linear != document {
		t.Fatalf("linear = %q, want %q", got.Linear, document)
	}
	if got.LinearPath != lineartools.ConfigPath {
		t.Fatalf("linear path = %q, want %q", got.LinearPath, lineartools.ConfigPath)
	}
	if got.LinearError != "" {
		t.Fatalf("linear error = %q", got.LinearError)
	}
}

// The panel draws the reason in place of the document, so a Linear file that
// cannot be read or generated has to reach the view as text.
func TestSettingsLoadReportsWhyTheLinearDocumentIsUnavailable(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	s.linear.Command = nil

	got := s.Load()
	if got.Linear != "" || got.LinearError != lineartools.ErrNoCommand.Error() {
		t.Fatalf("linear = %q, error = %q", got.Linear, got.LinearError)
	}
}

// Save refuses Orgs, Root, Editor, Launch, then Linear in that order, and never
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
		_, err := s.Save(SettingsInput{Orgs: orgs, Root: root, Linear: `{}`})
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

func TestSettingsSaveRefusesAnInvalidLinearConfigBeforeWritingEitherFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := testSettings(t, &config.Config{Root: t.TempDir()}, nil, nil, nil)

	for _, raw := range []string{"{not json", "null"} {
		_, err := s.Save(SettingsInput{
			Orgs: []string{"acme"}, Root: t.TempDir(), Linear: raw,
			StickerLabels: config.DefaultStickerLabels,
		})
		if err == nil || !strings.HasPrefix(err.Error(), "linear: ") {
			t.Fatalf("Save(%q) error = %v, want a Linear field error", raw, err)
		}
		if _, statErr := os.Stat(config.Path()); !os.IsNotExist(statErr) {
			t.Fatalf("Save(%q) wrote config.json despite refusing Linear", raw)
		}
		if _, statErr := os.Stat(s.linear.File); !os.IsNotExist(statErr) {
			t.Fatalf("Save(%q) wrote coding-tools.json despite refusing Linear", raw)
		}
	}
}

func TestSettingsSaveRefusesEachBlankStickerLabelBeforeWritingEitherFile(t *testing.T) {
	fields := []struct {
		name  string
		blank func(*config.StickerLabels)
	}{
		{name: "star", blank: func(labels *config.StickerLabels) { labels.Star = " \t" }},
		{name: "bookmark", blank: func(labels *config.StickerLabels) { labels.Bookmark = "" }},
		{name: "question", blank: func(labels *config.StickerLabels) { labels.Question = "\n" }},
		{name: "exclamation", blank: func(labels *config.StickerLabels) { labels.Exclamation = "  " }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
			s := testSettings(t, cfg, nil, nil, nil)
			labels := config.DefaultStickerLabels
			field.blank(&labels)

			_, err := s.Save(SettingsInput{
				Orgs: cfg.Orgs, Root: cfg.Root, Linear: `{}`, StickerLabels: labels,
			})
			if err == nil || !strings.HasPrefix(err.Error(), field.name+": ") {
				t.Fatalf("Save error = %v, want a %s field error", err, field.name)
			}
			if _, statErr := os.Stat(config.Path()); !os.IsNotExist(statErr) {
				t.Fatal("Save wrote config.json despite refusing a sticker label")
			}
			if _, statErr := os.Stat(s.linear.File); !os.IsNotExist(statErr) {
				t.Fatal("Save wrote coding-tools.json despite refusing a sticker label")
			}
		})
	}
}

// Save writes both files, so the Linear document the panel handed back lands on
// disk beside config.json.
func TestSettingsSaveWritesTheLinearDocumentBesideTheConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	s := testSettings(t, &config.Config{Root: root}, nil, nil, nil)
	document := `{"openIssue": {"path": "/bin/true"}}`

	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme"}, Root: root, Linear: document, StickerLabels: config.DefaultStickerLabels,
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.linear.File)
	if err != nil {
		t.Fatal(err)
	}
	if want := document + "\n"; string(raw) != want {
		t.Fatalf("wrote %q, want %q", raw, want)
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
		Orgs:   []string{" acme ", "acme", "", "second-org"},
		Root:   newRoot,
		Editor: "code --wait {}",
		Launch: `{"claude": ["claude", "--verbose"]}`,
		Linear: `{}`,
		StickerLabels: config.StickerLabels{
			Star: " Priority ", Bookmark: " Later ", Question: " Clarify ", Exclamation: " Broken ",
		},
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
	wantLabels := config.StickerLabels{
		Star: "Priority", Bookmark: "Later", Question: "Clarify", Exclamation: "Broken",
	}
	if cfg.StickerLabels == nil || *cfg.StickerLabels != wantLabels {
		t.Fatalf("live sticker labels = %#v, want %#v", cfg.StickerLabels, wantLabels)
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
	if onDisk.StickerLabels == nil || *onDisk.StickerLabels != wantLabels {
		t.Fatalf("on-disk sticker labels = %#v, want %#v", onDisk.StickerLabels, wantLabels)
	}
}

func TestSettingsSaveWakesChromeOnceOnlyWhenNormalisedLabelsChange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Orgs: []string{"acme"}, Root: root}
	s := testSettings(t, cfg, nil, nil, nil)
	wakes := 0
	s.wakeChrome = func() { wakes++ }

	save := func(labels config.StickerLabels) SaveResult {
		t.Helper()
		result, err := s.Save(SettingsInput{
			Orgs: cfg.Orgs, Root: root, Linear: `{}`, StickerLabels: labels,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	if result := save(config.DefaultStickerLabels); result.RestartRequired {
		t.Fatal("persisting effective defaults asked for a restart")
	}
	if wakes != 0 {
		t.Fatalf("persisting unchanged defaults woke chrome %d times", wakes)
	}

	custom := config.StickerLabels{
		Star: "Priority", Bookmark: "Later", Question: "Clarify", Exclamation: "Broken",
	}
	if result := save(custom); result.RestartRequired {
		t.Fatal("changing sticker labels asked for a restart")
	}
	if wakes != 1 {
		t.Fatalf("changing labels woke chrome %d times, want once", wakes)
	}

	padded := config.StickerLabels{
		Star: " Priority ", Bookmark: "\tLater", Question: "Clarify\n", Exclamation: " Broken ",
	}
	save(padded)
	if wakes != 1 {
		t.Fatalf("re-saving normalised labels woke chrome %d times, want once total", wakes)
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

	labels := config.StickerLabels{
		Star: "Priority", Bookmark: "Later", Question: "Clarify", Exclamation: "Broken",
	}
	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme", "other"}, Root: root, Linear: `{}`, StickerLabels: labels,
	}); err == nil {
		t.Fatal("Save succeeded despite an unwritable config path")
	}
	if wakes != 0 {
		t.Fatalf("failed config write woke chrome %d times", wakes)
	}
	if orgEvents != 0 {
		t.Fatalf("failed config write emitted %d org events", orgEvents)
	}
	if cfg.StickerLabels != nil {
		t.Fatalf("failed config write changed live labels to %#v", cfg.StickerLabels)
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
		err := saveConfig(cfg, func(next *config.Config) {
			close(firstEntered)
			<-releaseFirst
			next.Orgs = []string{"first"}
		}, nil, nil)
		errs <- err
	}()
	<-firstEntered
	go func() {
		close(secondStarted)
		err := saveConfig(cfg, func(next *config.Config) {
			next.Orgs = []string{"second"}
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
	s := testSettings(t, cfg, nil, nil, nil)

	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme"}, Root: root, Linear: `{}`, StickerLabels: config.DefaultStickerLabels,
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
		Orgs: []string{"acme"}, Root: "~/work/", Linear: `{}`, StickerLabels: config.DefaultStickerLabels,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("retyping the live root with ~ and a trailing slash asked for a restart")
	}

	result, err = s.Save(SettingsInput{
		Root: liveRoot, Orgs: []string{"acme"}, Linear: `{}`, StickerLabels: config.DefaultStickerLabels,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("changing only Orgs asked for a restart")
	}

	otherRoot := t.TempDir()
	result, err = s.Save(SettingsInput{
		Orgs: []string{"acme"}, Root: otherRoot, Linear: `{}`, StickerLabels: config.DefaultStickerLabels,
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
		Orgs: []string{"acme"}, Root: liveRoot, Linear: `{}`, StickerLabels: config.DefaultStickerLabels,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("typing the live root back did not clear the restart flag")
	}
}

// Settings.Quit must run the exact teardown closing the conversation window
// runs, not a bare renderer Quit: every open session ends and the renderer's
// own Quit is reached only after that teardown completes.
func TestSettingsQuitRunsTheSameTeardownClosingTheWindowRuns(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	quit := sync.OnceFunc(func() {
		windows.stopAll()
		reg.stopAll()
		r.Quit()
	})
	settings := testSettings(t, &config.Config{}, nil, nil, quit)

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

	settings.Quit()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	quitReached := r.quit
	r.mu.Unlock()
	if !quitReached {
		t.Fatal("Settings.Quit did not reach the renderer's own Quit")
	}
	state.mu.Lock()
	stoppedSession := state.stopped
	state.mu.Unlock()
	if !stoppedSession {
		t.Fatal("Settings.Quit did not stop the session registry")
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
		Orgs: []string{"acme", "other"}, Root: cfg.Root, Linear: `{}`, StickerLabels: config.DefaultStickerLabels,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := s.Save(SettingsInput{
		Orgs: []string{"acme", "other"}, Root: cfg.Root, Linear: `{}`, StickerLabels: config.DefaultStickerLabels,
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
	sharedLinear := filepath.Join(t.TempDir(), "coding-tools.json")
	first.linear.File = sharedLinear
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
	second.linear.File = sharedLinear
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
			Orgs: []string{"first"}, Root: root, Linear: `{"generation":"first"}`,
			StickerLabels: config.DefaultStickerLabels,
		})
		firstDone <- err
	}()
	<-firstEmitting
	secondDone := make(chan error, 1)
	go func() {
		_, err := second.Save(SettingsInput{
			Orgs: []string{"second"}, Root: root, Editor: "code", Linear: `{"generation":"second"}`,
			StickerLabels: config.DefaultStickerLabels,
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
	linearBeforeRelease, err := os.ReadFile(sharedLinear)
	if err != nil {
		t.Fatal(err)
	}
	if string(linearBeforeRelease) != "{\"generation\":\"first\"}\n" {
		t.Fatalf("Linear file before first notification completed = %q", linearBeforeRelease)
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
	linear, err := os.ReadFile(sharedLinear)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(live.Orgs, []string{"second"}) || !reflect.DeepEqual(disk.Orgs, live.Orgs) || !reflect.DeepEqual(got[len(got)-1], live.Orgs) {
		t.Fatalf("last event/disk/live orgs = %#v / %#v / %#v", got[len(got)-1], disk.Orgs, live.Orgs)
	}
	if !reflect.DeepEqual(disk.Editor, []string{"code"}) || !reflect.DeepEqual(live.Editor, disk.Editor) || string(linear) != "{\"generation\":\"second\"}\n" {
		t.Fatalf("final disk/live editor and Linear = %#v / %#v / %q", disk.Editor, live.Editor, linear)
	}
}
