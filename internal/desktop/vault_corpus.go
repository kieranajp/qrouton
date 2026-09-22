package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/vault"
)

type VaultCorpus struct {
	Token             string            `json:"token"`
	Name              string            `json:"name"`
	Documents         int               `json:"documents"`
	Skipped           int               `json:"skipped"`
	Sessions          []string          `json:"sessions"`
	Author            string            `json:"author"`
	RepositoryAliases map[string]string `json:"repositoryAliases"`
}
type VaultCorpusInput struct {
	Token               string                          `json:"token"`
	TargetProfile       string                          `json:"targetProfile"`
	Defaults            vault.CorpusDefaults            `json:"defaults"`
	RepositoryAliases   map[string]string               `json:"repositoryAliases"`
	SessionRepositories map[string][]vault.Repository   `json:"sessionRepositories"`
	Overrides           map[string]vault.ImportOverride `json:"overrides"`
	Namespaces          map[string]string               `json:"namespaces"`
	LinkMappings        map[string]string               `json:"linkMappings"`
}

func importParent(root *os.Root, name string) (*os.Root, string, func(), error) {
	parts := strings.Split(filepath.ToSlash(name), "/")
	var opened []*os.Root
	release := func() {
		for _, r := range opened {
			r.Close()
		}
	}
	for _, part := range parts[:len(parts)-1] {
		if part == "" || part == "." || part == ".." {
			release()
			return nil, "", func() {}, vault.ErrImportSource
		}
		handle, err := root.OpenFile(part, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
		if err != nil {
			release()
			return nil, "", func() {}, vault.ErrImportSource
		}
		info, err := handle.Stat()
		handle.Close()
		if err != nil || !info.IsDir() {
			release()
			return nil, "", func() {}, vault.ErrImportSource
		}
		child, err := root.OpenRoot(part)
		if err != nil {
			release()
			return nil, "", func() {}, vault.ErrImportSource
		}
		opened = append(opened, child)
		actual, err := child.Stat(".")
		if err != nil || !os.SameFile(info, actual) {
			release()
			return nil, "", func() {}, vault.ErrImportSource
		}
		root = child
	}
	return root, parts[len(parts)-1], release, nil
}
func importToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func (s *Settings) SelectVaultImportCorpus() (VaultCorpus, error) {
	out := VaultCorpus{Sessions: []string{}}
	if s.vaults == nil || s.chooseImportCorpus == nil {
		return out, vault.ErrImportSource
	}
	selected, err := s.chooseImportCorpus()
	if err != nil || selected == "" {
		return out, err
	}
	canonical, err := filepath.EvalSymlinks(selected)
	if err != nil || !filepath.IsAbs(canonical) {
		return out, vault.ErrImportSource
	}
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return out, vault.ErrImportSource
	}
	retained := false
	defer func() {
		if !retained {
			root.Close()
		}
	}()
	files := map[string]importFile{}
	var tokens []string
	manifests := map[string]string{}
	sessions := map[string]bool{}
	var walk func(string, int) error
	walk = func(relative string, depth int) error {
		if depth > 64 {
			return vault.ErrImportSource
		}
		dir, _, release, err := importParent(root, filepath.Join(relative, "__directory__"))
		if err != nil {
			return err
		}
		defer release()
		f, err := dir.Open(".")
		if err != nil {
			return vault.ErrImportSource
		}
		entries, err := f.ReadDir(10001)
		f.Close()
		if (err != nil && err != io.EOF) || len(entries) > 10000 {
			return vault.ErrImportSource
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			path := filepath.Join(relative, entry.Name())
			if entry.Type()&os.ModeSymlink != 0 {
				out.Skipped++
				continue
			}
			if entry.IsDir() {
				if err := walk(path, depth+1); err != nil {
					return err
				}
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return vault.ErrImportSource
			}
			if !info.Mode().IsRegular() {
				out.Skipped++
				continue
			}
			ext := strings.ToLower(filepath.Ext(path))
			kind := importMarkdown
			if entry.Name() == "qrouton.json" {
				kind = importManifest
			} else if ext != ".md" && ext != ".markdown" {
				out.Skipped++
				continue
			}
			if len(files) >= 8192 || len(tokens) >= 4096 {
				return vault.ErrImportSource
			}
			token, err := importToken()
			if err != nil {
				return err
			}
			file := importFile{root: root, path: filepath.Join(canonical, path), name: path, kind: kind}
			if _, err = readImportFile(file); err != nil {
				return err
			}
			files[token] = file
			if kind == importMarkdown {
				tokens = append(tokens, token)
				parts := strings.Split(filepath.ToSlash(path), "/")
				if len(parts) >= 4 && parts[1] == "shared" {
					sessions[parts[0]] = true
				}
			} else {
				manifests[filepath.Dir(path)] = token
			}
		}
		return nil
	}
	if err = walk("", 0); err != nil {
		return out, err
	}
	associations := map[string]string{}
	var evidence *os.Root
	evidenceUsed := false
	cfg := s.cfg.Snapshot()
	if cfg != nil && cfg.Root != "" {
		evidence, _ = os.OpenRoot(cfg.Root)
	}
	defer func() {
		if evidence != nil && (!retained || !evidenceUsed) {
			evidence.Close()
		}
	}()
	for slug := range sessions {
		out.Sessions = append(out.Sessions, slug)
		if manifests[slug] != "" {
			continue
		}
		if evidence == nil {
			continue
		}
		file := importFile{root: evidence, path: filepath.Join(cfg.Root, slug, "qrouton.json"), name: filepath.Join(slug, "qrouton.json"), kind: importManifest}
		data, err := readImportFile(file)
		if err != nil {
			continue
		}
		manifest, err := importSession(file, data)
		if err != nil || manifest.Slug != slug {
			continue
		}
		token, err := importToken()
		if err != nil {
			return out, err
		}
		files[token] = file
		manifests[slug] = token
		evidenceUsed = true
	}
	sort.Strings(out.Sessions)
	out.RepositoryAliases = corpusAliases(s.vaults.ctx, cfg)
	for _, token := range tokens {
		path := files[token].name
		slug := strings.Split(filepath.ToSlash(path), "/")[0]
		if m := manifests[slug]; m != "" {
			associations[token] = m
		}
	}
	out.Token, err = importToken()
	if err != nil {
		return out, err
	}
	out.Name = filepath.Base(canonical)
	out.Documents = len(tokens)
	selections := &s.vaults.importSelections
	selections.mu.Lock()
	defer selections.mu.Unlock()
	if selections.closed {
		return out, vault.ErrUnavailable
	}
	for _, file := range selections.files {
		file.root.Close()
	}
	selections.files = files
	selections.previews = nil
	selections.corpus = map[string][]string{out.Token: tokens}
	selections.associations = associations
	retained = true
	if s.importAuthor != nil {
		out.Author = s.importAuthor()
	}
	return out, nil
}
func (s *Settings) PreviewVaultCorpus(in VaultCorpusInput) (vault.CorpusPreview, error) {
	if s.vaults == nil {
		return vault.CorpusPreview{}, vault.ErrUnavailable
	}
	service, err := s.vaults.manager()
	if err != nil {
		return vault.CorpusPreview{}, err
	}
	selections := &s.vaults.importSelections
	selections.mu.Lock()
	defer selections.mu.Unlock()
	tokens, ok := selections.corpus[in.Token]
	if !ok {
		return vault.CorpusPreview{}, vault.ErrImportSource
	}
	request := vault.CorpusRequest{TargetProfile: in.TargetProfile, Defaults: in.Defaults, RepositoryAliases: in.RepositoryAliases, SessionRepositories: in.SessionRepositories, Overrides: in.Overrides, Namespaces: in.Namespaces, LinkMappings: in.LinkMappings}
	keys := map[string]string{}
	totalBytes := 0
	manifests := map[string]*vault.ImportSession{}
	for _, token := range tokens {
		file := selections.files[token]
		data, err := readImportFile(file)
		if err != nil {
			return vault.CorpusPreview{}, err
		}
		totalBytes += len(data)
		if totalBytes > 64<<20 {
			return vault.CorpusPreview{}, vault.ErrImportSource
		}
		source := vault.ImportSource{Key: token, Name: filepath.Base(file.name), Origin: file.path, Content: data}
		keys[token] = token
		if mt := selections.associations[token]; mt != "" {
			manifest := manifests[mt]
			if manifest == nil {
				mf := selections.files[mt]
				data, err := readImportFile(mf)
				if err != nil {
					return vault.CorpusPreview{}, err
				}
				totalBytes += len(data)
				if totalBytes > 64<<20 {
					return vault.CorpusPreview{}, vault.ErrImportSource
				}
				manifest, err = importSession(mf, data)
				if err != nil {
					return vault.CorpusPreview{}, err
				}
				manifests[mt] = manifest
			}
			source.Session = manifest
			keys[manifest.Key] = mt
		}
		request.Sources = append(request.Sources, vault.CorpusSource{Source: source, RelativePath: filepath.ToSlash(file.name)})
	}
	cfg := s.cfg.Snapshot()
	request.ResolveRevision = func(repository, recorded string) (string, error) {
		if cfg == nil || !vault.ValidRepository(repository) || !regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`).MatchString(recorded) {
			return "", vault.ErrImportSource
		}
		parts := strings.Split(repository, "/")
		mirror := filepath.Join(cfg.Root, ".mirrors", parts[1], parts[2]+".git")
		ctx, cancel := context.WithTimeout(s.vaults.ctx, time.Second)
		defer cancel()
		remote, err := exec.CommandContext(ctx, "git", "--git-dir", mirror, "config", "--get", "remote.origin.url").Output()
		if err != nil {
			return "", vault.ErrImportSource
		}
		identity := strings.TrimSuffix(strings.TrimSpace(string(remote)), ".git")
		identity = strings.TrimPrefix(identity, "https://")
		identity = strings.Replace(identity, "git@github.com:", "github.com/", 1)
		if !strings.EqualFold(identity, repository) {
			return "", vault.ErrImportSource
		}
		result, err := exec.CommandContext(ctx, "git", "--git-dir", mirror, "rev-parse", "--verify", recorded+"^{commit}").Output()
		return strings.TrimSpace(string(result)), err
	}
	result, err := service.PreviewCorpus(s.vaults.ctx, request)
	if err == nil {
		selections.previews = map[string]map[string]string{result.ID: keys}
	}
	return result, err
}
func (s *Settings) VaultCorpusEntry(preview, key string) (vault.ImportEntry, error) {
	if s.vaults == nil {
		return vault.ImportEntry{}, vault.ErrUnavailable
	}
	service, err := s.vaults.manager()
	if err != nil {
		return vault.ImportEntry{}, err
	}
	selections := &s.vaults.importSelections
	selections.mu.Lock()
	defer selections.mu.Unlock()
	if _, ok := selections.previews[preview][key]; !ok {
		return vault.ImportEntry{}, vault.ErrImportSource
	}
	return service.CorpusEntry(preview, key)
}

func localImportAuthor() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if data, err := exec.CommandContext(ctx, "git", "config", "--global", "--get", "user.name").Output(); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func corpusAliases(ctx context.Context, cfg *config.Config) map[string]string {
	aliases := map[string]string{}
	if cfg == nil || cfg.Root == "" {
		return aliases
	}
	candidates := map[string][]string{}
	orgs := map[string]bool{}
	for org := range cfg.VaultMappings {
		orgs[org] = true
	}
	for _, org := range cfg.Orgs {
		orgs[org] = true
	}
	if entries, err := os.ReadDir(filepath.Join(cfg.Root, ".mirrors")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				orgs[entry.Name()] = true
			}
		}
	}
	for org := range orgs {
		if strings.ContainsAny(org, "/\\") || org == "." || org == ".." {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(cfg.Root, ".mirrors", org))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".git") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".git")
			identity := "github.com/" + org + "/" + name
			if !vault.ValidRepository(identity) {
				continue
			}
			bounded, cancel := context.WithTimeout(ctx, time.Second)
			data, err := exec.CommandContext(bounded, "git", "--git-dir", filepath.Join(cfg.Root, ".mirrors", org, entry.Name()), "config", "--get", "remote.origin.url").Output()
			cancel()
			remote := strings.TrimSuffix(strings.TrimSpace(string(data)), ".git")
			remote = strings.TrimPrefix(remote, "https://")
			remote = strings.Replace(remote, "git@github.com:", "github.com/", 1)
			if err == nil && strings.EqualFold(remote, identity) {
				candidates[name] = append(candidates[name], identity)
				aliases[org+"/"+name] = identity
			}
		}
	}
	for name, identities := range candidates {
		if len(identities) == 1 {
			aliases[name] = identities[0]
		}
	}
	return aliases
}
