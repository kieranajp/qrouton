package session

// The on-disk contract: the manifest schema and the reads and writes that
// maintain it. qrouton.json is what makes a directory a session and what every
// other process — the launcher, the window chrome, the escalate tool — polls, so
// the schema and its atomic write live together, apart from the assembly
// behaviour that produces them.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

const manifestName = sessionpaths.ManifestName

const manifestSchemaVersion = 2

type RepoRole string

const (
	RepoRoleActive    RepoRole = "active"
	RepoRoleReference RepoRole = "reference"
)

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
	SchemaVersion int                `json:"schemaVersion"`
	Name          string             `json:"name"`
	Slug          string             `json:"slug"`
	Description   string             `json:"description"`
	TicketURL     string             `json:"ticketUrl,omitempty"`
	Mode          SessionMode        `json:"mode,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
	Repos         []ManifestRepo     `json:"repos"`
	Escalation    *EscalationOutcome `json:"escalation,omitempty"`
}

// EscalationStatus is how a picker-driven escalation attempt ended.
type EscalationStatus string

const (
	EscalationConfirmed EscalationStatus = "confirmed"
	EscalationCancelled EscalationStatus = "cancelled"
)

// EscalationOutcome records the most recent escalation attempt. The picker
// writes it as part of its single atomic manifest write; the escalate MCP tool
// polls At to notice an outcome newer than the picker it spawned.
type EscalationOutcome struct {
	Status EscalationStatus `json:"status"`
	At     time.Time        `json:"at"`
}

// EffectiveMode is the session's runner mode, defaulting to RPI for manifests
// written before the field existed.
func (m Manifest) EffectiveMode() SessionMode { return m.Mode.effective() }

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
		var m Manifest
		if json.Unmarshal(b, &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// SetMode rewrites the manifest's mode — escalation writes rpi, de-escalation
// assistant — so the next launch stamps the new prompt instead of reverting.
func SetMode(dir string, mode SessionMode) error {
	m, err := Load(dir)
	if err != nil {
		return err
	}
	m.Mode = mode.effective()
	return WriteManifest(dir, m)
}

// Load reads one session directory's manifest.
func Load(dir string) (Manifest, error) {
	b, err := os.ReadFile(sessionpaths.Manifest(dir))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// WriteManifest atomically replaces a session's manifest — temp file plus
// rename — so pollers re-reading it every few seconds never see a torn write.
func WriteManifest(dir string, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	// Read before the write: it compares m against what is still on disk.
	handoff := escalatesToRPI(dir, m)
	tmp := sessionpaths.Manifest(dir) + manifestTmpSuffix
	if err := os.WriteFile(tmp, b, fileMode); err != nil {
		return err
	}
	if err := os.Rename(tmp, sessionpaths.Manifest(dir)); err != nil {
		return err
	}
	if handoff {
		// The session-private directory need not exist yet: a manifest can be
		// written before the launcher stages anything. Best-effort beyond that —
		// losing the marker costs the fresh context, not the escalation itself.
		_ = os.MkdirAll(sessionpaths.Dir(dir), dirMode)
		_ = os.WriteFile(sessionpaths.HandoffPending(dir), nil, fileMode)
	}
	return nil
}

// escalatesToRPI reports whether writing m turns an assistant session into an
// RPI one. That transition alone hands the next runner a fresh conversation, so
// it is marked here — the one place every mode change passes through, rather
// than in each caller that happens to set a mode.
func escalatesToRPI(dir string, m Manifest) bool {
	prev, err := Load(dir)
	return err == nil && prev.Mode.effective() == ModeAssistant && m.Mode.effective() == ModeRPI
}
