package desktop

import (
	"strings"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// heldRepo is one repository the session already holds, wearing the role it holds
// it in. A held row is not composed again — that would clone a repo the agent is
// working in a second time — but a reference one can be named in Upgrades.
type heldRepo struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Locked bool   `json:"locked"`
}

// requestedRow is one repository an agent asked for, classified against the
// manifest. Upgrade means the session already reads it and the tick will take it
// up for editing rather than acquire it.
type requestedRow struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Upgrade bool   `json:"upgrade"`
}

// pickerFields is what the picker draws itself from: the branch anything added
// joins, and the rows the session already holds. Branch is empty for a session
// with no repositories yet, which is the escalation that acquires its first ones.
// Requested and Reason are empty for a picker the user opened themselves, which
// is how the overlay knows not to claim an agent asked for anything.
type pickerFields struct {
	Branch    string         `json:"branch"`
	Repos     []heldRepo     `json:"repos"`
	Requested []requestedRow `json:"requested"`
	Reason    string         `json:"reason"`
}

// pickerInput is the picker's answer. Repos are rows to acquire; Upgrades name
// rows the session already reads, to be checked out for editing instead — the
// two reach the session by different routes, so they arrive apart.
type pickerInput struct {
	Repos    []repoPick `json:"repos"`
	Upgrades []string   `json:"upgrades"`
}

// Picker is the second step over a live session, serving the agent and header
// escalation actions as well as the add-repos button.
type Picker struct {
	cfg       *config.Config
	sessions  *Sessions
	repos     *Repositories
	assembler assembly.Assembler
}

func newPicker(cfg *config.Config, reg *Sessions, repos *Repositories, signal func(string)) *Picker {
	return &Picker{cfg: cfg, sessions: reg, repos: repos,
		assembler: assembly.Assembler{Cfg: cfg, Signal: signal}}
}

// Escalate opens a persistent escalation picker from the session header. An
// RPI session can race a stale page click and is already at the destination.
func (p *Picker) Escalate(slug string) error {
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	m, err := session.Load(root)
	if err != nil {
		return err
	}
	if m.EffectiveMode() != session.ModeAssistant {
		return nil
	}
	state.requestPicker(workbench.PickerRequest{SessionRoot: root, Kind: workbench.PickerKindEscalate})
	p.sessions.touch()
	return nil
}

func (p *Picker) Load(slug string) (pickerFields, error) {
	state, root, err := p.root(slug)
	if err != nil {
		return pickerFields{}, err
	}
	m, err := session.Load(root)
	if err != nil {
		return pickerFields{}, err
	}
	pending := state.pendingPicker()
	held := make([]heldRepo, 0, len(m.Repos))
	for _, r := range m.Repos {
		held = append(held, heldRepo{
			ID:     (github.Repo{Org: r.Org, Name: r.Name}).ID(),
			Role:   string(r.Role.Effective()),
			Locked: true,
		})
	}
	return pickerFields{Branch: m.Branch(), Repos: held,
		Requested: classifyRequests(m, p.repos.Cached(), pending), Reason: reasonOf(pending)}, nil
}

func reasonOf(req *workbench.PickerRequest) string {
	if req == nil {
		return ""
	}
	return strings.TrimSpace(req.Reason)
}

// classifyRequests answers each requested repository against the manifest as it
// stands now, not as it stood when the agent asked: a request can sit for half an
// hour while the user adds the very repository it wanted. A row the session
// already holds in the role asked for, or holds for editing at all, is dropped —
// there is nothing left for the tick to do. An id nothing knows survives as it
// was asked for, so the banner can still show the user what was missed.
func classifyRequests(m session.Manifest, cached []github.Repo, req *workbench.PickerRequest) []requestedRow {
	if req == nil {
		return []requestedRow{}
	}
	type holding struct {
		id   string
		role session.RepoRole
	}
	held := make(map[string]holding, len(m.Repos))
	for _, r := range m.Repos {
		id := (github.Repo{Org: r.Org, Name: r.Name}).ID()
		held[strings.ToLower(id)] = holding{id: id, role: r.Role.Effective()}
	}
	listed := make(map[string]string, len(cached))
	for _, repo := range cached {
		listed[strings.ToLower(repo.ID())] = repo.ID()
	}
	rows := make([]requestedRow, 0, len(req.Requested))
	// Once each: the overlay draws these keyed by id, so a repository named
	// twice in one request would collide there.
	seen := make(map[string]bool, len(req.Requested))
	for _, want := range req.Requested {
		id := strings.TrimSpace(want.ID)
		if id == "" || seen[strings.ToLower(id)] {
			continue
		}
		seen[strings.ToLower(id)] = true
		// Not Effective(): an empty role means editing for a row the session
		// holds, but a request that names none is only asking to read.
		role := session.RepoRoleReference
		if session.RepoRole(want.Role) == session.RepoRoleEditing {
			role = session.RepoRoleEditing
		}
		// A matched row travels on as the session or the list spells it:
		// everything downstream compares ids exactly, so the agent's casing
		// would tick a row nothing resolves and drop it again at confirm.
		switch on := held[strings.ToLower(id)]; on.role {
		case "":
			if canonical, ok := listed[strings.ToLower(id)]; ok {
				id = canonical
			}
			rows = append(rows, requestedRow{ID: id, Role: string(role)})
		case session.RepoRoleReference:
			if role == session.RepoRoleEditing {
				rows = append(rows, requestedRow{ID: on.id, Role: string(session.RepoRoleEditing), Upgrade: true})
			}
		}
	}
	return rows
}

func (p *Picker) Confirm(slug string, in pickerInput) error {
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	pending := state.pendingPicker()
	m, err := session.Load(root)
	if err != nil {
		return err
	}
	draft := p.draft(m, pending, in)
	if problems := assembly.CheckAdditions(m, draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	assembler := p.assembler
	assembler.Cfg = p.cfg.Snapshot()
	if err := assembler.Confirm(root, draft, answerTo(pending), nil); err != nil {
		return err
	}
	state.clearPicker()
	p.sessions.repositoriesChanged(root)
	return nil
}

func (p *Picker) Cancel(slug string) error {
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	if err := assembly.Cancel(root, answerTo(state.pendingPicker())); err != nil {
		return err
	}
	state.clearPicker()
	p.sessions.touch()
	return nil
}

// answerTo separates the two facts a pending request carries: something is
// polling for the outcome stanza, and only an escalation also moves the mode.
func answerTo(req *workbench.PickerRequest) assembly.Answer {
	return assembly.Answer{
		Escalating: req != nil && req.Kind == workbench.PickerKindEscalate,
		Awaited:    req != nil,
	}
}

// draft is the session's own description of itself plus the rows just picked: the
// picker has no name, ticket or mode field, so those come from the manifest. An
// escalation is the exception — it proposes a name for the work it is escalating
// to, and the prefix a first branch is cut with.
func (p *Picker) draft(m session.Manifest, pending *workbench.PickerRequest, in pickerInput) assembly.Draft {
	repos := p.repos.Select(in.Repos)
	name, prefix := m.DisplayName(), assembly.Prefixes()[0]
	upgrades := heldRefs(m, in.Upgrades)
	if pending != nil {
		if proposed := strings.TrimSpace(pending.Name); proposed != "" {
			name = proposed
		}
		if proposed := strings.TrimSpace(pending.Prefix); proposed != "" {
			prefix = proposed
		}
	}
	return assembly.Draft{Name: name, Description: m.Description, Ticket: m.TicketURL,
		Prefix: prefix, Mode: m.EffectiveMode(), Repos: repos, Upgrades: upgrades}
}

// heldRefs keeps the ids naming a reference repository the session holds, once
// each. They resolve against the manifest rather than the cached repository list,
// so a page a refresh behind still names the rows the session actually reads.
func heldRefs(m session.Manifest, ids []string) []session.RepoRef {
	byID := make(map[string]session.RepoRef, len(m.Repos))
	for _, r := range m.Repos {
		if r.Role.Effective() != session.RepoRoleReference {
			continue
		}
		byID[(github.Repo{Org: r.Org, Name: r.Name}).ID()] = session.RepoRef{Org: r.Org, Name: r.Name}
	}
	refs := make([]session.RepoRef, 0, len(ids))
	for _, id := range ids {
		if ref, ok := byID[id]; ok {
			refs = append(refs, ref)
			delete(byID, id)
		}
	}
	return refs
}

// root is the session the picker is about, which is only ever one this workbench
// is running.
func (p *Picker) root(slug string) (*sessionState, string, error) {
	state := p.sessions.bySlug(slug)
	root := state.root()
	if root == "" {
		return nil, "", unknownSession(slug)
	}
	return state, root, nil
}

// queuePicker records an agent's request on the session it names and raises
// nothing. The overlay opens when the user next arrives there, so a request they
// never arrive at expires unseen rather than taking the screen.
func (s *Sessions) queuePicker(req workbench.PickerRequest) error {
	slug := slugFor(req.SessionRoot)
	state := s.bySlug(slug)
	if state == nil {
		return unknownSession(slug)
	}
	state.requestPicker(req)
	s.touch()
	return nil
}
