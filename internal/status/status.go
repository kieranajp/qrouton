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
// Agents carries only the manifest provider until the workbench overlays live data.
type Fields struct {
	Mode     string `json:"mode"`
	Phase    string `json:"phase"`
	Identity string `json:"identity"`
	Branch   string `json:"branch"`
	// Slug and Terminal name the session on screen and the conversation the page
	// attaches to; the page has no other way to address either.
	Slug                string                `json:"slug"`
	Terminal            string                `json:"terminal"`
	Sessions            []SessionRow          `json:"sessions"`
	Documents           []Document            `json:"documents"`
	RepositoryDocuments []RepositoryDocuments `json:"repositoryDocuments"`
	Repos               []RepoStat            `json:"repos"`
	Activity            string                `json:"activity"`
	Agents              AgentPanel            `json:"agents"`
	// Picker means the shown session has an escalation waiting on it. It is
	// workbench-side knowledge, so a file read never sets it.
	Picker bool `json:"picker"`
	// Welcoming means this window is asking the first-run questions, which only a
	// window holding no session does. Workbench-side knowledge too, so a file read
	// never sets it.
	Welcoming bool `json:"welcoming"`
}

// EmptyFields is the value every producer starts from, and the one place a slice
// field of it is initialised. No slice may be nil: a nil one marshals as JSON
// null, and the page's defaults only fill keys the payload omits, so null reaches
// a .length and takes the whole window down with it.
func EmptyFields() Fields {
	return Fields{
		Sessions:            []SessionRow{},
		Documents:           []Document{},
		RepositoryDocuments: []RepositoryDocuments{},
		Repos:               []RepoStat{},
		Agents:              AgentPanel{Agents: []AgentRecord{}},
	}
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
	Summary  AgentSummary  `json:"summary"`
	Unseen   int           `json:"unseen"`
	Opened   time.Time     `json:"opened"`
}

type AgentSummary struct {
	Attention string `json:"attention"`
	Active    int    `json:"active"`
	Coverage  string `json:"coverage"`
	Running   bool   `json:"running"`
}

type AgentRecord struct {
	ID          string    `json:"id"`
	RunID       string    `json:"run_id"`
	Provider    string    `json:"provider"`
	ParentID    string    `json:"parent_id"`
	Type        string    `json:"type"`
	Role        string    `json:"role"`
	State       string    `json:"state"`
	ParentKnown bool      `json:"parent_known"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
}

type AgentPanel struct {
	Provider       string        `json:"provider"`
	AttentionKnown bool          `json:"attention_known"`
	ChildrenKnown  bool          `json:"children_known"`
	ParentsKnown   bool          `json:"parents_known"`
	OutcomesKnown  bool          `json:"outcomes_known"`
	Agents         []AgentRecord `json:"agents"`
}

// SessionRepo names one of a session's editing repositories in the rail.
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

// RepositoryDocuments is one session repository whose own thoughts directory
// contains durable artifacts.
type RepositoryDocuments struct {
	Name      string     `json:"name"`
	Documents []Document `json:"documents"`
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
	fields := EmptyFields()
	m, err := session.Load(root)
	if err != nil {
		return fields
	}
	fields.Mode = modeLabel(m)
	fields.Phase = phase(root, m)
	fields.Identity = m.DisplayName()
	fields.Branch = m.Branch()
	fields.Slug = m.Slug
	fields.Agents.Provider = m.Runner
	fields.Documents = documents(root)
	fields.RepositoryDocuments = repositoryDocuments(root, m.Repos)
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

func modeLabel(m session.Manifest) string {
	if m.EffectiveMode() == session.ModeRPI {
		return modeRPILabel
	}
	return modeAssistantLabel
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
		opened, _ := session.LastOpened(filepath.Join(root, m.Slug))
		rows = append(rows, SessionRow{
			Name: name, Slug: m.Slug, Initials: initials(name), Mode: modeLabel(m),
			Repos: sessionRepos(m), Opened: opened,
			Summary: AgentSummary{Attention: AgentAttentionNone, Coverage: AgentCoverageNone},
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
		if r.Role.Effective() != session.RepoRoleEditing {
			continue
		}
		out = append(out, SessionRepo{Name: r.Org + repoSeparator + r.Name, Role: string(session.RepoRoleEditing)})
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
	out := documentsUnder(root, dir, func(path string) string { return filepath.Base(path) })
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// repositoryDocuments lists repository-owned thoughts in manifest order. An
// empty thoughts directory has no row in the picker.
func repositoryDocuments(root string, repos []session.ManifestRepo) []RepositoryDocuments {
	out := []RepositoryDocuments{}
	src := sessionpaths.Src(root)
	for _, repo := range repos {
		if repo.WorktreePath == "" {
			continue
		}
		worktree := repo.WorktreePath
		if !filepath.IsAbs(worktree) {
			worktree = filepath.Join(root, worktree)
		}
		worktree = filepath.Clean(worktree)
		rel, err := filepath.Rel(src, worktree)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		dir := filepath.Join(worktree, sessionpaths.ThoughtsDirName)
		documents := documentsUnder(root, dir, func(path string) string {
			name, _ := filepath.Rel(dir, path)
			return filepath.ToSlash(name)
		})
		if len(documents) == 0 {
			continue
		}
		sort.Slice(documents, func(i, j int) bool { return documents[i].Name < documents[j].Name })
		out = append(out, RepositoryDocuments{Name: repo.Name, Documents: documents})
	}
	return out
}

func documentsUnder(root, dir string, name func(string) string) []Document {
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
			Name: name(path), Path: rel, Kind: DocumentKind(rel), At: info.ModTime(),
		})
		return nil
	})
	return out
}

// artifactKinds is the whole artifact taxonomy: the directory a kind is filed
// under, the label it carries, and the letter its filenames are numbered from.
// A new kind is this one entry; the lookups and the filename pattern derive.
var artifactKinds = []struct{ dir, kind, letter string }{
	{dir: "plans", kind: KindPlan, letter: "p"},
	{dir: "specs", kind: KindSpec, letter: "s"},
	{dir: "research", kind: KindResearch, letter: "r"},
	{dir: "explainers", kind: KindExplainer, letter: "e"},
}

var kindByDir, kindByLetter, artifactPrefix = indexArtifacts()

func indexArtifacts() (map[string]string, map[string]string, *regexp.Regexp) {
	dirs := make(map[string]string, len(artifactKinds))
	letters := make(map[string]string, len(artifactKinds))
	class := strings.Builder{}
	for _, a := range artifactKinds {
		dirs[a.dir] = a.kind
		letters[a.letter] = a.kind
		class.WriteString(a.letter)
	}
	return dirs, letters, regexp.MustCompile(`(?i)^(([` + class.String() + `])\d+)[-_.]`)
}

// DocumentKind reads the shared artifact taxonomy first and the workflow's
// filename prefix second. Anything outside both is a note.
func DocumentKind(path string) string {
	for _, part := range strings.Split(strings.ToLower(filepath.ToSlash(path)), "/") {
		if kind, ok := kindByDir[part]; ok {
			return kind
		}
	}
	match := artifactPrefix.FindStringSubmatch(filepath.Base(path))
	if match == nil {
		return KindNote
	}
	return kindByLetter[strings.ToLower(match[2])]
}

// ArtifactID is the numbered prefix a workflow artifact's filename opens with,
// uppercased, and empty for a filename that opens with anything else.
func ArtifactID(path string) string {
	match := artifactPrefix.FindStringSubmatch(filepath.Base(path))
	if match == nil {
		return ""
	}
	return strings.ToUpper(match[1])
}
