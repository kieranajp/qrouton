package core

import (
	"strings"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// HeldRepo is one repository the session already holds, wearing the role it holds
// it in. A held row is not composed again — that would clone a repo the agent is
// working in a second time — but a reference one can be named in Upgrades.
type HeldRepo struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	// Base is the branch this repository was cut from, absent for the default one.
	Base   string `json:"base,omitempty"`
	Locked bool   `json:"locked"`
}

// PickerFields is what the picker draws itself from: the branch anything added
// joins, and the rows the session already holds. Branch is empty for a session
// with no repositories yet, which is the escalation that acquires its first ones.
type PickerFields struct {
	Branch string     `json:"branch"`
	Repos  []HeldRepo `json:"repos"`
}

// PickerInput is the picker's answer. Repos are rows to acquire; Upgrades name
// rows the session already reads, to be checked out for editing instead — the
// two reach the session by different routes, so they arrive apart.
type PickerInput struct {
	Repos    []RepoPick `json:"repos"`
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
	state.RequestPicker(workbench.PickerRequest{SessionRoot: root})
	p.sessions.Touch()
	return nil
}

func (p *Picker) Load(slug string) (PickerFields, error) {
	_, root, err := p.root(slug)
	if err != nil {
		return PickerFields{}, err
	}
	m, err := session.Load(root)
	if err != nil {
		return PickerFields{}, err
	}
	held := make([]HeldRepo, 0, len(m.Repos))
	for _, r := range m.Repos {
		held = append(held, HeldRepo{
			ID:     (github.Repo{Org: r.Org, Name: r.Name}).ID(),
			Role:   string(r.Role.Effective()),
			Base:   r.BaseBranch,
			Locked: true,
		})
	}
	return PickerFields{Branch: m.Branch(), Repos: held}, nil
}

func (p *Picker) Confirm(slug string, in PickerInput) error {
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	escalation := state.PendingPicker()
	m, err := session.Load(root)
	if err != nil {
		return err
	}
	draft := p.draft(m, escalation, in)
	if problems := assembly.CheckAdditions(m, draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	assembler := p.assembler
	assembler.Cfg = p.cfg.Snapshot()
	if err := assembler.Confirm(root, draft, escalation != nil, nil); err != nil {
		return err
	}
	state.ClearPicker()
	p.sessions.RepositoriesChanged(root)
	return nil
}

func (p *Picker) Cancel(slug string) error {
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	if err := assembly.Cancel(root, state.PendingPicker() != nil); err != nil {
		return err
	}
	state.ClearPicker()
	p.sessions.Touch()
	return nil
}

// draft is the session's own description of itself plus the rows just picked: the
// picker has no name, ticket or mode field, so those come from the manifest. An
// escalation is the exception — it proposes a name for the work it is escalating
// to, and the prefix a first branch is cut with.
func (p *Picker) draft(m session.Manifest, escalation *workbench.PickerRequest, in PickerInput) assembly.Draft {
	repos := p.repos.Select(in.Repos)
	name, prefix := m.DisplayName(), assembly.Prefixes()[0]
	upgrades := heldRefs(m, in.Upgrades)
	if escalation != nil {
		if proposed := strings.TrimSpace(escalation.Name); proposed != "" {
			name = proposed
		}
		if proposed := strings.TrimSpace(escalation.Prefix); proposed != "" {
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

func (p *Picker) root(slug string) (*SessionState, string, error) {
	state := p.sessions.BySlug(slug)
	root := state.Root()
	if root == "" {
		return nil, "", UnknownSession(slug)
	}
	return state, root, nil
}
