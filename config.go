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
	Org    string     `json:"org"`    // GitHub org for the repo picker
	Root   string     `json:"root"`   // sessions live flat under it; mirrors under <root>/.mirrors
	Launch [][]string `json:"launch"` // runner commands; default [["claude"]]; asked if >1
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
// QROUTON_ROOT / QROUTON_ORG override at runtime.
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
	if v := os.Getenv("QROUTON_ORG"); v != "" {
		cfg.Org = v
	}
	if len(cfg.Launch) == 0 {
		cfg.Launch = [][]string{{"claude"}}
	}
	cfg.Root = expandHome(cfg.Root)
	return cfg, os.MkdirAll(cfg.Root, 0o755)
}

func wizard() (*Config, error) {
	root, org := "~/work", "lifesum"
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Root directory").
			Description("Sessions live flat under it; repo mirrors under <root>/.mirrors").
			Value(&root),
		huh.NewInput().Title("GitHub org").
			Description("Org whose repos the session picker lists").
			Value(&org),
	)).Run()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Org: org, Root: root, Launch: [][]string{{"claude"}}}
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return cfg, os.WriteFile(configPath(), b, 0o644)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}
