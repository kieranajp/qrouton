package desktop

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/vault"
)

type setupProvider struct {
	pulls   atomic.Int32
	closes  atomic.Int32
	started chan struct{}
	finish  chan struct{}
	failure error
}

func (p *setupProvider) Probe(context.Context) (vault.Fingerprint, error) {
	return vault.Fingerprint{}, vault.ErrModelMissing
}
func (p *setupProvider) Embed(context.Context, vault.Fingerprint, []string) ([][]float32, error) {
	return nil, vault.ErrModelMissing
}
func (p *setupProvider) Close() error { p.closes.Add(1); return nil }
func (p *setupProvider) Pull(ctx context.Context, progress func(vault.PullProgress)) error {
	p.pulls.Add(1)
	progress(vault.PullProgress{Status: "downloading", Total: 10, Completed: 4})
	p.started <- struct{}{}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.finish:
		return p.failure
	}
}
func setupFixture(t *testing.T) (*Settings, *setupProvider) {
	t.Helper()
	_, cfg, v, _, _ := vaultFixture(t)
	provider := &setupProvider{started: make(chan struct{}, 4), finish: make(chan struct{})}
	directory := t.TempDir()
	v.indexOptions = func() vault.IndexOptions {
		return vault.IndexOptions{Provider: provider, InstallationID: "test-machine", StateDir: directory, ReconcileInterval: time.Hour}
	}
	v.providerCloser = provider
	v.installed = func() bool { return true }
	settings := testSettings(t, cfg, nil, nil, nil)
	settings.vaults = v
	return settings, provider
}
func waitSetup(t *testing.T, s *Settings, condition func(VaultSetupView) bool) VaultSetupView {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		view, err := s.VaultSetup()
		if err != nil {
			t.Fatal(err)
		}
		if condition(view) {
			return view
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("setup did not reach expected state")
	return VaultSetupView{}
}
func TestVaultSetupIsGlobalAndDoesNotPull(t *testing.T) {
	settings, p := setupFixture(t)
	view := waitSetup(t, settings, func(v VaultSetupView) bool {
		return len(v.Status.Profiles) == 2 && v.Status.Profiles[0].Index.Dependency == vault.DependencyModelMissing
	})
	if !view.Installed || view.Model != vault.EmbeddingModel || p.pulls.Load() != 0 {
		t.Fatalf("%+v", view)
	}
	scoped, err := settings.vaults.status(settings.vaults.sessions.current())
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.Profiles) != 1 || scoped.Profiles[0].Profile != "shared" {
		t.Fatal("setup expanded session scope")
	}
	if _, err := settings.vaults.read(settings.vaults.sessions.current(), vault.Reference{ID: "example/R1"}); err != nil {
		t.Fatal(err)
	}
	if err := settings.RetryVaultIndex("private"); err != nil {
		t.Fatal(err)
	}
	if err := settings.RetryVaultIndex("unknown"); !errors.Is(err, vault.ErrScope) {
		t.Fatal(err)
	}
}
func TestVaultDownloadExplicitProgressCancelAndClose(t *testing.T) {
	settings, p := setupFixture(t)
	if err := settings.DownloadVaultModel(); err != nil {
		t.Fatal(err)
	}
	<-p.started
	if err := settings.DownloadVaultModel(); err != nil {
		t.Fatal(err)
	}
	view, err := settings.VaultSetup()
	if err != nil {
		t.Fatal(err)
	}
	if view.Download.State != vaultDownloadRunning || view.Download.Progress.Completed != 4 || p.pulls.Load() != 1 {
		t.Fatalf("%+v", view.Download)
	}
	if err := settings.CancelVaultDownload(); err != nil {
		t.Fatal(err)
	}
	waitSetup(t, settings, func(v VaultSetupView) bool { return v.Download.State == vaultDownloadCancelled })
	if err := settings.DownloadVaultModel(); err != nil {
		t.Fatal(err)
	}
	<-p.started
	settings.vaults.close()
	if p.closes.Load() != 1 {
		t.Fatal("provider not closed")
	}
	if err := settings.DownloadVaultModel(); !errors.Is(err, vault.ErrUnavailable) {
		t.Fatal(err)
	}
	settings.vaults.close()
	if p.closes.Load() != 1 {
		t.Fatal("provider closed twice")
	}
}
func TestVaultReplacementJoinsDownload(t *testing.T) {
	settings, p := setupFixture(t)
	if err := settings.DownloadVaultModel(); err != nil {
		t.Fatal(err)
	}
	<-p.started
	next := settings.cfg.Snapshot()
	next.VaultProfiles = next.VaultProfiles[:1]
	next.VaultMappings = map[string]string{"team": "shared"}
	settings.cfg.Replace(next)
	view, err := settings.VaultSetup()
	if err != nil {
		t.Fatal(err)
	}
	if view.Download.State != vaultDownloadCancelled || len(view.Status.Profiles) != 1 {
		t.Fatalf("%+v", view)
	}
	if p.closes.Load() != 0 {
		t.Fatal("provider closed during replacement")
	}
}
func TestVaultDownloadCompletionAndFailure(t *testing.T) {
	for _, failure := range []error{nil, errors.New("private filesystem details")} {
		t.Run(errorText(failure), func(t *testing.T) {
			settings, p := setupFixture(t)
			p.failure = failure
			if err := settings.DownloadVaultModel(); err != nil {
				t.Fatal(err)
			}
			<-p.started
			close(p.finish)
			view := waitSetup(t, settings, func(v VaultSetupView) bool { return v.Download.State != vaultDownloadRunning })
			if failure == nil && view.Download.State != vaultDownloadComplete {
				t.Fatal(view.Download)
			}
			if failure != nil && (view.Download.State != vaultDownloadFailed || view.Download.Error != vault.ErrModelPull.Error()) {
				t.Fatal(view.Download)
			}
		})
	}
}
func TestDisabledVaultSkipsIndexSetup(t *testing.T) {
	v := newVaults(&config.Config{}, newSessions())
	defer v.close()
	v.indexOptions = func() vault.IndexOptions {
		t.Fatal("disabled vault initialized machine state")
		return vault.IndexOptions{}
	}
	settings := &Settings{vaults: v}
	view, err := settings.VaultSetup()
	if err != nil || view.Status.Enabled {
		t.Fatalf("%+v %v", view, err)
	}
	if err := settings.DownloadVaultModel(); !errors.Is(err, vault.ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestVaultStateFailureStaysReadableAndHidesPaths(t *testing.T) {
	settings, _ := setupFixture(t)
	settings.vaults.indexOptions = func() vault.IndexOptions { return vault.IndexOptions{Failure: config.ErrVaultState.Error()} }
	view, err := settings.VaultSetup()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range view.Status.Profiles {
		if profile.Index == nil || profile.Index.Error != config.ErrVaultState.Error() {
			t.Fatalf("%+v", profile)
		}
	}
	if _, err := settings.vaults.read(settings.vaults.sessions.current(), vault.Reference{ID: "example/R1"}); err != nil {
		t.Fatal(err)
	}
}

func TestVaultInitializationFailureRecoversOnExplicitRetry(t *testing.T) {
	settings, provider := setupFixture(t)
	directory := t.TempDir()
	var recovered atomic.Bool
	var calls atomic.Int32
	settings.vaults.indexOptions = func() vault.IndexOptions {
		calls.Add(1)
		if !recovered.Load() {
			return vault.IndexOptions{Failure: config.ErrVaultState.Error()}
		}
		return vault.IndexOptions{Provider: provider, InstallationID: "retry-machine", StateDir: directory, ReconcileInterval: time.Hour}
	}
	view, err := settings.VaultSetup()
	if err != nil {
		t.Fatal(err)
	}
	if view.Status.Profiles[0].Index.Error != config.ErrVaultState.Error() {
		t.Fatal(view.Status)
	}
	if _, err := settings.vaults.read(settings.vaults.sessions.current(), vault.Reference{ID: "example/R1"}); err != nil {
		t.Fatal(err)
	}
	recovered.Store(true)
	if _, err := settings.VaultSetup(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("status implicitly retried initialization")
	}
	if err := settings.RetryVaultIndex("shared"); err != nil {
		t.Fatal(err)
	}
	waitSetup(t, settings, func(view VaultSetupView) bool {
		return view.Status.Profiles[0].Index.Dependency == vault.DependencyModelMissing
	})
	if calls.Load() != 2 {
		t.Fatalf("options factory calls: %d", calls.Load())
	}
	if _, err := settings.vaults.read(settings.vaults.sessions.current(), vault.Reference{ID: "example/R1"}); err != nil {
		t.Fatal(err)
	}
}
