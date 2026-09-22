package desktop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/vault"
)

type importFile struct {
	root             *os.Root
	path, name, kind string
}
type importSelections struct {
	mu           sync.Mutex
	files        map[string]importFile
	previews     map[string]map[string]string
	corpus       map[string][]string
	associations map[string]string
	closed       bool
}
type VaultImportFile struct {
	Token   string `json:"token"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Session string `json:"session,omitempty"`
}
type VaultImportInput struct {
	Sources           []string                        `json:"sources"`
	Associations      map[string]string               `json:"associations"`
	Overrides         map[string]vault.ImportOverride `json:"overrides"`
	RepositoryAliases map[string]string               `json:"repositoryAliases"`
	Namespaces        map[string]string               `json:"namespaces"`
	LinkMappings      map[string]string               `json:"linkMappings"`
}

func (i *importSelections) close() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.closed = true
	for _, f := range i.files {
		f.root.Close()
	}
	i.files = nil
	i.previews = nil
}
func readImportFile(f importFile) ([]byte, error) {
	root, name, release, err := importParent(f.root, f.name)
	if err != nil {
		return nil, err
	}
	defer release()
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, vault.ErrImportSource
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return nil, vault.ErrImportSource
	}
	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil || len(data) > 8<<20 {
		return nil, vault.ErrImportSource
	}
	return data, nil
}
func importSession(f importFile, data []byte) (*vault.ImportSession, error) {
	var m session.Manifest
	if json.Unmarshal(data, &m) != nil || m.Slug == "" {
		return nil, vault.ErrImportSource
	}
	result := &vault.ImportSession{Key: vault.SourceSessionKey(f.path), Origin: f.path, Content: data, Slug: m.Slug, Workstream: m.Workstream, Ticket: m.TicketURL}
	if m.Vault != nil {
		result.PublicationDisabled = m.Vault.PublicationDisabled
	}
	for _, repo := range m.Repos {
		var revision *string
		if repo.Revision != "" {
			pin := repo.Revision
			revision = &pin
		}
		result.Repositories = append(result.Repositories, vault.Repository{Path: "github.com/" + repo.Org + "/" + repo.Name, Role: string(repo.Role), Revision: revision})
	}
	return result, nil
}
func (s *Settings) SelectVaultImportSources(kind string) ([]VaultImportFile, error) {
	if s.vaults == nil || s.chooseImportFiles == nil || (kind != importMarkdown && kind != importManifest) {
		return nil, vault.ErrImportSource
	}
	paths, err := s.chooseImportFiles(kind)
	if err != nil {
		return nil, err
	}
	selections := &s.vaults.importSelections
	selections.mu.Lock()
	defer selections.mu.Unlock()
	if selections.closed {
		return nil, vault.ErrUnavailable
	}
	if len(paths)+len(selections.files) > 256 {
		return nil, vault.ErrImportSource
	}
	if selections.files == nil {
		selections.files = map[string]importFile{}
	}
	result := []VaultImportFile{}
	committed := false
	defer func() {
		if !committed {
			for _, view := range result {
				file := selections.files[view.Token]
				file.root.Close()
				delete(selections.files, view.Token)
			}
		}
	}()
	for _, selected := range paths {
		canonical, err := filepath.EvalSymlinks(selected)
		if err != nil || !filepath.IsAbs(canonical) {
			return nil, vault.ErrImportSource
		}
		extension := strings.ToLower(filepath.Ext(canonical))
		if (kind == importMarkdown && extension != ".md" && extension != ".markdown") || (kind == importManifest && extension != ".json") {
			return nil, vault.ErrImportSource
		}
		root, err := os.OpenRoot(filepath.Dir(canonical))
		if err != nil {
			return nil, vault.ErrImportSource
		}
		file := importFile{root: root, path: canonical, name: filepath.Base(canonical), kind: kind}
		data, err := readImportFile(file)
		if err != nil {
			root.Close()
			return nil, err
		}
		view := VaultImportFile{Name: file.name, Kind: kind}
		if kind == importManifest {
			m, err := importSession(file, data)
			if err != nil {
				root.Close()
				return nil, err
			}
			view.Session = m.Slug
		}
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			root.Close()
			return nil, err
		}
		view.Token = hex.EncodeToString(bytes)
		selections.files[view.Token] = file
		result = append(result, view)
	}
	committed = true
	return result, nil
}
func (s *Settings) PreviewVaultImport(in VaultImportInput) (vault.ImportPreview, error) {
	if s.vaults == nil {
		return vault.ImportPreview{}, vault.ErrUnavailable
	}
	service, err := s.vaults.manager()
	if err != nil {
		return vault.ImportPreview{}, err
	}
	selections := &s.vaults.importSelections
	selections.mu.Lock()
	defer selections.mu.Unlock()
	request := vault.ImportRequest{Overrides: in.Overrides, RepositoryAliases: in.RepositoryAliases, Namespaces: in.Namespaces, LinkMappings: in.LinkMappings}
	keys := map[string]string{}
	for _, token := range in.Sources {
		file, exists := selections.files[token]
		if !exists || file.kind != importMarkdown {
			return vault.ImportPreview{}, vault.ErrImportSource
		}
		data, err := readImportFile(file)
		if err != nil {
			return vault.ImportPreview{}, err
		}
		source := vault.ImportSource{Key: token, Name: file.name, Origin: file.path, Content: data}
		keys[token] = token
		if manifestToken := in.Associations[token]; manifestToken != "" {
			manifestFile, exists := selections.files[manifestToken]
			if !exists || manifestFile.kind != importManifest {
				return vault.ImportPreview{}, vault.ErrImportSource
			}
			bytes, err := readImportFile(manifestFile)
			if err != nil {
				return vault.ImportPreview{}, err
			}
			source.Session, err = importSession(manifestFile, bytes)
			if err != nil {
				return vault.ImportPreview{}, err
			}
			keys[source.Session.Key] = manifestToken
		}
		request.Sources = append(request.Sources, source)
	}
	preview, err := service.PreviewImport(s.vaults.ctx, request)
	if err != nil {
		return preview, err
	}
	selections.previews = map[string]map[string]string{preview.ID: keys}
	return preview, nil
}
func (s *Settings) ConfirmVaultImport(preview string, selected []string) (vault.ImportReport, error) {
	if s.vaults == nil {
		return vault.ImportReport{}, vault.ErrUnavailable
	}
	service, err := s.vaults.manager()
	if err != nil {
		return vault.ImportReport{}, err
	}
	selections := &s.vaults.importSelections
	selections.mu.Lock()
	defer selections.mu.Unlock()
	keys, exists := selections.previews[preview]
	if !exists {
		return vault.ImportReport{}, vault.ErrImportSource
	}
	return service.ConfirmImport(s.vaults.ctx, preview, selected, func(key string) ([]byte, error) {
		token := keys[key]
		file, ok := selections.files[token]
		if !ok {
			return nil, vault.ErrImportSource
		}
		return readImportFile(file)
	})
}
func (s *Settings) VaultImportStatus() (vault.ImportReport, error) {
	if s.vaults == nil {
		return vault.ImportReport{Entries: []vault.ImportJob{}}, nil
	}
	service, err := s.vaults.manager()
	if err != nil {
		return vault.ImportReport{}, err
	}
	return service.ImportStatus()
}
func (s *Settings) RetryVaultImports(profile string) error {
	if s.vaults == nil {
		return vault.ErrUnavailable
	}
	service, err := s.vaults.manager()
	if err != nil {
		return err
	}
	return service.RetryImports(profile)
}
func (s *Settings) RevalidateVaultImportDestination(id string) error {
	if s.vaults == nil {
		return vault.ErrUnavailable
	}
	service, err := s.vaults.manager()
	if err != nil {
		return err
	}
	return service.RevalidateImportDestination(s.vaults.ctx, id)
}

func (s *Settings) ClearVaultImportSources() error {
	if s.vaults == nil {
		return nil
	}
	selections := &s.vaults.importSelections
	selections.mu.Lock()
	defer selections.mu.Unlock()
	for _, file := range selections.files {
		file.root.Close()
	}
	selections.files = nil
	selections.previews = nil
	selections.corpus = nil
	selections.associations = nil
	return nil
}
