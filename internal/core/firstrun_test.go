package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/vault"
)

func TestFirstRunProspectiveThoughtsDefaultUsesCanonicalPathRules(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	f := &FirstRun{}
	for root, want := range map[string]string{"": "", " / ": "/thoughts", " ~/work/../sessions/ ": filepath.Join(home, "sessions", "thoughts")} {
		if got := f.ThoughtsDefault(root); got != want {
			t.Errorf("ThoughtsDefault(%q) = %q, want %q", root, got, want)
		}
	}
}

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
	touched := make(chan struct{}, 1)
	stubs := &firstRunStubs{}
	f := newFirstRun(cfg, func() { touched <- struct{}{} }, stubs.relaunch, stubs.quit, nil, nil)

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
	case <-touched:
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
	f := newFirstRun(cfg, func() {}, stubs.relaunch, stubs.quit, nil, nil)

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
	f := newFirstRun(cfg, func() {}, stubs.relaunch, stubs.quit, nil, nil)

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

func TestFirstRunConfigLoadsAndRoutesSessionsToTheDerivedDefaultRoot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("QROUTON_ROOT", "")
	stubs := &firstRunStubs{}
	f := newFirstRun(&config.Config{Root: t.TempDir()}, func() {}, stubs.relaunch, stubs.quit, nil, nil)
	next := filepath.Join(t.TempDir(), "elsewhere")
	if _, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: next}); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(config.Path()); strings.Contains(string(raw), "thoughts") {
		t.Fatalf("first run pinned a thoughts root:\n%s", raw)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := session.Create(cfg, session.CreateRequest{Name: "Scratch"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(filepath.Join(dir, "thoughts"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(next, "thoughts", "scratch"))
	if real != want {
		t.Fatalf("thoughts resolve to %s, want %s", real, want)
	}
	if _, err := SessionThoughts(OpenVault(cfg), cfg.ThoughtsRoots(), dir); err != nil {
		t.Fatal(err)
	}
}

func TestChangedRootHoldsTheConfigWriterUntilTheSuccessorIsReady(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldRoot := t.TempDir()
	newRoot := filepath.Join(t.TempDir(), "new-root")
	cfg := &config.Config{Orgs: []string{"initial"}, Root: oldRoot}
	relaunchEntered := make(chan struct{})
	successorReady := make(chan struct{})
	firstDone := make(chan error, 1)
	f := newFirstRun(cfg, func() {}, func() error {
		close(relaunchEntered)
		<-successorReady
		return nil
	}, func() {}, nil, nil)
	go func() {
		_, err := f.Save(FirstRunInput{Orgs: []string{"first"}, Root: newRoot})
		firstDone <- err
	}()
	<-relaunchEntered

	if disk := savedConfig(t); disk.Root != newRoot || !disk.Welcomed || !reflect.DeepEqual(disk.Orgs, []string{"first"}) {
		t.Fatalf("disk before successor readiness = %+v", disk)
	}
	secondValidated := make(chan struct{})
	secondDone := make(chan error, 1)
	settings := testSettings(t, cfg, func([]string) error {
		close(secondValidated)
		return nil
	}, nil, nil)
	go func() {
		_, err := settings.Save(SettingsInput{
			Orgs: []string{"second"}, Root: oldRoot, Editor: "code",
			Stickers: defaultStickers(),
		})
		secondDone <- err
	}()
	<-secondValidated
	requireBlocked(t, secondDone, "queued Settings writer")
	if disk := savedConfig(t); disk.Root != newRoot || !disk.Welcomed || !reflect.DeepEqual(disk.Orgs, []string{"first"}) {
		t.Fatalf("queued writer changed disk before successor readiness: %+v", disk)
	}

	close(successorReady)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	live := cfg.Snapshot()
	disk := savedConfig(t)
	if !reflect.DeepEqual(live.Orgs, []string{"second"}) || !reflect.DeepEqual(disk.Orgs, live.Orgs) {
		t.Fatalf("final disk/live orgs = %#v / %#v", disk.Orgs, live.Orgs)
	}
	if disk.Root != oldRoot || live.Root != oldRoot || !disk.Welcomed {
		t.Fatalf("final disk = %+v, live = %+v", disk, live)
	}
}

// A relaunch that never came up must leave the gate raised: the old workbench
// cannot carry on into assembly on a root it is no longer configured for.
func TestFirstRunSaveKeepsTheGateUpWhenTheRelaunchFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Root: t.TempDir()}
	stubs := &firstRunStubs{fail: errors.New("workbench never answered")}
	f := newFirstRun(cfg, func() {}, stubs.relaunch, stubs.quit, nil, nil)

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

	opts, _ := testOptions(t)
	opts.Options = Options{Root: cfg.Root, Config: cfg}
	opts.SessionRoot = ""
	kernel := testWorkbench(t, newFakeClient(), opts)
	startWorkbench(t, kernel, opts)
	waitFor(t, "the chrome to keep first run raised", func() bool { return kernel.Chrome.Snapshot().Welcoming })
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
	f := newFirstRun(cfg, func() {}, stubs.relaunch, stubs.quit, nil, nil)

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
	f := newFirstRun(cfg, func() {}, stubs.relaunch, stubs.quit, nil, nil)

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
	f := newFirstRun(&config.Config{}, func() {}, nil, nil, nil, nil)

	if login := f.Login(); login != "" {
		t.Fatalf("Login() = %q, want no account named", login)
	}
}

// toServer sends every request to server, whatever host it names, so GitHub
// lookups reach a test double and never the network.
type toServer struct{ server *httptest.Server }

func (s toServer) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := url.Parse(s.server.URL)
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = target.Scheme, target.Host
	return s.server.Client().Transport.RoundTrip(req)
}

// gitHubLogins is a GitHub double that names octocat for good-token and refuses
// every other token, counting the lookups.
func gitHubLogins(t *testing.T) (*http.Client, *atomic.Int32) {
	t.Helper()
	var lookups atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lookups.Add(1)
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	}))
	t.Cleanup(server.Close)
	return &http.Client{Transport: toServer{server}}, &lookups
}

// Catches Login answering anything but the account GitHub names for the
// resolved token, or naming one after GitHub refused the token.
func TestFirstRunLoginNamesTheTokensAccountAndNothingWhenTheLookupFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	client, lookups := gitHubLogins(t)
	f := newFirstRun(&config.Config{}, func() {}, nil, nil, nil, nil)
	f.githubAPI = client

	t.Setenv("GITHUB_TOKEN", "good-token")
	if got := f.Login(); got != "octocat" {
		t.Fatalf("Login() = %q, want octocat", got)
	}
	t.Setenv("GITHUB_TOKEN", "revoked-token")
	if got := f.Login(); got != "" || lookups.Load() != 2 {
		t.Fatalf("Login() after a refused lookup = %q after %d lookups, want no account named", got, lookups.Load())
	}
}

// Catches Login asking GitHub about an empty token when none resolves.
func TestFirstRunLoginAsksNothingWhenNoTokenResolves(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	client, lookups := gitHubLogins(t)
	f := newFirstRun(&config.Config{}, func() {}, nil, nil, nil, nil)
	f.githubAPI = client
	if got := f.Login(); got != "" || lookups.Load() != 0 {
		t.Fatalf("Login() with no token = %q after %d lookups", got, lookups.Load())
	}
}

func TestFirstRunChooseRootAnswersThePickerAndNothingOnACancel(t *testing.T) {
	chosen := "/sessions/elsewhere"
	f := newFirstRun(&config.Config{}, func() {}, nil, nil,
		func() (string, error) { return chosen, nil }, nil)
	if got, err := f.ChooseRoot(); err != nil || got != chosen {
		t.Fatalf("ChooseRoot() = %q, %v, want %q", got, err, chosen)
	}

	chosen = ""
	if got, err := f.ChooseRoot(); err != nil || got != "" {
		t.Fatalf("a cancelled picker answered %q, %v, want the field left alone", got, err)
	}
}

func TestFirstRunStoresTheThoughtsFolderOnlyWhenItDiffersFromTheDerivedOne(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	for _, answer := range []string{"", "  ", filepath.Join(root, "thoughts"), filepath.Join(root, "thoughts") + "/"} {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		f := newFirstRun(&config.Config{Root: root}, func() {}, nil, nil, nil, nil)
		if _, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: root, Thoughts: answer}); err != nil {
			t.Fatal(err)
		}
		if raw, _ := os.ReadFile(config.Path()); strings.Contains(string(raw), "thoughts") {
			t.Fatalf("answer %q pinned a thoughts root:\n%s", answer, raw)
		}
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Root: root}
	var reconfigured []config.ThoughtsRoot
	f := newFirstRun(cfg, func() {}, nil, nil, nil, func(c *config.Config) error { reconfigured = c.ThoughtsRoots(); return nil })
	if _, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: root, Thoughts: " ~/Vault "}); err != nil {
		t.Fatal(err)
	}
	if saved := savedConfig(t); saved.Thoughts.Default != "~/Vault" {
		t.Fatalf("saved default = %q, want ~ kept", saved.Thoughts.Default)
	}
	want := filepath.Join(home, "Vault")
	if got := cfg.ThoughtsRoots()[0].Path; got != want {
		t.Fatalf("live default = %q, want %q", got, want)
	}
	if len(reconfigured) == 0 || reconfigured[0].Path != want {
		t.Fatalf("search was reconfigured with %v", reconfigured)
	}
}

func TestFirstRunRefusesAThoughtsFolderHoldingTheRootAndWritesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	parent := t.TempDir()
	root := filepath.Join(parent, "sessions")
	cfg := &config.Config{Root: root}
	f := newFirstRun(cfg, func() {}, nil, nil, nil, nil)

	_, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: root, Thoughts: parent})
	if !errors.Is(err, config.ErrThoughtsHoldsRoot) || !strings.HasPrefix(err.Error(), "thoughts: ") {
		t.Fatalf("Save = %v, want a thoughts field refusal", err)
	}
	if _, statErr := os.Stat(config.Path()); !os.IsNotExist(statErr) {
		t.Fatal("a refused save wrote config.json")
	}
	if cfg.Welcomed {
		t.Fatal("a refused save marked the config welcomed")
	}
}

func TestFirstRunSaveUnderOverridesWritesTheFileAsTypedAndKeepsTheOverridesLive(t *testing.T) {
	_, cfg, override := overriddenSettings(t)
	f := newFirstRun(cfg, func() {}, nil, nil, nil, nil)

	result, err := f.Save(FirstRunInput{Orgs: []string{"acme", "typed-org"}, Root: "~/work"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Relaunching {
		t.Fatal("an unchanged root relaunched under QROUTON_ROOT")
	}
	onDisk := savedConfig(t)
	if onDisk.Root != "~/work" || onDisk.Thoughts.Roots[0].Path != "~/shared" || !onDisk.Welcomed {
		t.Fatalf("on disk = root %q, thoughts %+v, welcomed %v", onDisk.Root, onDisk.Thoughts, onDisk.Welcomed)
	}
	if !reflect.DeepEqual(onDisk.Orgs, []string{"acme", "typed-org"}) {
		t.Fatalf("on disk orgs = %v, want the typed ones", onDisk.Orgs)
	}
	live := cfg.Snapshot()
	if live.Root != override || !reflect.DeepEqual(live.Orgs, []string{"pinned-org"}) || !live.Welcomed {
		t.Fatalf("live = root %q, orgs %v, welcomed %v", live.Root, live.Orgs, live.Welcomed)
	}
}

func TestFirstRunSaveUnderOverridesRefusesAThoughtsFolderHoldingTheOverrideRoot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	vault := t.TempDir()
	override := filepath.Join(vault, "sessions")
	t.Setenv("QROUTON_ROOT", override)
	fileRoot := filepath.Join(t.TempDir(), "work")
	if err := os.MkdirAll(filepath.Dir(config.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	file := `{"orgs": ["acme"], "root": "` + fileRoot + `"}`
	if err := os.WriteFile(config.Path(), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	before := cfg.Snapshot()
	f := newFirstRun(cfg, func() {}, nil, nil, nil, nil)

	_, err = f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: fileRoot, Thoughts: vault})
	if !errors.Is(err, config.ErrThoughtsHoldsRoot) || !strings.HasPrefix(err.Error(), "thoughts: ") {
		t.Fatalf("Save = %v, want a thoughts field refusal", err)
	}
	disk, readErr := os.ReadFile(config.Path())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(disk) != file || savedConfig(t).Welcomed {
		t.Fatalf("a refused save changed config.json: %s", disk)
	}
	if !reflect.DeepEqual(cfg.Snapshot(), before) {
		t.Fatalf("a refused save changed the live config: %+v", cfg.Snapshot())
	}
}

// Catches a welcome that moves the thoughts folder but leaves search on the old index.
func TestFirstRunThatMovesTheThoughtsFolderReindexesSoASessionSearchesTheNewOne(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("QROUTON_ROOT", "")
	root, moved := t.TempDir(), t.TempDir()
	for path, text := range map[string]string{
		filepath.Join(root, "thoughts", "s1", "old.md"): "# Old\n\nNothing here.\n",
		filepath.Join(moved, "s1", "note.md"):           "# Wombats\n\nWombats dig burrows.\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{Root: root}
	set := vault.NewSet(offlineEmbedder{}, t.TempDir())
	if err := set.Configure(ThoughtsProfiles(cfg)); err != nil {
		t.Fatal(err)
	}
	f := newFirstRun(cfg, func() {}, nil, nil, nil, ReconfigureVault(set))
	if res, err := f.Save(FirstRunInput{Orgs: []string{"acme"}, Root: root, Thoughts: moved}); err != nil || res.Relaunching {
		t.Fatalf("Save = %+v, %v", res, err)
	}

	sessionDir := t.TempDir()
	if err := os.Symlink(filepath.Join(moved, "s1"), filepath.Join(sessionDir, "thoughts")); err != nil {
		t.Fatal(err)
	}
	thoughts, err := SessionThoughts(set, cfg.ThoughtsRoots(), sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := thoughts.Search(context.Background(), vault.Query{Text: "wombat burrows"})
	if err != nil || len(res.Hits) != 1 || res.Hits[0].Ref != config.DefaultThoughtsID+":s1/note.md" {
		t.Fatalf("search = %+v, %v", res, err)
	}
}

func TestFirstRunRelaunchCarriesOnlyAnUnclaimedSeed(t *testing.T) {
	kernel := testWorkbench(t, newFakeClient(), workbenchOptions{Options: Options{Config: &config.Config{Root: t.TempDir()}}})
	if _, err := kernel.Offer("LIF-2841", "Linear prompt"); err != nil {
		t.Fatal(err)
	}
	var carried, carriedPrompt string
	relaunch := pendingRelaunch(func(request func() (string, string)) error {
		carried, carriedPrompt = request()
		return nil
	}, kernel.PendingTicket)
	if err := relaunch(); err != nil || carried != "https://linear.app/issue/LIF-2841" || carriedPrompt != "Linear prompt" {
		t.Fatalf("unclaimed relaunch = %q, %q, %v", carried, carriedPrompt, err)
	}
	kernel.Assembly.Begin()
	carried = "not called"
	carriedPrompt = "not called"
	if err := relaunch(); err != nil || carried != "" {
		t.Fatalf("claimed relaunch = %q, %q, %v", carried, carriedPrompt, err)
	}
}

func TestFirstRunRelaunchReadsAPendingSeedAfterItOwnsTheHandoff(t *testing.T) {
	kernel := testWorkbench(t, newFakeClient(), workbenchOptions{Options: Options{Config: &config.Config{Root: t.TempDir()}}})
	var carried string
	relaunch := pendingRelaunch(func(request func() (string, string)) error {
		if _, err := kernel.Offer("LIF-2841", "Late prompt"); err != nil {
			return err
		}
		carried, _ = request()
		return nil
	}, kernel.PendingTicket)
	if err := relaunch(); err != nil {
		t.Fatal(err)
	}
	if carried != "https://linear.app/issue/LIF-2841" {
		t.Fatalf("late pending seed = %q", carried)
	}
}

// Catches a settings restart carrying the pending ticket into the successor,
// which would open it a second time there, or first run dropping it.
func TestASettingsRestartRelaunchesWithoutATicket(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var carried []string
	kernel := testWorkbench(t, newFakeClient(), workbenchOptions{Options: Options{
		Config: &config.Config{Root: t.TempDir()},
		Relaunch: func(pending func() (string, string)) error {
			if pending == nil {
				carried = append(carried, "")
				return nil
			}
			url, _ := pending()
			carried = append(carried, url)
			return nil
		},
		Quit: func() {},
	}})
	if _, err := kernel.Offer("LIF-2841", ""); err != nil {
		t.Fatal(err)
	}
	if err := kernel.Settings.Relaunch(); err != nil {
		t.Fatal(err)
	}
	if err := kernel.FirstRun.relaunch(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"", "https://linear.app/issue/LIF-2841"}; !reflect.DeepEqual(carried, want) {
		t.Fatalf("settings then first run carried %q, want %q", carried, want)
	}
}

// Catches a core with no relaunch handing settings or first run one that panics.
func TestACoreWithNoRelaunchRefusesToRestart(t *testing.T) {
	kernel := testWorkbench(t, newFakeClient(), workbenchOptions{Options: Options{
		Config: &config.Config{Root: t.TempDir()}, Quit: func() { t.Fatal("quit with no successor") }}})
	if err := kernel.Settings.Relaunch(); !errors.Is(err, ErrNoRelaunch) {
		t.Fatalf("Settings.Relaunch = %v, want ErrNoRelaunch", err)
	}
	if kernel.FirstRun.relaunch != nil {
		t.Fatal("first run was given a relaunch with none to give")
	}
}
