package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Orgs []string `json:"orgs"` // GitHub orgs for the repo picker
	Root string   `json:"root"` // sessions live flat under it; mirrors under <root>/.mirrors

	// Keyed by runner id rather than a bare argv list: with the runner buried in
	// argv[0], an override that dropped a flag looked like one that merely named
	// a runner.
	Launch map[string][]string `json:"launch,omitempty"`

	Editor []string `json:"editor,omitempty"`

	// Absent reads false, so a hand-written config sees the first-run flow once.
	Welcomed bool `json:"welcomed,omitempty"`
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

// Load reads config.json if it exists, and otherwise starts from defaults —
// deliberately without prompting. A zero-repo session needs neither a root nor
// owners, so nothing may block a launch here, and an empty owner list is simply
// an empty repository list. QROUTON_ROOT / QROUTON_ORGS override at runtime.
func Load() (*Config, error) {
	cfg := &Config{}
	b, err := os.ReadFile(Path())
	switch {
	case os.IsNotExist(err):
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
	if strings.TrimSpace(cfg.Root) == "" {
		cfg.Root = defaultRoot
	}
	cfg.Root = expandHome(cfg.Root)
	return cfg, os.MkdirAll(cfg.Root, dirMode)
}

// Save writes the whole config back to disk, creating its directory.
func Save(cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(Path()), dirMode); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), b, fileMode)
}

// WithoutOverrides drops the runtime overrides from an environment, so a process
// inheriting it reads the config file's answer rather than this one's.
func WithoutOverrides(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, envAssign)
		if name == rootEnvVar || name == orgsEnvVar {
			continue
		}
		out = append(out, entry)
	}
	return out
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

// ExpandHome resolves a leading ~ against the user's home directory, the same
// expansion Load applies to a configured root.
func ExpandHome(p string) string { return expandHome(p) }

func expandHome(p string) string {
	if strings.HasPrefix(p, homeSlash) || p == homePrefix {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, homePrefix))
	}
	return p
}
