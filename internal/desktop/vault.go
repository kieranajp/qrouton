package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type vaults struct {
	providerCloser       io.Closer
	indexOptions         func() vault.IndexOptions
	downloader           modelDownloader
	installed            func() bool
	download             vaultDownload
	cfg                  *config.Config
	sessions             *Sessions
	mu                   sync.Mutex
	key                  string
	service              *vault.Service
	initializationFailed bool
	closed               bool
}

func newVaults(cfg *config.Config, sessions *Sessions) *vaults {
	return &vaults{cfg: cfg, sessions: sessions}
}
func vaultProfiles(cfg *config.Config) []vault.Profile {
	profiles := make([]vault.Profile, 0, len(cfg.VaultProfiles))
	for _, p := range cfg.VaultProfiles {
		profiles = append(profiles, vault.Profile{ID: p.ID, Name: p.Name, Root: p.Root})
	}
	return profiles
}
func (v *vaults) manager() (*vault.Service, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return nil, vault.ErrUnavailable
	}
	cfg := v.cfg.Snapshot()
	if cfg == nil {
		cfg = &config.Config{}
	}
	encoded, _ := json.Marshal(struct {
		Profiles []config.VaultProfile
		Mappings map[string]string
	}{cfg.VaultProfiles, cfg.VaultMappings})
	if v.service != nil && v.key == string(encoded) {
		return v.service, nil
	}
	if v.service != nil {
		v.stopDownload()
		_ = v.service.Close()
		v.service = nil
	}
	var service *vault.Service
	var err error
	v.initializationFailed = false
	if v.indexOptions != nil && len(cfg.VaultProfiles) > 0 {
		options := v.indexOptions()
		v.initializationFailed = options.Failure != ""
		v.downloader, _ = options.Provider.(modelDownloader)
		service, err = vault.NewIndexedService(vaultProfiles(cfg), cfg.VaultMappings, options)
	} else {
		service, err = vault.NewService(vaultProfiles(cfg), cfg.VaultMappings)
	}
	if err != nil {
		return nil, err
	}
	v.service, v.key = service, string(encoded)
	return service, nil
}
func (v *vaults) close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return
	}
	v.closed = true
	v.stopDownload()
	if v.service != nil {
		_ = v.service.Close()
	}
	if v.providerCloser != nil {
		_ = v.providerCloser.Close()
	}
}
func manifestScope(m session.Manifest) (vault.Scope, error) {
	scope := vault.Scope{}
	attached := []string{}
	for _, repo := range m.Repos {
		attached = append(attached, "github.com/"+repo.Org+"/"+repo.Name)
	}
	scope.Repositories = attached
	if m.Vault != nil {
		if len(m.Vault.Repositories) > 0 {
			for _, repo := range m.Vault.Repositories {
				if !slices.Contains(attached, repo) {
					return scope, vault.ErrScope
				}
			}
			scope.Repositories = slices.Clone(m.Vault.Repositories)
		}
		scope.ReadProfiles = slices.Clone(m.Vault.ReadProfiles)
		scope.Destination = m.Vault.Destination
	}
	return scope, nil
}
func (v *vaults) scope(owner *sessionState) (vault.Scope, error) {
	if owner == nil {
		return vault.Scope{}, ErrNoSession
	}
	manifest, err := session.Load(owner.root())
	if err != nil {
		return vault.Scope{}, err
	}
	return manifestScope(manifest)
}
func (v *vaults) status(owner *sessionState) (vault.Status, error) {
	service, err := v.manager()
	if err != nil {
		return vault.Status{}, err
	}
	scope, err := v.scope(owner)
	if err != nil {
		return vault.Status{}, err
	}
	return service.Status(scope), nil
}
func (v *vaults) read(owner *sessionState, ref vault.Reference) (vault.ReadResult, error) {
	service, err := v.manager()
	if err != nil {
		return vault.ReadResult{}, err
	}
	scope, err := v.scope(owner)
	if err != nil {
		return vault.ReadResult{}, err
	}
	return service.Read(scope, ref)
}
func (v *vaults) search(owner *sessionState, in workbench.VaultSearchRequest) (workbench.VaultSearchResult, error) {
	status, err := v.status(owner)
	if err != nil {
		return workbench.VaultSearchResult{}, err
	}
	if in.Profile != "" && !slices.Contains(status.Scope.Profiles, in.Profile) {
		return workbench.VaultSearchResult{}, vault.ErrScope
	}
	return workbench.VaultSearchResult{Status: status, Outcome: vault.StateUnavailable, Results: []vault.Reference{}}, nil
}

type VaultSessionView struct {
	Session      string                 `json:"session"`
	Workstream   string                 `json:"workstream"`
	Selection    session.VaultSelection `json:"selection"`
	Repositories []string               `json:"repositories"`
	Status       vault.Status           `json:"status"`
}
type VaultSessionInput struct {
	Session    string                 `json:"session"`
	Workstream string                 `json:"workstream"`
	Selection  session.VaultSelection `json:"selection"`
}

func (s *Settings) VaultSession() (VaultSessionView, error) {
	out := VaultSessionView{Repositories: []string{}}
	if s.vaults == nil || s.vaults.sessions.current() == nil {
		return out, nil
	}
	owner := s.vaults.sessions.current()
	m, err := session.Load(owner.root())
	if err != nil {
		return out, err
	}
	out.Session = owner.slug()
	out.Workstream = m.Workstream
	if m.Vault != nil {
		out.Selection = *m.Vault
	}
	for _, repo := range m.Repos {
		out.Repositories = append(out.Repositories, "github.com/"+repo.Org+"/"+repo.Name)
	}
	out.Status, err = s.vaults.status(owner)
	return out, err
}
func (s *Settings) SaveVaultSession(in VaultSessionInput) (VaultSessionView, error) {
	if s.vaults == nil {
		return VaultSessionView{}, ErrNoSession
	}
	owner := s.vaults.sessions.current()
	if owner == nil || owner.slug() != in.Session {
		return VaultSessionView{}, ErrNoSession
	}
	cfg := s.cfg.Snapshot()
	err := session.UpdateManifest(owner.root(), func(m session.Manifest) (session.Manifest, error) {
		next := m
		next.Workstream = strings.TrimSpace(in.Workstream)
		next.Vault = &in.Selection
		scope, err := manifestScope(next)
		if err != nil {
			return m, err
		}
		if _, err = vault.ResolveReadProfiles(vaultProfiles(cfg), cfg.VaultMappings, scope); err != nil {
			return m, err
		}
		if scope.Destination != "" {
			if _, err = vault.ResolveDestination(vaultProfiles(cfg), cfg.VaultMappings, scope); err != nil {
				return m, err
			}
		}
		return next, nil
	})
	if err != nil {
		return VaultSessionView{}, fmt.Errorf(vaultSettingsErrorFormat, err)
	}
	if s.vaultChanged != nil {
		s.vaultChanged()
	}
	return s.VaultSession()
}
func (r *registry) admitVault(owner *sessionState, opts workbench.WindowOptions) (workbench.WindowOptions, error) {
	if opts.Vault == nil {
		return opts, nil
	}
	if r.vaults == nil {
		return opts, vault.ErrUnavailable
	}
	result, err := r.vaults.read(owner, *opts.Vault)
	if err != nil {
		return opts, err
	}
	if len(result.Content) > workbench.DocumentLimit {
		return opts, ErrVaultDocumentLarge
	}
	label := result.Document.Title
	if label == "" {
		label = path.Base(result.Reference.Path)
	}
	label = result.Reference.Profile + vaultLabelSeparator + label
	return workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatMarkdown,
		Vault: &result.Reference, Source: vaultSource(result.Reference), Label: label,
		Content: result.Content, Span: opts.Span, Select: opts.Select}, nil
}
func vaultSource(ref vault.Reference) string {
	identity := ref.ID
	if identity == "" {
		identity = ref.Path
	}
	return vaultSourcePrefix + ref.Profile + "/" + identity
}
func (r *registry) refreshVault(id string) (bool, error) {
	r.vaultPublishMu.Lock()
	defer r.vaultPublishMu.Unlock()
	var owner *sessionState
	var ref *vault.Reference
	var manager *vaults
	var epoch uint64
	if err := r.with(id, func(window *agentWindow) error {
		owner = window.session
		manager = r.vaults
		epoch = r.vaultEpoch
		if window.opts.Vault != nil {
			copy := *window.opts.Vault
			ref = &copy
		}
		return nil
	}); err != nil {
		return false, err
	}
	if ref == nil {
		return false, nil
	}
	var result vault.ReadResult
	var readErr error
	if manager == nil {
		readErr = vault.ErrUnavailable
	} else {
		result, readErr = manager.read(owner, *ref)
	}
	if len(result.Content) > workbench.DocumentLimit {
		readErr = ErrVaultDocumentLarge
	}
	var changed bool
	var doc document
	err := r.with(id, func(window *agentWindow) error {
		if r.vaultEpoch != epoch || window.session != owner || window.opts.Vault == nil || *window.opts.Vault != *ref {
			return vault.ErrScope
		}
		text := result.Content
		if readErr != nil {
			text = ""
		}
		changed = window.opts.Content != text
		window.opts.Content = text
		doc = documentFor(window)
		return nil
	})
	if err != nil {
		return false, err
	}
	if changed {
		r.emit(windowContentEvent+id, doc)
	}
	return changed, readErr
}
func (r *registry) setVaults(manager *vaults) { r.mu.Lock(); defer r.mu.Unlock(); r.vaults = manager }
