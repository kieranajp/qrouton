package config

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
)

// ThoughtsFingerprint identifies the effective thoughts folders: the default
// path and each shared folder's id, path and orgs. Order, case and spacing of
// orgs, and the order of folders, do not change it.
func (c *Config) ThoughtsFingerprint() string {
	var entries []string
	for _, r := range c.ThoughtsRoots() {
		orgs := make([]string, 0, len(r.Orgs))
		for _, org := range r.Orgs {
			orgs = append(orgs, strings.ToLower(strings.TrimSpace(org)))
		}
		slices.Sort(orgs)
		entries = append(entries, strings.Join([]string{r.ID, ExpandHome(r.Path), strings.Join(orgs, fingerprintOrgJoin)}, fingerprintFieldJoin))
	}
	slices.Sort(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, fingerprintEntryJoin)))
	return hex.EncodeToString(sum[:fingerprintBytes])
}
