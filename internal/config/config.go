package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

type Config struct {
	Orgs   []string   `json:"orgs"`   // GitHub orgs for the repo picker
	Root   string     `json:"root"`   // sessions live flat under it; mirrors under <root>/.mirrors
	Launch [][]string `json:"launch"` // optional exact overrides for supported runner commands
	Editor []string   `json:"editor,omitempty"`
}

// xdgDir resolves $XDG_<base>_HOME/qrouton, or its documented fallback.
func xdgDir(envVar, fallback string) string {
	base := os.Getenv(envVar)
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(base, appDirName)
}

func Path() string {
	return filepath.Join(xdgDir(configHomeEnvVar, configHomeFallback), configFileName)
}

func CachePath() string {
	return filepath.Join(xdgDir(cacheHomeEnvVar, cacheHomeFallback), cacheFileName)
}

// Load reads config.json, running the first-run wizard if it doesn't exist.
// QROUTON_ROOT / QROUTON_ORGS override at runtime.
func Load() (*Config, error) {
	cfg := &Config{}
	b, err := os.ReadFile(Path())
	switch {
	case os.IsNotExist(err):
		if cfg, err = wizard(); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", Path(), err)
		}
	}
	if v := os.Getenv(rootEnvVar); v != "" {
		cfg.Root = v
	}
	if v := os.Getenv(orgsEnvVar); v != "" {
		cfg.Orgs = splitOrgs(v)
	}
	if len(cfg.Orgs) == 0 {
		return nil, fmt.Errorf("%s: %w", Path(), ErrNoOrgs)
	}
	cfg.Root = expandHome(cfg.Root)
	if strings.TrimSpace(cfg.Root) == "" {
		return nil, fmt.Errorf("%s: %w", Path(), ErrNoRoot)
	}
	return cfg, os.MkdirAll(cfg.Root, dirMode)
}

func wizard() (*Config, error) {
	root, orgs := wizardRootDefault, wizardOrgsDefault
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title(wizardRootTitle).
			Description(wizardRootDescription).
			Value(&root).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errRootRequired
				}
				return nil
			}),
		huh.NewInput().Title(wizardOrgsTitle).
			Description(wizardOrgsDescription).
			Value(&orgs).
			Validate(func(s string) error {
				if len(splitOrgs(s)) == 0 {
					return errOrgsRequired
				}
				return nil
			}),
	)).Run()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Orgs: splitOrgs(orgs), Root: root}
	if err := os.MkdirAll(filepath.Dir(Path()), dirMode); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return cfg, os.WriteFile(Path(), b, fileMode)
}

func splitOrgs(s string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, org := range strings.Split(s, orgSeparator) {
		org = strings.TrimSpace(org)
		if org != "" && !seen[org] {
			seen[org] = true
			out = append(out, org)
		}
	}
	return out
}

func expandHome(p string) string {
	if strings.HasPrefix(p, homeSlash) || p == homePrefix {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, homePrefix))
	}
	return p
}
