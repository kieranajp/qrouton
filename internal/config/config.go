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
	Orgs []string `json:"orgs"` // GitHub orgs for the repo picker
	Root string   `json:"root"` // sessions live flat under it; mirrors under <root>/.mirrors

	// Keyed by runner id rather than a bare argv list: with the runner buried in
	// argv[0], an override that dropped a flag looked like one that merely named
	// a runner.
	Launch map[string][]string `json:"launch,omitempty"`

	Editor []string `json:"editor,omitempty"`

	// "dock" (the default) for a tab in the session's right pane, "float" for an
	// OS window.
	Windows string `json:"windows,omitempty"`
}

// Only the floating value floats, so a misspelling still gives the default.
func (c *Config) Dock() bool { return c.Windows != WindowsFloat }

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
// owners, so nothing may block a launch here; EnsureOrgs asks at the first
// repository search. QROUTON_ROOT / QROUTON_ORGS override at runtime.
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

// EnsureOrgs prompts for GitHub owners and persists them, the first time
// something actually needs them — which is the repository picker, not qrouton
// itself. A no-op once they are known, so the ordinary path never prompts.
func EnsureOrgs(cfg *Config) error {
	if len(cfg.Orgs) > 0 {
		return nil
	}
	orgs := wizardOrgsDefault
	err := huh.NewInput().Title(wizardOrgsTitle).
		Description(wizardOrgsDescription).
		Value(&orgs).
		Validate(func(s string) error {
			if len(splitOrgs(s)) == 0 {
				return errOrgsRequired
			}
			return nil
		}).Run()
	if err != nil {
		return err
	}
	cfg.Orgs = splitOrgs(orgs)
	return Save(cfg)
}

// Save writes the whole config back to disk, creating its directory. It is how
// the owners EnsureOrgs collected survive to the next launch.
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
