package core

import (
	"fmt"
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/vault"
)

func OpenVault(cfg *config.Config) *vault.Set {
	set := vault.NewSet(vault.NewOllama(), filepath.Join(config.CacheDir(), vault.CacheDirName))
	_ = set.Configure(ThoughtsProfiles(cfg))
	return set
}

func ThoughtsProfiles(cfg *config.Config) []vault.Profile {
	var profiles []vault.Profile
	for _, root := range cfg.ThoughtsRoots() {
		profiles = append(profiles, vault.Profile{ID: root.ID, Root: root.Path})
	}
	return profiles
}

func ReconfigureVault(set *vault.Set) func(*config.Config) error {
	return func(cfg *config.Config) error {
		if set == nil {
			return nil
		}
		return set.Configure(ThoughtsProfiles(cfg))
	}
}

// SessionThoughts follows the session's thoughts link, so search reads the
// root the session writes to and nothing in its manifest can widen it. The
// narrowest configured root decides, so an unindexed folder is refused rather
// than read through a wider one.
func SessionThoughts(set *vault.Set, roots []config.ThoughtsRoot, sessionRoot string) (*vault.Service, error) {
	real, err := filepath.EvalSymlinks(sessionpaths.ThoughtsLink(sessionRoot))
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
