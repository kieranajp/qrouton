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
	Orgs        []string   `json:"orgs"`   // GitHub orgs for the repo picker
	Root        string     `json:"root"`   // sessions live flat under it; mirrors under <root>/.mirrors
	Launch      [][]string `json:"launch"` // optional exact overrides for supported runner commands
	Editor      []string   `json:"editor,omitempty"`
	Multiplexer string     `json:"multiplexer,omitempty"` // workspace backend; empty selects Zellij
}

// xdgDir: $XDG_x_HOME/qrouton, falling back to ~/.config|~/.cache per the spec (not os.UserConfigDir —
// on darwin that's ~/Library/Application Support and the design pinned XDG).
func xdgDir(envVar, fallback string) string {
	base := os.Getenv(envVar)
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(base, "qrouton")
}

func Path() string      { return filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "config.json") }
func CachePath() string { return filepath.Join(xdgDir("XDG_CACHE_HOME", ".cache"), "repos.json") }

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
	if v := os.Getenv("QROUTON_ROOT"); v != "" {
		cfg.Root = v
	}
	if v := os.Getenv("QROUTON_ORGS"); v != "" {
		cfg.Orgs = splitOrgs(v)
	}
	if len(cfg.Orgs) == 0 {
		return nil, fmt.Errorf("%s: orgs must contain at least one GitHub organization", Path())
	}
	cfg.Root = expandHome(cfg.Root)
	if strings.TrimSpace(cfg.Root) == "" {
		return nil, fmt.Errorf("%s: root must be set (or export QROUTON_ROOT)", Path())
	}
	return cfg, os.MkdirAll(cfg.Root, 0o755)
}

func wizard() (*Config, error) {
	root, orgs := "~/work", "lifesum"
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Root directory").
			Description("Sessions live flat under it; repo mirrors under <root>/.mirrors").
			Value(&root).
			Validate(func(s string) error {
				// An empty root would be written to disk and then fail Load on
				// every subsequent start until the config is hand-edited.
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("root directory is required")
				}
				return nil
			}),
		huh.NewInput().Title("GitHub orgs").
			Description("Comma-separated organizations whose repos the session picker lists").
			Value(&orgs).
			Validate(func(s string) error {
				if len(splitOrgs(s)) == 0 {
					return fmt.Errorf("need at least one organization")
				}
				return nil
			}),
	)).Run()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Orgs: splitOrgs(orgs), Root: root}
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return cfg, os.WriteFile(Path(), b, 0o644)
}

func splitOrgs(s string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, org := range strings.Split(s, ",") {
		org = strings.TrimSpace(org)
		if org != "" && !seen[org] {
			seen[org] = true
			out = append(out, org)
		}
	}
	return out
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}
