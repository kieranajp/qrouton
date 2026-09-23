package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/vault"
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

	Thoughts Thoughts `json:"thoughts,omitzero"`
}

// Thoughts names where sessions write their documents. Default is private.
type Thoughts struct {
	Default string         `json:"default,omitempty"`
	Roots   []ThoughtsRoot `json:"roots,omitempty"`
}

type ThoughtsRoot struct {
	ID   string   `json:"id"`
	Path string   `json:"path"`
	Orgs []string `json:"orgs,omitempty"`
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
	c.Thoughts = replacement.Thoughts
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
	out.Thoughts = Thoughts{Default: c.Thoughts.Default}
	for _, r := range c.Thoughts.Roots {
		out.Thoughts.Roots = append(out.Thoughts.Roots, ThoughtsRoot{ID: r.ID, Path: r.Path, Orgs: slices.Clone(r.Orgs)})
	}
	return out
}

// ThoughtsRoots lists the default root first, at <root>/thoughts unless set.
func (c *Config) ThoughtsRoots() []ThoughtsRoot {
	snapshot := c.Snapshot()
	def := snapshot.Thoughts.Default
	if strings.TrimSpace(def) == "" {
		def = filepath.Join(snapshot.Root, sessionpaths.ThoughtsDirName)
	}
	return append([]ThoughtsRoot{{ID: DefaultThoughtsID, Path: def}}, snapshot.Thoughts.Roots...)
}

// RouteThoughts answers a mapped root only when every org maps to it.
func (c *Config) RouteThoughts(orgs []string) ThoughtsRoot {
	roots := c.ThoughtsRoots()
	routed := -1
	for _, org := range orgs {
		i := slices.IndexFunc(roots, func(r ThoughtsRoot) bool {
			return slices.ContainsFunc(r.Orgs, func(o string) bool { return strings.EqualFold(strings.TrimSpace(o), strings.TrimSpace(org)) })
		})
		if i < 0 || routed >= 0 && i != routed {
			return roots[0]
		}
		routed = i
	}
	return roots[max(routed, 0)]
}

func validateThoughts(roots []ThoughtsRoot) error {
	profiles := make([]vault.Profile, len(roots))
	owners := map[string]string{}
	for i, r := range roots {
		if i > 0 && (strings.EqualFold(r.ID, DefaultThoughtsID) || len(r.Orgs) == 0) {
			return fmt.Errorf("%w: %q", ErrThoughtsRoot, r.ID)
		}
		for _, org := range r.Orgs {
			key := strings.ToLower(strings.TrimSpace(org))
			if key == "" {
				return fmt.Errorf("%w: %q", ErrThoughtsRoot, r.ID)
			}
			if owner, ok := owners[key]; ok {
				return fmt.Errorf("%w: %s on %q and %q", ErrThoughtsOrg, org, owner, r.ID)
			}
			owners[key] = r.ID
		}
		profiles[i] = vault.Profile{ID: r.ID, Root: r.Path}
	}
	return vault.Validate(profiles)
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
	cfg.Thoughts.Default = expandHome(cfg.Thoughts.Default)
	for i := range cfg.Thoughts.Roots {
		cfg.Thoughts.Roots[i].Path = expandHome(cfg.Thoughts.Roots[i].Path)
	}
	if err := validateThoughts(cfg.ThoughtsRoots()); err != nil {
		return nil, fmt.Errorf("%s: %w", Path(), err)
	}
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
