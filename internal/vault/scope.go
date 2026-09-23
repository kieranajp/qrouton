package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validate refuses duplicate ids and roots that share, nest in, or reach each
// other through symlinks.
func Validate(profiles []Profile) error {
	resolved := make([]string, len(profiles))
	for i, p := range profiles {
		if !profileID.MatchString(p.ID) || !filepath.IsAbs(p.Root) {
			return fmt.Errorf("%w: %q", ErrInvalidProfile, p.ID)
		}
		real := resolve(p.Root)
		for j, other := range profiles[:i] {
			if other.ID == p.ID {
				return fmt.Errorf("%w: %q", ErrDuplicateProfile, p.ID)
			}
			if contains(resolved[j], real) || contains(real, resolved[j]) {
				return fmt.Errorf("%w: %q and %q", ErrOverlappingRoots, other.ID, p.ID)
			}
		}
		resolved[i] = real
	}
	return nil
}

// resolve follows symlinks as far as the path exists, so a root not created
// yet still compares against the others.
func resolve(path string) string {
	path = filepath.Clean(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(resolve(parent), filepath.Base(path))
}

// contains also compares directory identity, so two spellings of one path on
// a case-insensitive filesystem still overlap.
func contains(parent, child string) bool {
	if rel, err := filepath.Rel(parent, child); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	outer, err := os.Stat(parent)
	if err != nil {
		return false
	}
	for dir := child; ; dir = filepath.Dir(dir) {
		if inner, err := os.Stat(dir); err == nil && os.SameFile(outer, inner) {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}
