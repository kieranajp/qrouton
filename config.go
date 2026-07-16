package main

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

func configPath() string { return filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "config.json") }
func cachePath() string  { return filepath.Join(xdgDir("XDG_CACHE_HOME", ".cache"), "repos.json") }

// loadConfig reads config.json, running the first-run wizard if it doesn't exist.
// QROUTON_ROOT / QROUTON_ORGS override at runtime.
func loadConfig() (*Config, error) {
	cfg := &Config{}
	b, err := os.ReadFile(configPath())
	switch {
	case os.IsNotExist(err):
		if cfg, err = wizard(); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", configPath(), err)
		}
	}
	if v := os.Getenv("QROUTON_ROOT"); v != "" {
		cfg.Root = v
	}
	if v := os.Getenv("QROUTON_ORGS"); v != "" {
		cfg.Orgs = splitOrgs(v)
	}
	if len(cfg.Orgs) == 0 {
		return nil, fmt.Errorf("%s: orgs must contain at least one GitHub organization", configPath())
	}
	// Older first-run wizards always wrote this value as the built-in default. Treat that exact
	// shape as unset so existing installations receive current runner defaults.
	if len(cfg.Launch) == 1 && len(cfg.Launch[0]) == 1 && cfg.Launch[0][0] == "claude" {
		cfg.Launch = nil
	}
	cfg.Root = expandHome(cfg.Root)
	return cfg, os.MkdirAll(cfg.Root, 0o755)
}

func wizard() (*Config, error) {
	root, orgs := "~/work", "lifesum"
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Root directory").
			Description("Sessions live flat under it; repo mirrors under <root>/.mirrors").
			Value(&root),
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
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return cfg, os.WriteFile(configPath(), b, 0o644)
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
