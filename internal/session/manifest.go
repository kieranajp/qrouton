package session

// The on-disk contract: the manifest schema and the reads and writes that
// maintain it. qrouton.json is what makes a directory a session and what every
// other process — the launcher, the window chrome, the escalate tool — polls, so
// the schema and the serialised write that maintains it live together, apart
// from the assembly behaviour that produces them.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

const manifestName = sessionpaths.ManifestName

const manifestSchemaVersion = 2

type RepoRole string

const (
	RepoRoleEditing   RepoRole = "editing"
	RepoRoleReference RepoRole = "reference"
)

// Effective reads an unset role as editing. Assembly always names a role, so an
// empty one means a hand-edited manifest and editing is the safe reading.
func (r RepoRole) Effective() RepoRole {
	if r == "" {
		return RepoRoleEditing
	}
	return r
}

func (r RepoRole) IsEditing() bool { return r.Effective() == RepoRoleEditing }

type Sticker string

const (
	StickerStar        Sticker = "star"
	StickerBookmark    Sticker = "bookmark"
	StickerQuestion    Sticker = "question"
	StickerExclamation Sticker = "exclamation"
)

func (s Sticker) Effective() Sticker {
	switch s {
	case StickerStar, StickerBookmark, StickerQuestion, StickerExclamation:
		return s
	default:
		return ""
	}
}

func (s Sticker) Next() Sticker {
	switch s.Effective() {
	case "":
		return StickerStar
	case StickerStar:
		return StickerBookmark
	case StickerBookmark:
		return StickerQuestion
	case StickerQuestion:
		return StickerExclamation
	default:
		return ""
	}
}

// SessionMode selects the system prompt (and opening message) the runner starts
// under. RPI is the default orchestrated Research→Plan→Implement workflow;
// Assistant is a lighter, open-ended coding session that can escalate to RPI
// on request. Both modes stamp the same prompts, skills, and MCP tools.
type SessionMode string

const (
	ModeRPI       SessionMode = "rpi"
	ModeAssistant SessionMode = "assistant"
)

// effective treats an unset or unknown mode as RPI, keeping manifests written
// before the field existed on the default workflow.
func (m SessionMode) effective() SessionMode {
	if m == ModeAssistant {
		return ModeAssistant
	}
	return ModeRPI
}

// RepoSelection pairs repository metadata with its role in a session.
type RepoSelection struct {
	Repo github.Repo
	Role RepoRole
}

type ProgressStep string
type ProgressStatus string

const (
	ProgressMirror   ProgressStep = "mirror"
	ProgressWorktree ProgressStep = "worktree"
	ProgressScaffold ProgressStep = "scaffold"
	ProgressManifest ProgressStep = "manifest"

	ProgressStarted   ProgressStatus = "started"
	ProgressCompleted ProgressStatus = "completed"
	ProgressFailed    ProgressStatus = "failed"

	// ProgressAdvanced is an update within a step that is still running — git's
	// own clone and fetch progress. Consumers that only report outcomes ignore
	// it; the assembly screen draws it as a bar.
	ProgressAdvanced ProgressStatus = "advanced"
)

type Progress struct {
	Step   ProgressStep
	Status ProgressStatus
	Repo   *github.Repo
	Role   RepoRole
	Err    error

	// Phase and Percent carry git's own reckoning on a ProgressAdvanced event,
	// e.g. "Receiving objects" at 47.
	Phase   string
	Percent int
}

type ProgressFunc func(Progress)

type Manifest struct {
	SchemaVersion int         `json:"schemaVersion"`
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	Description   string      `json:"description"`
	TicketURL     string      `json:"ticketUrl,omitempty"`
	Mode          SessionMode `json:"mode,omitempty"`
	Sticker       Sticker     `json:"sticker,omitempty"`
	// Runner is the coding agent this session was assembled with, so every later
	// boot starts the one that was chosen rather than the workbench's default.
	Runner    string         `json:"runner,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	Repos     []ManifestRepo `json:"repos"`
	Picker    *PickerOutcome `json:"picker,omitempty"`
}

// PickerStatus is how a picker a Go-side request was awaiting ended. Readers
// match Confirmed explicitly: everything else, a zero value included, is
// not-confirmed.
type PickerStatus string

const (
	PickerConfirmed PickerStatus = "confirmed"
	PickerCancelled PickerStatus = "cancelled"
)

// PickerOutcome records the most recent awaited picker. The picker writes it as
// part of its single atomic manifest write, so the Repos a poller reads
// alongside a fresh stanza are the set the user confirmed; the blocked MCP tool
// polls At to notice an outcome newer than the picker it spawned. A picker the
// user opened themselves records nothing — nothing is awaiting it.
type PickerOutcome struct {
	Status PickerStatus `json:"status"`
	At     time.Time    `json:"at"`
}

// EffectiveMode is the session's runner mode, defaulting to RPI for manifests
// written before the field existed.
func (m Manifest) EffectiveMode() SessionMode { return m.Mode.effective() }

// DisplayName is what a session is called: its Name, or its Slug for one written
// before it had a name.
func (m Manifest) DisplayName() string {
	if m.Name != "" {
		return m.Name
	}
	return m.Slug
}

// Branch is the session branch: the first editing repo's branch.
func (m Manifest) Branch() string {
	for _, r := range m.Repos {
		if r.Branch != "" {
			return r.Branch
		}
	}
	return ""
}

type ManifestRepo struct {
	Name          string   `json:"name"`
	Org           string   `json:"org"`
	Role          RepoRole `json:"role,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	DefaultBranch string   `json:"defaultBranch,omitempty"`
	Revision      string   `json:"revision,omitempty"`
	WorktreePath  string   `json:"worktreePath"`
	SSHURL        string   `json:"sshUrl,omitempty"` // clone URL for mirror re-creation on resume
}

// Scan: a session is any direct child of root containing a qrouton.json.
func Scan(root string) ([]Manifest, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name(), manifestName))
		if err != nil {
			continue
		}
		if m, err := decode(b); err == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// decode is the one reader of the on-disk document, so a manifest a session
// cannot be resumed from never appears in a listing either.
func decode(b []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	for _, r := range m.Repos {
		if role := r.Role.Effective(); role != RepoRoleEditing && role != RepoRoleReference {
			return Manifest{}, invalidRole(r.Role, r.Org, r.Name)
		}
	}
	return m, nil
}

// SetMode rewrites the manifest's mode — escalation writes rpi, de-escalation
// assistant — so the next launch stamps the new prompt instead of reverting.
func SetMode(dir string, mode SessionMode) error {
	return UpdateManifest(dir, func(m Manifest) (Manifest, error) {
		m.Mode = mode.effective()
		return m, nil
	})
}

func CycleSticker(dir string) (Sticker, error) {
	var committed Sticker
	err := UpdateManifest(dir, func(m Manifest) (Manifest, error) {
		m.Sticker = m.Sticker.Next()
		committed = m.Sticker
		return m, nil
	})
	return committed, err
}

// Load reads one session directory's manifest.
func Load(dir string) (Manifest, error) {
	b, err := os.ReadFile(sessionpaths.Manifest(dir))
	if err != nil {
		return Manifest{}, err
	}
	return decode(b)
}

// UpdateManifest applies mutate to a session's manifest under a lock the other
// writing processes take too, so a load, an edit and the replace that follows
// it are one step rather than three a concurrent writer can interleave with.
func UpdateManifest(dir string, mutate func(Manifest) (Manifest, error)) error {
	return withManifestLock(dir, func() error {
		prev, err := Load(dir)
		if err != nil {
			return err
		}
		m, err := mutate(prev)
		if err != nil {
			return err
		}
		return writeManifest(dir, prev, m)
	})
}

// WriteManifest replaces a session's manifest with one composed from nothing on
// disk. Changing a manifest that already exists goes through UpdateManifest.
func WriteManifest(dir string, m Manifest) error {
	prev, _ := Load(dir)
	return writeManifest(dir, prev, m)
}

func writeManifest(dir string, prev, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.Replace(sessionpaths.Manifest(dir), b, fileMode); err != nil {
		return err
	}
	if escalatesToRPI(prev, m) {
		// The session-private directory need not exist yet: a manifest can be
		// written before the launcher stages anything. Best-effort beyond that —
		// losing the marker costs the fresh context, not the escalation itself.
		_ = os.MkdirAll(sessionpaths.Dir(dir), dirMode)
		_ = os.WriteFile(sessionpaths.HandoffPending(dir), nil, fileMode)
	}
	return nil
}

// withManifestLock serialises a read-modify-write against the desktop, the mode
// verb and assembly, which are separate processes editing the same file.
func withManifestLock(dir string, fn func() error) error {
	if err := os.MkdirAll(sessionpaths.Dir(dir), dirMode); err != nil {
		return err
	}
	return atomicfile.WithLock(sessionpaths.ManifestLock(dir), fileMode, fn)
}

// escalatesToRPI reports whether replacing prev with m turns an assistant
// session into an RPI one. That transition alone hands the next runner a fresh
// conversation, so it is marked here — the one place every mode change passes
// through, rather than in each caller that happens to set a mode.
func escalatesToRPI(prev, m Manifest) bool {
	return prev.Mode.effective() == ModeAssistant && m.Mode.effective() == ModeRPI
}
