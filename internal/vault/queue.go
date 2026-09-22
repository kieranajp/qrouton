package vault

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ImportJob struct {
	ID         string `json:"id"`
	ArtifactID string `json:"artifactId"`
	Profile    string `json:"profile"`
	SessionKey string `json:"sessionKey,omitempty"`
	State      string `json:"state"`
	Error      string `json:"error,omitempty"`
}
type ImportReport struct {
	Entries []ImportJob `json:"entries"`
}
type importDependency struct {
	ArtifactID string `json:"artifactId"`
	Path       string `json:"path"`
	Hash       string `json:"hash"`
	Job        string `json:"job,omitempty"`
}
type queueEntry struct {
	Assets       []ImportAssetInfo  `json:"assets,omitempty"`
	Path         string             `json:"path"`
	Dependencies []importDependency `json:"dependencies,omitempty"`
	ImportJob
	Version   int    `json:"version"`
	Root      string `json:"root"`
	Canonical []byte `json:"canonical"`
	Hash      string `json:"hash"`
	Preview   string `json:"preview"`
	Source    string `json:"source"`
}
type publicationPolicy struct {
	Disabled bool `json:"disabled"`
}

func (s *Service) ConfigureImports(stateDir string) error {
	if !filepath.IsAbs(stateDir) {
		return ErrImportState
	}
	s.importMu.Lock()
	defer s.importMu.Unlock()
	if s.importClosed {
		return ErrUnavailable
	}
	base := canonicalProfileRoot(stateDir)
	if s.importState != "" {
		if s.importState != base {
			return ErrImportState
		}
		return nil
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return ErrImportState
	}
	root, err := os.OpenRoot(stateDir)
	if err != nil {
		return ErrImportState
	}
	defer root.Close()
	store, err := openChildDirectory(root, importDirectoryName, true)
	if err != nil {
		return ErrImportState
	}
	store.Close()
	s.importState = base
	s.importRoot = filepath.Join(base, importDirectoryName)
	for _, profile := range s.profiles {
		if worker := s.workers[profile.ID]; worker != nil {
			select {
			case <-worker.done:
			default:
				worker.signal()
				continue
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		directory := profileStatePath(base, profile)
		ledger, ledgerErr := loadLedger(directory)
		worker := &profileWorker{profile: profile, canonicalRoot: canonicalProfileRoot(profile.Root), stateDir: directory, options: IndexOptions{StateDir: base}, validate: func() error { return ValidateProfiles(s.profiles, s.mappings) }, ledger: ledger, ledgerErr: ledgerErr, allowed: map[string]string{}, lineage: map[string]LineageState{}, retry: make(chan struct{}, 1), cancel: cancel, done: make(chan struct{}), beforeRefresh: func(ctx context.Context) error { return s.processImports(ctx, profile) }}
		s.publishers[profile.ID] = worker
		go worker.run(ctx)
	}
	return nil
}
func (s *Service) importDirectory() (string, error) {
	s.importMu.RLock()
	defer s.importMu.RUnlock()
	if s.importClosed {
		return "", ErrUnavailable
	}
	if s.importRoot == "" {
		return "", ErrImportState
	}
	return s.importRoot, nil
}
func (s *Service) withPublicationPolicy(ctx context.Context, key string, fn func(*os.Root, publicationPolicy) error) error {
	directory, err := s.importDirectory()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ErrImportState
	}
	defer root.Close()
	name := policyPrefix + contentHash(key)
	lock, err := acquireNamedWriter(ctx, root, name+".lock")
	if err != nil {
		return err
	}
	defer releaseWriter(lock)
	if err := ctx.Err(); err != nil {
		return err
	}
	policy := publicationPolicy{}
	b, err := readRegular(root, name+".json", 4096)
	if err == nil {
		if json.Unmarshal(b, &policy) != nil {
			return ErrImportState
		}
	} else if !os.IsNotExist(err) {
		return ErrImportState
	}
	if _, err := s.importDirectory(); err != nil {
		return err
	}
	return fn(root, policy)
}
func (s *Service) SetPublicationDisabled(ctx context.Context, sessionKey string, disabled bool) error {
	return s.UpdatePublicationPolicy(ctx, sessionKey, disabled, func() error { return nil })
}
func (s *Service) UpdatePublicationPolicy(ctx context.Context, sessionKey string, disabled bool, persist func() error) error {
	if sessionKey == "" || persist == nil {
		return ErrImportSource
	}
	err := s.withPublicationPolicy(ctx, sessionKey, func(root *os.Root, _ publicationPolicy) error {
		save := func() error {
			return storeJSON(root, policyPrefix+contentHash(sessionKey)+".json", publicationPolicy{Disabled: disabled})
		}
		if disabled {
			if err := save(); err != nil {
				return err
			}
			return persist()
		}
		if err := persist(); err != nil {
			return err
		}
		return save()
	})
	if err == nil {
		_ = s.RetryImports("")
	}
	return err
}
func importQueueID(entry ImportEntry, source ImportSource) string {
	sessionKey := ""
	if source.Session != nil {
		sessionKey = source.Session.Key
	}
	return queuePrefix + contentHash(entry.Destination+"\n"+entry.Document.ID+"\n"+contentHash(entry.Canonical)+"\n"+sessionKey+"\n"+entry.Path)
}
func (s *Service) ConfirmImport(ctx context.Context, previewID string, selectedKeys []string, revalidate func(string) ([]byte, error)) (ImportReport, error) {
	directory, err := s.importDirectory()
	if err != nil {
		return ImportReport{}, err
	}
	if revalidate == nil || len(selectedKeys) == 0 {
		return ImportReport{}, ErrImportSource
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ImportReport{}, ErrImportState
	}
	defer root.Close()
	record, err := loadImportRecord(root, previewID)
	if err != nil {
		return ImportReport{}, err
	}
	selected := map[string]bool{}
	entries := map[string]ImportEntry{}
	sources := map[string]ImportSource{}
	for _, entry := range record.Preview.Entries {
		entries[entry.Key] = entry
	}
	for _, source := range record.Sources {
		sources[source.Key] = source
	}
	keys := map[string]bool{}
	for _, key := range selectedKeys {
		entry, exists := entries[key]
		if !exists || entry.Disposition == "excluded" || len(entry.Repairs) > 0 || entry.Canonical == "" {
			return ImportReport{}, ErrImportRepair
		}
		if selected[key] {
			return ImportReport{}, ErrImportSource
		}
		selected[key] = true
		for _, asset := range entry.Assets {
			keys[asset.Key] = true
		}
		keys[key] = true
		for _, evidenceKey := range entry.EvidenceKeys {
			evidence, ok := sources[evidenceKey]
			if !ok || record.Hashes[evidenceKey] == "" {
				return ImportReport{}, ErrImportStale
			}
			keys[evidenceKey] = true
			if evidence.Session != nil {
				keys[evidence.Session.Key] = true
			}
		}
		if source := sources[key]; source.Session != nil {
			keys[source.Session.Key] = true
		}
	}
	for key := range keys {
		if err := ctx.Err(); err != nil {
			return ImportReport{}, err
		}
		current, err := revalidate(key)
		if err != nil || contentHash(string(current)) != record.Hashes[key] {
			return ImportReport{}, ErrImportStale
		}
	}
	for _, key := range selectedKeys {
		entry := entries[key]
		for _, pin := range record.Dependencies[key] {
			selectedTarget := false
			for targetKey := range selected {
				target := entries[targetKey]
				if target.Destination == entry.Destination && target.Document.ID == pin.ArtifactID && target.Path == pin.Path && contentHash(target.Canonical) == pin.Hash {
					selectedTarget = true
				}
			}
			if !selectedTarget {
				existing, err := s.Read(Scope{ReadProfiles: []string{entry.Destination}}, Reference{Profile: entry.Destination, ID: pin.ArtifactID, Path: pin.Path})
				if err != nil || contentHash(existing.Content) != pin.Hash {
					return ImportReport{}, ErrImportRepair
				}
			}
		}
		for _, dependency := range entry.Dependencies {
			target := entries[dependency]
			scope := Scope{ReadProfiles: []string{entry.Destination}}
			existing, readErr := s.Read(scope, Reference{Profile: entry.Destination, ID: target.Document.ID, Path: target.Path})
			if readErr == nil && existing.Content == target.Canonical {
				continue
			}
			if selected[dependency] {
				if errors.Is(readErr, ErrUnavailable) {
					continue
				}
				if errors.Is(readErr, ErrNotFound) && target.Path == target.Document.ID+".md" {
					if _, idErr := s.Read(scope, Reference{Profile: entry.Destination, ID: target.Document.ID}); errors.Is(idErr, ErrNotFound) {
						continue
					}
				}
			}
			return ImportReport{}, ErrImportRepair
		}
	}
	if err := ValidateProfiles(s.profiles, s.mappings); err != nil {
		return ImportReport{}, err
	}
	for _, key := range selectedKeys {
		entry, source := entries[key], sources[key]
		profile := s.profile(entry.Destination)
		if profile.ID == "" {
			return ImportReport{}, ErrScope
		}
		if canonicalProfileRoot(profile.Root) != record.Roots[key] {
			return ImportReport{}, ErrDestinationChanged
		}
		sessionKey := ""
		if source.Session != nil {
			sessionKey = source.Session.Key
			if source.Session.PublicationDisabled {
				return ImportReport{}, ErrPublicationDisabled
			}
		}
		if err := s.withPublicationPolicy(ctx, sessionKey, func(_ *os.Root, policy publicationPolicy) error {
			if policy.Disabled {
				return ErrPublicationDisabled
			}
			return nil
		}); err != nil {
			return ImportReport{}, err
		}
	}
	report := ImportReport{Entries: []ImportJob{}}
	for _, key := range selectedKeys {
		entry, source := entries[key], sources[key]
		profile := s.profile(entry.Destination)
		if profile.ID == "" {
			return report, ErrScope
		}
		if canonicalProfileRoot(profile.Root) != record.Roots[key] {
			return report, ErrDestinationChanged
		}
		sessionKey := ""
		if source.Session != nil {
			sessionKey = source.Session.Key
		}
		canonical := []byte(entry.Canonical)
		hash := contentHash(entry.Canonical)
		id := importQueueID(entry, source)
		dependencies := []importDependency{}
		for _, dependency := range entry.Dependencies {
			target := entries[dependency]
			pin := importDependency{ArtifactID: target.Document.ID, Path: target.Path, Hash: contentHash(target.Canonical)}
			if selected[dependency] {
				pin.Job = importQueueID(target, sources[dependency])
			}
			dependencies = append(dependencies, pin)
		}
		for _, pin := range record.Dependencies[key] {
			found := false
			for _, dependency := range dependencies {
				if dependency.ArtifactID == pin.ArtifactID {
					found = true
				}
			}
			if !found {
				pin.Job = ""
				dependencies = append(dependencies, pin)
			}
		}
		job := queueEntry{Assets: entry.Assets, Path: entry.Path, Dependencies: dependencies, ImportJob: ImportJob{ID: id, ArtifactID: entry.Document.ID, Profile: entry.Destination, SessionKey: sessionKey, State: ImportPending}, Version: 1, Root: record.Roots[key], Canonical: canonical, Hash: hash, Preview: previewID, Source: key}
		err := s.withPublicationPolicy(ctx, sessionKey, func(store *os.Root, policy publicationPolicy) error {
			if source.Session != nil && source.Session.PublicationDisabled {
				policy.Disabled = true
				if err := storeJSON(store, policyPrefix+contentHash(sessionKey)+".json", policy); err != nil {
					return err
				}
			}
			if policy.Disabled {
				return ErrPublicationDisabled
			}
			if prior, err := loadQueueEntry(store, id+".json"); err == nil {
				job = prior
				return nil
			} else if !os.IsNotExist(err) {
				return err
			}
			b, err := json.Marshal(job)
			if err != nil {
				return ErrImportState
			}
			return writeOnce(store, id+".json", b)
		})
		if err != nil {
			return report, err
		}
		report.Entries = append(report.Entries, job.ImportJob)
	}
	_ = s.RetryImports("")
	return report, nil
}
func loadQueueEntry(root *os.Root, name string) (queueEntry, error) {
	var job queueEntry
	b, err := readRegular(root, name, 16<<20)
	if err != nil {
		return job, err
	}
	if json.Unmarshal(b, &job) != nil || job.Version != 1 || job.ID+".json" != name || !strings.HasPrefix(job.ID, queuePrefix) || !componentPattern.MatchString(job.ID) || contentHash(string(job.Canonical)) != job.Hash {
		return job, ErrImportState
	}
	if job.Path == "" {
		job.Path = job.ArtifactID + ".md"
	}
	if !filepath.IsLocal(job.Path) {
		return job, ErrImportState
	}
	d, err := Parse(job.Canonical)
	if err != nil || d.ID != job.ArtifactID {
		return job, ErrImportState
	}
	return job, nil
}
func (s *Service) ImportStatus() (ImportReport, error) {
	directory, err := s.importDirectory()
	if err != nil {
		return ImportReport{}, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ImportReport{}, ErrImportState
	}
	defer root.Close()
	file, err := root.Open(".")
	if err != nil {
		return ImportReport{}, ErrImportState
	}
	defer file.Close()
	names, err := file.Readdirnames(-1)
	if err != nil {
		return ImportReport{}, ErrImportState
	}
	sort.Strings(names)
	report := ImportReport{Entries: []ImportJob{}}
	for _, name := range names {
		if !strings.HasPrefix(name, queuePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		job, err := loadQueueEntry(root, name)
		if err != nil {
			return report, ErrImportState
		}
		view := job.ImportJob
		if view.State != ImportWritten && view.State != ImportConflict {
			profile := s.profile(view.Profile)
			if profile.ID == "" {
				view.State = ImportUnavailable
				view.Error = ErrScope.Error()
			} else if canonicalProfileRoot(profile.Root) != job.Root {
				view.State = ImportDestinationChanged
				view.Error = ErrDestinationChanged.Error()
			}
			policyBytes, policyErr := readRegular(root, policyPrefix+contentHash(view.SessionKey)+".json", 4096)
			if policyErr != nil && !os.IsNotExist(policyErr) {
				return report, ErrImportState
			}
			if policyErr == nil {
				var policy publicationPolicy
				if json.Unmarshal(policyBytes, &policy) != nil {
					return report, ErrImportState
				}
				if policy.Disabled {
					view.State = ImportPaused
					view.Error = ErrPublicationDisabled.Error()
				}
			}
		}
		report.Entries = append(report.Entries, view)
	}
	return report, nil
}
func (s *Service) RetryImports(profile string) error {
	if _, err := s.importDirectory(); err != nil {
		return err
	}
	if profile != "" && s.profile(profile).ID == "" {
		return ErrScope
	}
	s.importMu.RLock()
	defer s.importMu.RUnlock()
	for id, worker := range s.workers {
		if profile == "" || id == profile {
			worker.signal()
		}
	}
	for id, worker := range s.publishers {
		if profile == "" || id == profile {
			worker.signal()
		}
	}
	return nil
}
func (s *Service) RevalidateImportDestination(ctx context.Context, id string) error {
	if !componentPattern.MatchString(id) {
		return ErrImportSource
	}
	directory, err := s.importDirectory()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ErrImportState
	}
	defer root.Close()
	job, err := loadQueueEntry(root, id+".json")
	if err != nil {
		return err
	}
	err = s.withPublicationPolicy(ctx, job.SessionKey, func(store *os.Root, _ publicationPolicy) error {
		current, err := loadQueueEntry(store, id+".json")
		if err != nil {
			return err
		}
		if current.State == ImportWritten || current.State == ImportConflict {
			return ErrImportRepair
		}
		if err := ValidateProfiles(s.profiles, s.mappings); err != nil {
			return err
		}
		profile := s.profile(current.Profile)
		if profile.ID == "" {
			return ErrScope
		}
		current.Root = canonicalProfileRoot(profile.Root)
		current.State = ImportPending
		current.Error = ""
		return storeJSON(store, id+".json", current)
	})
	if err == nil {
		_ = s.RetryImports(job.Profile)
	}
	return err
}
func (s *Service) processImports(ctx context.Context, profile Profile) error {
	directory, err := s.importDirectory()
	if errors.Is(err, ErrImportState) {
		return nil
	}
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ErrImportState
	}
	file, err := root.Open(".")
	if err != nil {
		root.Close()
		return ErrImportState
	}
	names, err := file.Readdirnames(-1)
	file.Close()
	root.Close()
	if err != nil {
		return ErrImportState
	}
	sort.Strings(names)
	for _, name := range names {
		if !strings.HasPrefix(name, queuePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			return ErrImportState
		}
		job, err := loadQueueEntry(root, name)
		root.Close()
		if err != nil {
			return ErrImportState
		}
		if job.Profile != profile.ID || job.State == ImportWritten || job.State == ImportConflict {
			continue
		}
		err = s.withPublicationPolicy(ctx, job.SessionKey, func(store *os.Root, policy publicationPolicy) error {
			job, err := loadQueueEntry(store, name)
			if err != nil {
				return err
			}
			if job.State == ImportWritten || job.State == ImportConflict {
				return nil
			}
			if policy.Disabled {
				job.State = ImportPaused
				job.Error = ErrPublicationDisabled.Error()
				return storeJSON(store, name, job)
			}
			if err := ValidateProfiles(s.profiles, s.mappings); err != nil {
				job.State = ImportUnavailable
				job.Error = ErrInvalidConfig.Error()
				return storeJSON(store, name, job)
			}
			if canonicalProfileRoot(profile.Root) != job.Root {
				job.State = ImportDestinationChanged
				job.Error = ErrDestinationChanged.Error()
				return storeJSON(store, name, job)
			}
			err = s.writeCanonical(ctx, profile, job, store)
			switch {
			case err == nil:
				job.State = ImportWritten
				job.Error = ""
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				return err
			case errors.Is(err, ErrImportPending):
				job.State = ImportPending
				job.Error = ErrImportPending.Error()
			case errors.Is(err, ErrImportRepair):
				job.State = ImportRepairRequired
				job.Error = ErrImportRepair.Error()
			case errors.Is(err, ErrConflict):
				job.State = ImportConflict
				job.Error = ErrConflict.Error()
			default:
				job.State = ImportUnavailable
				job.Error = ErrImportWrite.Error()
			}
			return storeJSON(store, name, job)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
