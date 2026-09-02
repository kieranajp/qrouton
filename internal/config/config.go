package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kieranajp/qrouton/internal/atomicfile"
)

type Config struct {
	mu      *sync.RWMutex
	writeMu *sync.Mutex

	Orgs []string `json:"orgs"` // GitHub orgs for the repo picker
	Root string   `json:"root"` // sessions live flat under it; mirrors under <root>/.mirrors

	// Keyed by runner id rather than a bare argv list: with the runner buried in
	// argv[0], an override that dropped a flag looked like one that merely named
	// a runner.
	Launch map[string][]string `json:"launch,omitempty"`

	Editor []string `json:"editor,omitempty"`

	// Absent reads false, so a hand-written config sees the first-run flow once.
	Welcomed bool `json:"welcomed,omitempty"`

	StickerLabels *StickerLabels `json:"stickerLabels,omitempty"`
}

type StickerLabels struct {
	Star        string `json:"star"`
	Bookmark    string `json:"bookmark"`
	Question    string `json:"question"`
	Exclamation string `json:"exclamation"`
}

var DefaultStickerLabels = StickerLabels{
	Star:        "Important",
	Bookmark:    "Read later",
	Question:    "Needs follow-up",
	Exclamation: "Has bugs",
}

var zeroConfigMutex sync.RWMutex
var zeroConfigWriteMutex sync.Mutex

func (c *Config) mutex() *sync.RWMutex {
	if c.mu != nil {
		return c.mu
	}
	return &zeroConfigMutex
}

func (c *Config) writeMutex() *sync.Mutex {
	if c.writeMu != nil {
		return c.writeMu
	}
	return &zeroConfigWriteMutex
}

// Transact holds the config writer boundary while fn works from one snapshot.
func (c *Config) Transact(fn func(*Config) error) error {
	c.writeMutex().Lock()
	defer c.writeMutex().Unlock()
	return fn(c.Snapshot())
}

// Snapshot answers an independent generation of the complete config.
func (c *Config) Snapshot() *Config {
	if c == nil {
		return nil
	}
	c.mutex().RLock()
	defer c.mutex().RUnlock()
	return clone(c)
}

// Replace publishes every field from next as one live generation.
func (c *Config) Replace(next *Config) {
	replacement := next.Snapshot()
	c.mutex().Lock()
	defer c.mutex().Unlock()
	c.Orgs = replacement.Orgs
	c.Root = replacement.Root
	c.Launch = replacement.Launch
	c.Editor = replacement.Editor
	c.Welcomed = replacement.Welcomed
	c.StickerLabels = replacement.StickerLabels
}

func clone(c *Config) *Config {
	out := &Config{
		mu:       &sync.RWMutex{},
		writeMu:  &sync.Mutex{},
		Orgs:     append([]string(nil), c.Orgs...),
		Root:     c.Root,
		Editor:   append([]string(nil), c.Editor...),
		Welcomed: c.Welcomed,
	}
	if c.Launch != nil {
		out.Launch = make(map[string][]string, len(c.Launch))
		for runner, argv := range c.Launch {
			out.Launch[runner] = append([]string(nil), argv...)
		}
	}
	if c.StickerLabels != nil {
		labels := *c.StickerLabels
		out.StickerLabels = &labels
	}
	return out
}

func (c *Config) EffectiveStickerLabels() StickerLabels {
	return effectiveStickerLabels(c.Snapshot())
}

func effectiveStickerLabels(c *Config) StickerLabels {
	labels := DefaultStickerLabels
	if c.StickerLabels == nil {
		return labels
	}
	if c.StickerLabels.Star != "" {
		labels.Star = c.StickerLabels.Star
	}
	if c.StickerLabels.Bookmark != "" {
		labels.Bookmark = c.StickerLabels.Bookmark
	}
	if c.StickerLabels.Question != "" {
		labels.Question = c.StickerLabels.Question
	}
	if c.StickerLabels.Exclamation != "" {
		labels.Exclamation = c.StickerLabels.Exclamation
	}
	return labels
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
	cfg := &Config{mu: &sync.RWMutex{}, writeMu: &sync.Mutex{}}
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

func Save(cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(Path()), dirMode); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg.Snapshot(), "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Replace(Path(), b, fileMode)
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
