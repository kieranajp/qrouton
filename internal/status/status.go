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
// leaves Repos, Activity and Sessions empty: the first costs subprocesses, the
// second needs a live PTY, and the last is the app's rather than a session's.
//
// No slice here may be nil. A nil one marshals as JSON null, and the page's
// defaults only fill keys the payload omits, so null reaches a .length and
// takes the whole window down with it.
type Fields struct {
	Mode     string `json:"mode"`
	Phase    string `json:"phase"`
	Identity string `json:"identity"`
	Branch   string `json:"branch"`
	// Slug and Terminal name the session on screen and the conversation the page
	// attaches to; the page has no other way to address either.
	Slug      string       `json:"slug"`
	Terminal  string       `json:"terminal"`
	Sessions  []SessionRow `json:"sessions"`
	Documents []Document   `json:"documents"`
	Repos     []RepoStat   `json:"repos"`
	Activity  string       `json:"activity"`
	// Picker means the shown session has an escalation waiting on it. It is
	// workbench-side knowledge, so a file read never sets it.
	Picker bool `json:"picker"`
	// Welcoming means this window is asking the first-run questions, which only a
	// window holding no session does. Workbench-side knowledge too, so a file read
	// never sets it.
	Welcoming bool `json:"welcoming"`
}

// SessionRow is one session under the sessions root. A Terminal means this
// workbench holds a conversation for it; Activity and Unseen are all a row claims
// about a session that is not on screen.
type SessionRow struct {
	Name     string        `json:"name"`
	Slug     string        `json:"slug"`
	Initials string        `json:"initials"`
	Mode     string        `json:"mode"`
	Repos    []SessionRepo `json:"repos"`
	Terminal string        `json:"terminal"`
	Activity string        `json:"activity"`
	Unseen   int           `json:"unseen"`
	Opened   time.Time     `json:"opened"`
}

// SessionRepo names one of a session's repositories, which is what tells two
// sessions apart in the rail.
type SessionRepo struct {
	Name string `json:"name"`
	Role string `json:"role"`
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

// Read reports everything a file read can answer about a session. A root with no
// manifest answers with the session-level fields empty rather than nothing at all.
func Read(root string) Fields {
	fields := Fields{Sessions: []SessionRow{}, Documents: []Document{}, Repos: []RepoStat{}}
	m, err := session.Load(root)
	if err != nil {
		return fields
	}
	fields.Mode = modeAssistantLabel
	if m.EffectiveMode() == session.ModeRPI {
		fields.Mode = modeRPILabel
	}
	fields.Phase = phase(root, m)
	fields.Identity = m.DisplayName()
	fields.Branch = m.Branch()
	fields.Slug = m.Slug
	fields.Documents = documents(root)
	return fields
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
		out = append(out, RepoStat{
			Name:       stat.Org + repoSeparator + stat.Name,
			Role:       string(stat.Role),
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

// Sessions lists every session under the sessions root, most recently opened
// first and never-opened ones last by name. That order seeds the rail once; what
// the rail then draws is the caller's, because a row's position addresses it.
func Sessions(root string) []SessionRow {
	found, err := session.Scan(root)
	if err != nil {
		return []SessionRow{}
	}
	rows := make([]SessionRow, 0, len(found))
	for _, m := range found {
		name := m.DisplayName()
		mode := modeAssistantLabel
		if m.EffectiveMode() == session.ModeRPI {
			mode = modeRPILabel
		}
		opened, _ := session.LastOpened(filepath.Join(root, m.Slug))
		rows = append(rows, SessionRow{
			Name: name, Slug: m.Slug, Initials: initials(name), Mode: mode,
			Repos: sessionRepos(m), Opened: opened,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].Opened.Equal(rows[j].Opened) {
			return rows[i].Opened.After(rows[j].Opened)
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

// Unseen counts the documents each session has written since it was last shown.
// One never opened reports none: everything it holds would otherwise count.
func Unseen(root string) map[string]int {
	found, err := session.Scan(root)
	if err != nil {
		return map[string]int{}
	}
	out := make(map[string]int, len(found))
	for _, m := range found {
		if count, shown := UnseenIn(filepath.Join(root, m.Slug)); shown {
			out[m.Slug] = count
		}
	}
	return out
}

// UnseenIn counts one session's documents written since it was last shown, and
// reports false for a session never shown. Arriving at a session recounts it
// without rereading every other one's tree.
func UnseenIn(sessionRoot string) (int, bool) {
	since, ok := session.LastOpened(sessionRoot)
	if !ok {
		return 0, false
	}
	return writtenSince(sessionRoot, since), true
}

func writtenSince(root string, since time.Time) int {
	count := 0
	_ = filepath.WalkDir(sessionpaths.Thoughts(root), func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), markdownSuffix) {
			return nil
		}
		if info, err := entry.Info(); err == nil && info.ModTime().After(since) {
			count++
		}
		return nil
	})
	return count
}

// sessionRepos names a session's repositories in the order they were picked,
// which is the order the manifest holds them in.
func sessionRepos(m session.Manifest) []SessionRepo {
	out := make([]SessionRepo, 0, len(m.Repos))
	for _, r := range m.Repos {
		out = append(out, SessionRepo{Name: r.Name, Role: string(r.Role.Effective())})
	}
	return out
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
			Name: entry.Name(), Path: rel, Kind: DocumentKind(rel), At: info.ModTime(),
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// DocumentKind reads the shared artifact taxonomy first and the workflow's
// filename prefix second. Anything outside both is a note.
func DocumentKind(path string) string {
	for _, part := range strings.Split(strings.ToLower(filepath.ToSlash(path)), "/") {
		switch part {
		case "plans":
			return KindPlan
		case "specs":
			return KindSpec
		case "research":
			return KindResearch
		case "explainers":
			return KindExplainer
		}
	}
	name := filepath.Base(path)
	switch {
	case planPrefix.MatchString(name):
		return KindPlan
	case specPrefix.MatchString(name):
		return KindSpec
	case researchPrefix.MatchString(name):
		return KindResearch
	case explainerPrefix.MatchString(name):
		return KindExplainer
	default:
		return KindNote
	}
}

var (
	planPrefix      = regexp.MustCompile(`(?i)^p\d+[-_.]`)
	specPrefix      = regexp.MustCompile(`(?i)^s\d+[-_.]`)
	researchPrefix  = regexp.MustCompile(`(?i)^r\d+[-_.]`)
	explainerPrefix = regexp.MustCompile(`(?i)^e\d+[-_.]`)
)
