package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type identityRecord struct {
	Hashes   []string `json:"hashes"`
	Conflict bool     `json:"conflict,omitempty"`
}
type identityLedger struct {
	Version int                       `json:"version"`
	Entries map[string]identityRecord `json:"entries"`
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
func profileStatePath(directory string, profile Profile) string {
	return filepath.Join(directory, contentHash(profile.ID))
}
func loadLedger(directory string) (identityLedger, error) {
	ledger := identityLedger{Version: 1, Entries: map[string]identityRecord{}}
	root, err := os.OpenRoot(directory)
	if os.IsNotExist(err) {
		return ledger, nil
	}
	if err != nil {
		return ledger, ErrIdentityState
	}
	defer root.Close()
	b, err := readRegular(root, identityFilename, maxIndexBytes)
	if os.IsNotExist(err) {
		return ledger, nil
	}
	if err != nil {
		return ledger, ErrIdentityState
	}
	if err := json.Unmarshal(b, &ledger); err != nil || ledger.Version != 1 || ledger.Entries == nil {
		return identityLedger{}, ErrIdentityState
	}
	for id, record := range ledger.Entries {
		if !validID(id) || len(record.Hashes) == 0 {
			return identityLedger{}, ErrIdentityState
		}
		for _, hash := range record.Hashes {
			if len(hash) != 64 {
				return identityLedger{}, ErrIdentityState
			}
			if _, err := hex.DecodeString(hash); err != nil {
				return identityLedger{}, ErrIdentityState
			}
		}
	}
	return ledger, nil
}
func (ledger identityLedger) conflicts(id, hash string) bool {
	record, exists := ledger.Entries[id]
	if !exists {
		return false
	}
	return record.Conflict || len(record.Hashes) != 1 || record.Hashes[0] != hash
}
func (ledger *identityLedger) observe(inv inventory) bool {
	changed := false
	for _, entry := range inv.entries {
		if !entry.Supported || entry.Document.ID == "" {
			continue
		}
		id := entry.Document.ID
		hash := contentHash(entry.Content)
		record := ledger.Entries[id]
		found := false
		for _, prior := range record.Hashes {
			found = found || prior == hash
		}
		if !found {
			record.Hashes = append(record.Hashes, hash)
			sort.Strings(record.Hashes)
			changed = true
		}
		conflict := len(record.Hashes) > 1 || inv.conflicts[id]
		if conflict && !record.Conflict {
			record.Conflict = true
			changed = true
		}
		ledger.Entries[id] = record
	}
	return changed
}
func saveLedger(directory string, ledger identityLedger) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ErrIdentityState
	}
	defer root.Close()
	b, err := json.Marshal(ledger)
	if err != nil {
		return ErrIdentityState
	}
	if err := replaceRoot(root, identityFilename, b); err != nil {
		return fmt.Errorf("%w: ledger write", ErrIdentityState)
	}
	return nil
}
