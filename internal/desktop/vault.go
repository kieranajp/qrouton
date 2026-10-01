package desktop

import (
	"fmt"
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/vault"
)

// openVault answers nil only when there is nowhere to cache vectors.
func openVault(cfg *config.Config) *vault.Set {
	cacheDir, err := vault.DefaultCacheDir()
	if err != nil {
		return nil
	}
	set := vault.NewSet(vault.NewOllama(), cacheDir)
	_ = set.Configure(thoughtsProfiles(cfg))
	return set
}

func thoughtsProfiles(cfg *config.Config) []vault.Profile {
	var profiles []vault.Profile
	for _, root := range cfg.ThoughtsRoots() {
		profiles = append(profiles, vault.Profile{ID: root.ID, Root: root.Path})
	}
	return profiles
}

func reconfigureVault(set *vault.Set) func(*config.Config) error {
	return func(cfg *config.Config) error {
		if set == nil {
			return nil
		}
		return set.Configure(thoughtsProfiles(cfg))
	}
}

// sessionThoughts follows the session's thoughts link, so search reads the
// root the session writes to and nothing in its manifest can widen it. The
// narrowest configured root decides, so an unindexed folder is refused rather
// than read through a wider one.
func sessionThoughts(set *vault.Set, roots []config.ThoughtsRoot, sessionRoot string) (*vault.Service, error) {
	real, err := filepath.EvalSymlinks(filepath.Join(sessionRoot, sessionpaths.ThoughtsDirName))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoThoughtsRoot, err)
	}
	var match *config.ThoughtsRoot
	for i, r := range roots {
		if vault.Contains(r.Path, real) && (match == nil || vault.Contains(match.Path, r.Path)) {
			match = &roots[i]
		}
	}
	if match != nil {
		if service := set.ForProfile(vault.Profile{ID: match.ID, Root: match.Path}); service != nil {
			return service, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoThoughtsRoot, real)
}
