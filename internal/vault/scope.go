package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Root string `json:"root"`
}
type Scope struct {
	Repositories []string `json:"repositories,omitempty"`
	ReadProfiles []string `json:"readProfiles,omitempty"`
	Destination  string   `json:"destination,omitempty"`
}
type ReadScope struct {
	Profiles          []string `json:"profiles"`
	Unmapped          []string `json:"unmapped"`
	SelectionRequired bool     `json:"selectionRequired"`
}

func ValidateProfiles(profiles []Profile, mappings map[string]string) error {
	ids := map[string]bool{}
	roots := map[string]bool{}
	for _, profile := range profiles {
		if !componentPattern.MatchString(profile.ID) || strings.TrimSpace(profile.Name) == "" || !filepath.IsAbs(profile.Root) || ids[profile.ID] || roots[filepath.Clean(profile.Root)] {
			return ErrInvalidConfig
		}
		ids[profile.ID] = true
		root := filepath.Clean(profile.Root)
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			root = resolved
		} else if !os.IsNotExist(err) {
			return ErrInvalidConfig
		}
		for prior := range roots {
			relative, err := filepath.Rel(prior, root)
			reverse, backErr := filepath.Rel(root, prior)
			if (err == nil && (relative == "." || filepath.IsLocal(relative))) || (backErr == nil && (reverse == "." || filepath.IsLocal(reverse))) {
				return ErrInvalidConfig
			}
		}
		roots[root] = true
	}
	orgs := map[string]bool{}
	for org, id := range mappings {
		normalized := strings.ToLower(org)
		if !ValidRepository("github.com/"+org+"/repo") || !ids[id] || orgs[normalized] {
			return ErrInvalidConfig
		}
		orgs[normalized] = true
	}
	return nil
}
func ResolveReadProfiles(profiles []Profile, mappings map[string]string, scope Scope) (ReadScope, error) {
	result := ReadScope{Profiles: []string{}, Unmapped: []string{}}
	known := map[string]bool{}
	for _, profile := range profiles {
		known[profile.ID] = true
	}
	normalized := map[string]string{}
	for org, id := range mappings {
		normalized[strings.ToLower(org)] = id
	}
	admitted := map[string]bool{}
	unmapped := map[string]bool{}
	for _, repo := range scope.Repositories {
		if !ValidRepository(repo) {
			return result, fmt.Errorf("%w: repository %s", ErrScope, repo)
		}
		org := strings.ToLower(strings.Split(repo, "/")[1])
		id := normalized[org]
		if id == "" || !known[id] {
			unmapped[org] = true
		} else {
			admitted[id] = true
		}
	}
	if len(scope.ReadProfiles) > 0 {
		selected := map[string]bool{}
		for _, id := range scope.ReadProfiles {
			if !known[id] || (len(scope.Repositories) > 0 && !admitted[id]) {
				return result, ErrScope
			}
			selected[id] = true
		}
		admitted = selected
	}
	result.SelectionRequired = len(scope.Repositories) == 0 && len(scope.ReadProfiles) == 0
	for id := range admitted {
		result.Profiles = append(result.Profiles, id)
	}
	for org := range unmapped {
		result.Unmapped = append(result.Unmapped, org)
	}
	sort.Strings(result.Profiles)
	sort.Strings(result.Unmapped)
	return result, nil
}
func ResolveDestination(profiles []Profile, mappings map[string]string, scope Scope) (string, error) {
	if scope.Destination != "" {
		for _, profile := range profiles {
			if profile.ID == scope.Destination {
				return profile.ID, nil
			}
		}
		return "", ErrScope
	}
	scope.ReadProfiles = nil
	resolved, err := ResolveReadProfiles(profiles, mappings, scope)
	if err != nil {
		return "", err
	}
	if len(resolved.Profiles) != 1 || len(resolved.Unmapped) > 0 || len(scope.Repositories) == 0 {
		return "", ErrDestinationRequired
	}
	return resolved.Profiles[0], nil
}
