package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

type conflictEvidence struct {
	ArtifactID string   `json:"artifactId"`
	Incoming   string   `json:"incoming"`
	Existing   []string `json:"existing"`
}

func writeOnce(root *os.Root, name string, data []byte) error {
	if !filepath.IsLocal(name) {
		return ErrScope
	}
	if info, err := root.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return ErrImportWrite
		}
		existing, err := readRegular(root, name, maxIndexBytes)
		if err != nil {
			return err
		}
		if bytes.Equal(existing, data) {
			return nil
		}
		return ErrConflict
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := randomName(".incoming-")
	if err != nil {
		return err
	}
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := root.Link(temporary, name); err != nil {
		if !os.IsExist(err) {
			return err
		}
		existing, readErr := readRegular(root, name, maxIndexBytes)
		if readErr == nil && bytes.Equal(existing, data) {
			return nil
		}
		return ErrConflict
	}
	return syncRoot(root)
}
func preserveConflict(store *os.Root, job queueEntry, existing []ReadResult) error {
	evidence := conflictEvidence{ArtifactID: job.ArtifactID, Incoming: job.Hash, Existing: []string{}}
	filename := conflictPrefix + job.ID + ".json"
	if prior, err := readRegular(store, filename, 1<<20); err == nil {
		if json.Unmarshal(prior, &evidence) != nil {
			return ErrImportState
		}
	}
	if err := writeOnce(store, evidencePrefix+job.Hash, job.Canonical); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, hash := range evidence.Existing {
		seen[hash] = true
	}
	for _, entry := range existing {
		hash := contentHash(entry.Content)
		if err := writeOnce(store, evidencePrefix+hash, []byte(entry.Content)); err != nil {
			return err
		}
		if !seen[hash] {
			evidence.Existing = append(evidence.Existing, hash)
			seen[hash] = true
		}
	}
	sort.Strings(evidence.Existing)
	return storeJSON(store, filename, evidence)
}
func (s *Service) writeCanonical(ctx context.Context, profile Profile, job queueEntry, store *os.Root) error {
	document, err := Parse(job.Canonical)
	if err != nil || document.ID != job.ArtifactID {
		return ErrInvalidDocument
	}
	if document.LegacyImport != nil {
		record, err := loadImportRecord(store, job.Preview)
		if err != nil {
			return ErrImportRepair
		}
		proven := false
		for _, source := range record.Sources {
			if source.Key != job.Source {
				continue
			}
			if contentHash(string(source.Content)) == document.LegacyImport.SourceHash {
				proven = true
			}
			if original, err := Parse(source.Content); err == nil && original.LegacyImport != nil && original.LegacyImport.SourceHash == document.LegacyImport.SourceHash {
				proven = true
			}
		}
		reviewed := false
		for _, entry := range record.Preview.Entries {
			if entry.Key == job.Source && entry.Canonical == string(job.Canonical) && entry.Destination == job.Profile && entry.Disposition != "excluded" {
				reviewed = true
			}
		}
		if !proven || !reviewed {
			return ErrImportRepair
		}
	}
	root, err := os.OpenRoot(profile.Root)
	if err != nil {
		return ErrImportWrite
	}
	defer root.Close()
	lock, err := acquireNamedWriter(ctx, root, publicationLockFilename)
	if err != nil {
		return err
	}
	defer releaseWriter(lock)
	if canonicalProfileRoot(profile.Root) != job.Root {
		return ErrDestinationChanged
	}
	inv, err := scan(profile)
	if err != nil {
		return err
	}
	s.importMu.RLock()
	state := s.importState
	s.importMu.RUnlock()
	directory := profileStatePath(state, profile)
	ledger, err := loadLedger(directory)
	if err != nil {
		return err
	}
	ledger.observe(inv)
	existing := []ReadResult{}
	same := false
	sameElsewhere := false
	different := false
	for _, entry := range inv.entries {
		if entry.Document.ID == document.ID {
			existing = append(existing, entry)
			if entry.Content == string(job.Canonical) {
				if entry.Reference.Path == job.Path {
					same = true
				} else {
					sameElsewhere = true
				}
			} else {
				different = true
			}
		}
		if entry.Reference.Path == job.Path && entry.Document.ID != document.ID {
			if err := preserveConflict(store, job, []ReadResult{entry}); err != nil {
				return err
			}
			return ErrImportRepair
		}
	}
	if different || inv.conflicts[document.ID] || ledger.conflicts(document.ID, job.Hash) {
		if err := preserveConflict(store, job, existing); err != nil {
			return err
		}
		return quarantineImport(root, store, directory, ledger, document, job, existing, different)
	}
	if sameElsewhere && !same {
		return ErrImportRepair
	}
	graph := importGraph(inv)
	graph[document.ID] = document
	if ResolveLineage(graph)[document.ID].Invalid {
		return ErrImportRepair
	}
	if err := s.validateImportDependencies(store, profile, job, inv, ledger, map[string]bool{}); err != nil {
		return err
	}
	if err := writeImportAssets(ctx, root, store, job, document.Session); err != nil {
		return err
	}
	if same {
		return saveLedger(directory, ledger)
	}
	if job.Path != document.ID+".md" {
		return ErrImportRepair
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	namespace, err := openChildDirectory(root, document.Session, true)
	if err != nil {
		if errors.Is(err, syscall.ENAMETOOLONG) {
			return ErrImportRepair
		}
		return ErrImportWrite
	}
	defer namespace.Close()
	local := strings.TrimPrefix(document.ID, document.Session+"/")
	if err := writeOnce(namespace, local+".md", job.Canonical); err != nil {
		if errors.Is(err, syscall.ENAMETOOLONG) {
			return ErrImportRepair
		}
		if errors.Is(err, ErrConflict) {
			occupied, readErr := readRegular(namespace, local+".md", maxIndexBytes)
			if readErr != nil {
				return ErrImportWrite
			}
			observed, parseErr := Parse(occupied)
			evidence := []ReadResult{{Document: observed, Content: string(occupied), Supported: parseErr == nil}}
			if err := preserveConflict(store, job, evidence); err != nil {
				return err
			}
			if parseErr != nil || observed.ID != document.ID {
				return ErrImportRepair
			}
			ledger.observe(inventory{entries: evidence, conflicts: map[string]bool{}})
			return quarantineImport(root, store, directory, ledger, document, job, evidence, true)
		}
		return err
	}
	ledger.observe(inventory{entries: []ReadResult{{Document: document, Content: string(job.Canonical), Supported: true}}, conflicts: map[string]bool{}})
	return saveLedger(directory, ledger)
}

func quarantineImport(root, store *os.Root, directory string, ledger identityLedger, document Document, job queueEntry, existing []ReadResult, sibling bool) error {
	if err := preserveConflict(store, job, existing); err != nil {
		return err
	}
	record := ledger.Entries[document.ID]
	record.Conflict = true
	found := false
	for _, hash := range record.Hashes {
		found = found || hash == job.Hash
	}
	if !found {
		record.Hashes = append(record.Hashes, job.Hash)
		sort.Strings(record.Hashes)
	}
	ledger.Entries[document.ID] = record
	if err := saveLedger(directory, ledger); err != nil {
		return err
	}
	if sibling {
		namespace, err := openChildDirectory(root, document.Session, true)
		if err != nil {
			return ErrImportWrite
		}
		err = writeOnce(namespace, "conflict-"+job.Hash+".md", job.Canonical)
		namespace.Close()
		if err != nil {
			return err
		}
	}
	return ErrConflict
}

func (s *Service) validateImportDependencies(store *os.Root, profile Profile, job queueEntry, inv inventory, ledger identityLedger, seen map[string]bool) error {
	if seen[job.ID] {
		return nil
	}
	seen[job.ID] = true
	for _, pin := range job.Dependencies {
		if !validID(pin.ArtifactID) || !filepath.IsLocal(pin.Path) {
			return ErrImportRepair
		}
		if inv.conflicts[pin.ArtifactID] || ledger.conflicts(pin.ArtifactID, pin.Hash) {
			return ErrImportRepair
		}
		found := false
		for _, entry := range inv.entries {
			if entry.Document.ID != pin.ArtifactID {
				continue
			}
			if entry.Reference.Path == pin.Path && entry.Supported && contentHash(entry.Content) == pin.Hash {
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, entry := range inv.entries {
			if entry.Document.ID == pin.ArtifactID || entry.Reference.Path == pin.Path {
				return ErrImportRepair
			}
		}
		if pin.Job == "" {
			return ErrImportRepair
		}
		pending, err := loadQueueEntry(store, pin.Job+".json")
		if os.IsNotExist(err) {
			return ErrImportPending
		}
		if err != nil {
			return ErrImportRepair
		}
		if pending.Profile != profile.ID || pending.ArtifactID != pin.ArtifactID || pending.Path != pin.Path || pending.Hash != pin.Hash {
			return ErrImportRepair
		}
		if pending.Root != job.Root || pending.Root != canonicalProfileRoot(profile.Root) {
			return ErrImportPending
		}
		if pending.State == ImportConflict || pending.State == ImportRepairRequired || pending.State == ImportWritten {
			return ErrImportRepair
		}
		if pending.State == ImportUnavailable {
			return ErrImportPending
		}
		policy := publicationPolicy{}
		encoded, err := readRegular(store, policyPrefix+contentHash(pending.SessionKey)+".json", 4096)
		if err == nil {
			if json.Unmarshal(encoded, &policy) != nil {
				return ErrImportState
			}
		} else if !os.IsNotExist(err) {
			return ErrImportState
		}
		if policy.Disabled {
			return ErrImportPending
		}
		if err := s.validateImportDependencies(store, profile, pending, inv, ledger, seen); err != nil {
			return err
		}
	}
	return nil
}
