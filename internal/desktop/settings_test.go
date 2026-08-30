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

	"github.com/kieranajp/qrouton/internal/config"
)

func testSettings(t *testing.T, cfg *config.Config, validateEditor func([]string) error,
	validateLaunch func(map[string][]string) error, quit func()) *Settings {
	t.Helper()
	s := newSettings(cfg, func(string, any) {}, validateEditor, validateLaunch, []string{
		"/Applications/qrouton.app/Contents/MacOS/qrouton",
		"--linear-issue",
	}, []string{"LINEAR_PROMPT"}, quit)
	s.linearFile = filepath.Join(t.TempDir(), "coding-tools.json")
	return s
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

func TestSettingsLoadAnswersEmptyStringForNoLaunchOverrides(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	if got := s.Load().Launch; got != "" {
		t.Fatalf("launch = %q, want empty", got)
	}
}

func TestSettingsLoadPrefillsTheLinearConfigWithThisExecutable(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)

	got := s.Load()
	if got.LinearPath != linearConfigPath {
		t.Fatalf("linear path = %q, want %q", got.LinearPath, linearConfigPath)
	}
	if got.LinearError != "" {
		t.Fatalf("linear error = %q", got.LinearError)
	}
	var document linearConfig
	if err := json.Unmarshal([]byte(got.Linear), &document); err != nil {
		t.Fatalf("linear starter is not valid JSON: %v (%q)", err, got.Linear)
	}
	if document.OpenIssue.Path != s.linearCommand[0] {
		t.Fatalf("linear executable = %q, want %q", document.OpenIssue.Path, s.linearCommand[0])
	}
	wantArgs := []string{s.linearCommand[1], linearIssueTemplate}
	if !reflect.DeepEqual(document.OpenIssue.Args, wantArgs) {
		t.Fatalf("linear args = %#v, want %#v", document.OpenIssue.Args, wantArgs)
	}
	if !reflect.DeepEqual(document.OpenIssue.Env, s.linearEnv) {
		t.Fatalf("linear env = %#v, want %#v", document.OpenIssue.Env, s.linearEnv)
	}
}

func TestSettingsLoadKeepsAnExistingLinearConfigVerbatim(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	existing := "{\n  \"openIssue\": {\"path\": \"/other/tool\"}\n}\n\n"
	if err := os.WriteFile(s.linearFile, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := s.Load().Linear; got != existing {
		t.Fatalf("linear config = %q, want %q", got, existing)
	}
}

func TestSettingsLoadNamesAMissingLinearCommandInsteadOfWritingABrokenStarter(t *testing.T) {
	s := testSettings(t, &config.Config{}, nil, nil, nil)
	s.linearCommand = nil

	got := s.Load()
	if got.Linear != "" || got.LinearError != ErrNoLinearCommand.Error() {
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
	if _, err := os.Stat(s.linearFile); !os.IsNotExist(err) {
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
		_, err := s.Save(SettingsInput{Orgs: []string{"acme"}, Root: t.TempDir(), Linear: raw})
		if err == nil || !strings.HasPrefix(err.Error(), "linear: ") {
			t.Fatalf("Save(%q) error = %v, want a Linear field error", raw, err)
		}
		if _, statErr := os.Stat(config.Path()); !os.IsNotExist(statErr) {
			t.Fatalf("Save(%q) wrote config.json despite refusing Linear", raw)
		}
		if _, statErr := os.Stat(s.linearFile); !os.IsNotExist(statErr) {
			t.Fatalf("Save(%q) wrote coding-tools.json despite refusing Linear", raw)
		}
	}
}

func TestSettingsSaveCreatesTheLinearStarterAndItsDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	s := testSettings(t, &config.Config{Root: root}, nil, nil, nil)
	s.linearFile = filepath.Join(t.TempDir(), ".linear", "coding-tools.json")
	starter := s.Load().Linear

	if _, err := s.Save(SettingsInput{Orgs: []string{"acme"}, Root: root, Linear: starter}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.linearFile)
	if err != nil {
		t.Fatal(err)
	}
	var document linearConfig
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.OpenIssue.Path != s.linearCommand[0] {
		t.Fatalf("linear executable = %q, want %q", document.OpenIssue.Path, s.linearCommand[0])
	}
}

func TestSettingsSaveDoesNotRewriteAnUnchangedLinearConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	s := testSettings(t, &config.Config{Root: root}, nil, nil, nil)
	existing := "{\n  \"custom\": true\n}\n\n"
	if err := os.WriteFile(s.linearFile, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Save(SettingsInput{Orgs: []string{"acme"}, Root: root, Linear: s.Load().Linear}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.linearFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != existing {
		t.Fatalf("unchanged Linear config was rewritten as %q", raw)
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
}

// Save rewrites the whole file, so a marker it does not carry forward re-arms
// first run the moment anyone opens the panel.
func TestSettingsSaveKeepsTheWelcomedMarker(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Root: root, Welcomed: true}
	s := testSettings(t, cfg, nil, nil, nil)

	if _, err := s.Save(SettingsInput{Orgs: []string{"acme"}, Root: root, Linear: `{}`}); err != nil {
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

	result, err := s.Save(SettingsInput{Orgs: []string{"acme"}, Root: "~/work/", Linear: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("retyping the live root with ~ and a trailing slash asked for a restart")
	}

	result, err = s.Save(SettingsInput{Root: liveRoot, Orgs: []string{"acme"}, Linear: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestartRequired {
		t.Fatal("changing only Orgs asked for a restart")
	}

	otherRoot := t.TempDir()
	result, err = s.Save(SettingsInput{Orgs: []string{"acme"}, Root: otherRoot, Linear: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired {
		t.Fatal("a genuinely different root did not ask for a restart")
	}
	if cfg.Root != liveRoot {
		t.Fatalf("live root mutated to %q", cfg.Root)
	}

	result, err = s.Save(SettingsInput{Orgs: []string{"acme"}, Root: liveRoot, Linear: `{}`})
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

	if _, err := s.Save(SettingsInput{Orgs: []string{"acme", "other"}, Root: cfg.Root, Linear: `{}`}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := s.Save(SettingsInput{Orgs: []string{"acme", "other"}, Root: cfg.Root, Linear: `{}`}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	want := [][]string{{"acme", "other"}}
	if !reflect.DeepEqual(announced, want) {
		t.Fatalf("announced = %#v, want %#v — once, for the save that changed them", announced, want)
	}
}
