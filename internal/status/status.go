// Package status derives what the workbench window can say about a session.
// Every field comes from the manifest, from disk or from git; nothing here
// infers what the agent is doing.
package status

import (
	"context"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// Fields is the session the workbench window draws around its terminal. Read
// leaves Repos and Activity empty: one costs subprocesses, the other needs a
// live PTY, so their owners fill them.
//
// No slice here may be nil. A nil one marshals as JSON null, and the page's
// defaults only fill keys the payload omits, so null reaches a .length and
// takes the whole window down with it.
type Fields struct {
	Mode      string       `json:"mode"`
	Phase     string       `json:"phase"`
	Identity  string       `json:"identity"`
	Branch    string       `json:"branch"`
	Sessions  []SessionRow `json:"sessions"`
	Documents []Document   `json:"documents"`
	Repos     []RepoStat   `json:"repos"`
	Activity  string       `json:"activity"`
}

// SessionRow is one session under the same root; Live marks this process's own.
type SessionRow struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Initials string `json:"initials"`
	Mode     string `json:"mode"`
	Repos    int    `json:"repos"`
	Live     bool   `json:"live"`
}

// Document is one durable artifact under thoughts/shared, its Path relative to
// the session root.
type Document struct {
	Name string    `json:"name"`
	Path string    `json:"path"`
	Kind string    `json:"kind"`
	At   time.Time `json:"at"`
}

// RepoStat is how far one repository has moved, in the window's vocabulary.
type RepoStat struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Commits    int    `json:"commits"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
	Measured   bool   `json:"measured"`
}

// Read reports everything a file read can answer, or false when the manifest
// cannot be loaded — the caller decides what to render in its place.
func Read(root string) (Fields, bool) {
	m, err := session.Load(root)
	if err != nil {
		return Fields{}, false
	}
	fields := Fields{
		Mode:      modeAssistantLabel,
		Phase:     phase(root, m),
		Identity:  displayName(m),
		Branch:    m.Branch(),
		Sessions:  sessions(root),
		Documents: documents(root),
	}
	if m.EffectiveMode() == session.ModeRPI {
		fields.Mode = modeRPILabel
	}
	return fields, true
}

// displayName is the one owner of a session's human name: its Name if set,
// else its Slug.
func displayName(m session.Manifest) string {
	if m.Name != "" {
		return m.Name
	}
	return m.Slug
}

// Repos measures the session's repositories against the branches they were cut
// from. Apart from Read because it shells git, and so polls more slowly.
func Repos(ctx context.Context, root string) []RepoStat {
	m, err := session.Load(root)
	if err != nil {
		return []RepoStat{}
	}
	out := make([]RepoStat, 0, len(m.Repos))
	for _, stat := range session.RepoStats(ctx, filepath.Dir(root), m) {
		role := roleEditing
		if stat.Role == session.RepoRoleReference {
			role = roleReference
		}
		out = append(out, RepoStat{
			Name:       stat.Org + repoSeparator + stat.Name,
			Role:       role,
			Commits:    stat.Commits,
			Insertions: stat.Insertions,
			Deletions:  stat.Deletions,
			Measured:   stat.Measured,
		})
	}
	return out
}

// phase is the macro-phase only: scratch until repositories exist, then the
// stage the session's durable documents put it in — a research doc means
// planning, a plan means implementing.
func phase(root string, m session.Manifest) string {
	if len(m.Repos) == 0 {
		return phaseScratch
	}
	ws := session.Status(filepath.Dir(root), m)
	switch {
	case ws.Plan:
		return phaseImplement
	case ws.Research:
		return phasePlan
	default:
		return phaseResearch
	}
}

// sessions lists every session under the same root, the live one first. A rail
// row is the only way a session that is not on screen can speak.
func sessions(root string) []SessionRow {
	found, err := session.Scan(filepath.Dir(root))
	if err != nil {
		return []SessionRow{}
	}
	live := filepath.Base(root)
	rows := make([]SessionRow, 0, len(found))
	for _, m := range found {
		name := displayName(m)
		mode := modeAssistantLabel
		if m.EffectiveMode() == session.ModeRPI {
			mode = modeRPILabel
		}
		rows = append(rows, SessionRow{
			Name: name, Slug: m.Slug, Initials: initials(name), Mode: mode,
			Repos: len(m.Repos), Live: m.Slug == live,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Live != rows[j].Live {
			return rows[i].Live
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

// initials abbreviates a session to the two letters a rail row has room for.
func initials(name string) string {
	words := strings.Fields(nonLetter.ReplaceAllString(strings.ToLower(name), fieldSeparator))
	switch len(words) {
	case 0:
		return ""
	case 1:
		if len(words[0]) == 1 {
			return words[0]
		}
		return words[0][:2]
	default:
		return words[0][:1] + words[1][:1]
	}
}

var nonLetter = regexp.MustCompile(`[^a-z0-9]+`)

// documents lists what the session has written, newest first.
func documents(root string) []Document {
	dir := sessionpaths.Thoughts(root)
	out := []Document{}
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), markdownSuffix) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		out = append(out, Document{
			Name: entry.Name(), Path: rel, Kind: kind(entry.Name()), At: info.ModTime(),
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// kind reads the filename prefix the workflow already stamps; anything else is
// a note, which is what an unprefixed document is.
func kind(name string) string {
	switch {
	case planPrefix.MatchString(name):
		return kindPlan
	case specPrefix.MatchString(name):
		return kindSpec
	case researchPrefix.MatchString(name):
		return kindResearch
	default:
		return kindNote
	}
}

var (
	planPrefix     = regexp.MustCompile(`(?i)^p\d+[-_.]`)
	specPrefix     = regexp.MustCompile(`(?i)^s\d+[-_.]`)
	researchPrefix = regexp.MustCompile(`(?i)^r\d+[-_.]`)
)
