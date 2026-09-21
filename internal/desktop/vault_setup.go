package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/vault"
)

type modelDownloader interface {
	Pull(context.Context, func(vault.PullProgress)) error
}
type vaultDownload struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	view   VaultDownloadView
}
type VaultDownloadView struct {
	State    string             `json:"state"`
	Progress vault.PullProgress `json:"progress"`
	Error    string             `json:"error,omitempty"`
}
type VaultSetupView struct {
	Status    vault.Status      `json:"status"`
	Download  VaultDownloadView `json:"download"`
	Installed bool              `json:"installed"`
	Model     string            `json:"model"`
}

func productionVaultOptions(provider vault.Provider) vault.IndexOptions {
	options := vault.IndexOptions{Provider: provider}
	state, err := config.VaultState()
	if err != nil {
		options.Failure = config.ErrVaultState.Error()
		return options
	}
	options.InstallationID, options.StateDir = state.InstallationID, state.Directory
	return options
}
func ollamaInstalled() bool {
	if _, err := exec.LookPath(ollamaCommand); err == nil {
		return true
	}
	info, err := os.Stat(ollamaApplication)
	return err == nil && info.IsDir()
}
func (v *vaults) stopDownload() {
	v.download.mu.Lock()
	cancel, done := v.download.cancel, v.download.done
	v.download.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
func (s *Settings) VaultSetup() (VaultSetupView, error) {
	out := VaultSetupView{Model: vault.EmbeddingModel, Status: vault.Status{Profiles: []vault.ProfileStatus{}}}
	if s.vaults == nil {
		return out, nil
	}
	v := s.vaults
	service, err := v.manager()
	if err != nil {
		return out, err
	}
	v.mu.Lock()
	if v.closed || v.service != service {
		v.mu.Unlock()
		return out, vault.ErrUnavailable
	}
	cfg := v.cfg.Snapshot()
	scope := vault.Scope{}
	for _, profile := range cfg.VaultProfiles {
		scope.ReadProfiles = append(scope.ReadProfiles, profile.ID)
	}
	v.download.mu.Lock()
	out.Download = v.download.view
	v.download.mu.Unlock()
	v.mu.Unlock()
	out.Status = service.Status(scope)
	v.mu.Lock()
	current := !v.closed && v.service == service
	v.mu.Unlock()
	if !current {
		return VaultSetupView{}, vault.ErrUnavailable
	}
	if out.Status.Enabled && v.installed != nil {
		out.Installed = v.installed()
	}
	return out, nil
}
func (s *Settings) DownloadVaultModel() error {
	if s.vaults == nil {
		return vault.ErrUnavailable
	}
	v := s.vaults
	service, err := v.manager()
	if err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed || v.service != service || v.downloader == nil || len(v.cfg.Snapshot().VaultProfiles) == 0 {
		return vault.ErrUnavailable
	}
	v.download.mu.Lock()
	if v.download.view.State == vaultDownloadRunning {
		v.download.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	v.download.cancel, v.download.done = cancel, done
	v.download.view = VaultDownloadView{State: vaultDownloadRunning}
	v.download.mu.Unlock()
	profiles := v.cfg.Snapshot().VaultProfiles
	downloader := v.downloader
	go func() {
		defer close(done)
		err := downloader.Pull(ctx, func(progress vault.PullProgress) {
			v.download.mu.Lock()
			v.download.view.Progress = progress
			v.download.mu.Unlock()
		})
		state, message := vaultDownloadComplete, ""
		if errors.Is(err, context.Canceled) {
			state = vaultDownloadCancelled
		} else if err != nil {
			state, message = vaultDownloadFailed, vault.ErrModelPull.Error()
		}
		if err == nil {
			for _, profile := range profiles {
				_ = service.Retry(profile.ID)
			}
		}
		v.download.mu.Lock()
		v.download.view.State, v.download.view.Error = state, message
		v.download.mu.Unlock()
	}()
	return nil
}
func (s *Settings) CancelVaultDownload() error {
	if s.vaults == nil {
		return nil
	}
	v := s.vaults
	v.mu.Lock()
	defer v.mu.Unlock()
	v.stopDownload()
	return nil
}
func (s *Settings) RetryVaultIndex(profile string) error {
	if s.vaults == nil {
		return vault.ErrUnavailable
	}
	v := s.vaults
	v.mu.Lock()
	if v.initializationFailed {
		v.key = ""
	}
	v.mu.Unlock()
	service, err := v.manager()
	if err != nil {
		return err
	}
	return service.Retry(profile)
}
