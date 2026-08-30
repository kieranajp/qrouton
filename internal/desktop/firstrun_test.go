package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/status"
)

// firstRunStubs counts the relaunch and the teardown, and records the order they
// ran in: quitting before a live successor answered would leave no window at all.
type firstRunStubs struct {
	relaunches int
	quits      int
	order      []string
	fail       error
}

func (s *firstRunStubs) relaunch() error {
	s.relaunches++
	s.order = append(s.order, "relaunch")
	return s.fail
}

func (s *firstRunStubs) quit() {
	s.quits++
	s.order = append(s.order, "quit")
}

func savedConfig(t *testing.T) config.Config {
	t.Helper()
	raw, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var saved config.Config
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	return saved
}

func TestFirstRunSaveWithAnUnchangedRootPersistsBothAnswersAndDropsTheGate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Root: root}
	reg := newSessions()
	stubs := &firstRunStubs{}
	f := newFirstRun(cfg, reg, stubs.relaunch, stubs.quit, nil)

	result, err := f.Save(FirstRunInput{Orgs: []string{" acme ", "acme", "", "second-org"}, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Relaunching {
		t.Fatal("an unchanged root relaunched the workbench")
	}
	if stubs.relaunches != 0 || stubs.quits != 0 {
		t.Fatalf("relaunched %d times and quit %d times for an unchanged root", stubs.relaunches, stubs.quits)
	}

	wantOrgs := []string{"acme", "second-org"}
	saved := savedConfig(t)
	if !reflect.DeepEqual(saved.Orgs, wantOrgs) || saved.Root != root || !saved.Welcomed {
		t.Fatalf("saved config = %+v, want both answers and the marker", saved)
	}
	if !reflect.DeepEqual(cfg.Orgs, wantOrgs) || !cfg.Welcomed {
		t.Fatalf("live config = %+v, want the orgs and the marker", cfg)
	}
	if cfg.Root != root {
		t.Fatalf("live root = %q, want the boot value %q", cfg.Root, root)
	}
	select {
	case <-reg.touched:
	default:
		t.Fatal("the chrome poller was not woken, so the overlay stays up for a tick")
	}
}

// Load trims only to decide whether a root was given at all, so untrimmed
// padding on disk becomes a sessions directory with spaces in its name.
func TestFirstRunSaveStoresTheRootWithoutSurroundingSpace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Root: root}
	stubs := &firstRunStubs{}
	f := newFirstRun(cfg, newSessions(), stubs.relaunch, stubs.quit, nil)

	if _, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: "  " + root + "  "}); err != nil {
		t.Fatal(err)
	}
	if saved := savedConfig(t); saved.Root != root {
		t.Fatalf("saved root = %q, want %q", saved.Root, root)
	}
	if stubs.relaunches != 0 {
		t.Fatal("a retyped root with padding was read as a different one")
	}
}

// The rail's scanner and boot path closed over the boot root, so the successor is
// what reaches a new one.
func TestFirstRunSaveWithAChangedRootRelaunchesThenQuits(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Root: t.TempDir()}
	stubs := &firstRunStubs{}
	f := newFirstRun(cfg, newSessions(), stubs.relaunch, stubs.quit, nil)

	next := filepath.Join(t.TempDir(), "elsewhere")
	result, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: next})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Relaunching {
		t.Fatal("a changed root did not report the relaunch")
	}
	if want := []string{"relaunch", "quit"}; !reflect.DeepEqual(stubs.order, want) {
		t.Fatalf("ran %v, want %v", stubs.order, want)
	}
	saved := savedConfig(t)
	if saved.Root != next || !saved.Welcomed || !reflect.DeepEqual(saved.Orgs, []string{"acme"}) {
		t.Fatalf("saved config = %+v, want the new root and the marker", saved)
	}
	if _, err := os.Stat(next); err != nil {
		t.Fatal("the new sessions root was not created:", err)
	}
}

// A relaunch that never came up must leave the gate raised: the old workbench
// cannot carry on into assembly on a root it is no longer configured for.
func TestFirstRunSaveKeepsTheGateUpWhenTheRelaunchFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Root: t.TempDir()}
	stubs := &firstRunStubs{fail: errors.New("workbench never answered")}
	reg := newSessions()
	f := newFirstRun(cfg, reg, stubs.relaunch, stubs.quit, nil)

	_, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: filepath.Join(t.TempDir(), "elsewhere")})
	if err == nil {
		t.Fatal("Save reported success for a relaunch that failed")
	}
	if !errors.Is(err, stubs.fail) {
		t.Fatalf("Save error = %v, want the relaunch's own %v", err, stubs.fail)
	}
	if stubs.quits != 0 {
		t.Fatal("Save quit the workbench with no successor to hand over to")
	}
	if cfg.Welcomed {
		t.Fatal("the live marker was set, so the gate drops with no successor up")
	}

	r := newFakeRenderer()
	pushChrome(reg, cfg.Root, cfg, nil, nil, nil, r.Emit)
	r.mu.Lock()
	defer r.mu.Unlock()
	if fields, ok := r.events[chromeEvent].(status.Fields); !ok || !fields.Welcoming {
		t.Fatalf("chrome = %+v, want first run still raised", fields)
	}
}

// The spawn is unreachable without a completed write, which is what makes a
// successor re-running the flow impossible.
func TestFirstRunSaveRefusesAnUnusableRootAndTouchesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	blocked := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: t.TempDir()}
	stubs := &firstRunStubs{}
	f := newFirstRun(cfg, newSessions(), stubs.relaunch, stubs.quit, nil)

	owned := []string{"acme"}
	for _, in := range []FirstRunInput{
		{Orgs: owned, Root: "   "},
		{Orgs: owned, Root: filepath.Join(blocked, "sub")},
	} {
		if _, err := f.Save(in); err == nil {
			t.Fatalf("Save accepted %q as a sessions root", in.Root)
		}
	}
	if stubs.relaunches != 0 || stubs.quits != 0 {
		t.Fatal("a refused save reached the relaunch")
	}
	if cfg.Welcomed {
		t.Fatal("a refused save marked the config welcomed, so the flow never reappears")
	}
	if _, err := os.Stat(config.Path()); !os.IsNotExist(err) {
		t.Fatal("a refused save wrote config.json")
	}
}

// The binding is callable without the screens, and an owner list saved empty
// would be marked welcomed: no owners, no question to come back to, and a
// session-less workbench has no route to Settings either.
func TestFirstRunSaveRefusesAnEmptyOwnerListAndTouchesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Root: t.TempDir()}
	stubs := &firstRunStubs{}
	f := newFirstRun(cfg, newSessions(), stubs.relaunch, stubs.quit, nil)

	root := filepath.Join(t.TempDir(), "sessions")
	for _, orgs := range [][]string{nil, {}, {"", "   "}} {
		_, err := f.Save(FirstRunInput{Orgs: orgs, Root: root})
		if !errors.Is(err, ErrNoOwners) {
			t.Fatalf("Save(%q) error = %v, want ErrNoOwners", orgs, err)
		}
		if !strings.HasPrefix(err.Error(), "orgs: ") {
			t.Fatalf("refusal %q does not name the field the screen puts it on", err)
		}
	}

	if cfg.Welcomed {
		t.Fatal("a refused save marked the config welcomed, so the flow never reappears")
	}
	if stubs.relaunches != 0 || stubs.quits != 0 {
		t.Fatal("a refused save reached the relaunch")
	}
	if _, err := os.Stat(config.Path()); !os.IsNotExist(err) {
		t.Fatal("a refused save wrote config.json")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("a refused save created the sessions root it was given")
	}
}

// A wrong or absent account is the useful signal, so the screen is given "" to
// say so rather than an error it has nowhere to put.
func TestFirstRunLoginAnswersNothingWithoutCredentials(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	f := newFirstRun(&config.Config{}, newSessions(), nil, nil, nil)

	if login := f.Login(); login != "" {
		t.Fatalf("Login() = %q, want no account named", login)
	}
}

func TestFirstRunChooseRootAnswersThePickerAndNothingOnACancel(t *testing.T) {
	chosen := "/sessions/elsewhere"
	f := newFirstRun(&config.Config{}, newSessions(), nil, nil,
		func() (string, error) { return chosen, nil })
	if got, err := f.ChooseRoot(); err != nil || got != chosen {
		t.Fatalf("ChooseRoot() = %q, %v, want %q", got, err, chosen)
	}

	chosen = ""
	if got, err := f.ChooseRoot(); err != nil || got != "" {
		t.Fatalf("a cancelled picker answered %q, %v, want the field left alone", got, err)
	}
}
