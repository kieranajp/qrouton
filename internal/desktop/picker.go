package desktop

import (
	"context"
	"fmt"
	"strings"
	"sync"

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

// pickerFields is what the picker draws itself from: the branch anything added
// joins, and the rows the session already holds. Branch is empty for a session
// with no repositories yet, which is the escalation that acquires its first ones.
type pickerFields struct {
	Branch string     `json:"branch"`
	Repos  []heldRepo `json:"repos"`
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

	// mu serializes this picker's own two paths. Two of them at once could race on
	// a mirror or a worktree, and the second Refresh would cancel the first. It
	// says nothing about a new session being assembled beside them.
	mu sync.Mutex
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
	state.requestPicker(workbench.PickerRequest{SessionRoot: root})
	p.sessions.touch()
	return nil
}

func (p *Picker) Load(slug string) (pickerFields, error) {
	_, root, err := p.root(slug)
	if err != nil {
		return pickerFields{}, err
	}
	m, err := session.Load(root)
	if err != nil {
		return pickerFields{}, err
	}
	held := make([]heldRepo, 0, len(m.Repos))
	for _, r := range m.Repos {
		held = append(held, heldRepo{
			ID:     (github.Repo{Org: r.Org, Name: r.Name}).ID(),
			Role:   string(r.Role.Effective()),
			Locked: true,
		})
	}
	return pickerFields{Branch: m.Branch(), Repos: held}, nil
}

func (p *Picker) Confirm(slug string, in pickerInput) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	escalation := state.pendingPicker()
	m, err := session.Load(root)
	if err != nil {
		return err
	}
	draft := p.draft(m, escalation, in)
	if problems := assembly.CheckAdditions(m, draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	if err := p.assembler.Confirm(root, draft, escalation != nil, nil); err != nil {
		return err
	}
	state.clearPicker()
	p.sessions.repositoriesChanged(root)
	return nil
}

// addReposHook adapts the picker to the control socket. A workbench built
// without a picker serves no add rather than panicking on one.
func addReposHook(p *Picker) func(string, []repoAddition) (addReposResult, error) {
	if p == nil {
		return nil
	}
	return p.add
}

// addReposResult is what one agent-initiated add did. The three lists are
// disjoint: a repository was cloned, taken up, or already there. A successful add
// fills all three rather than leaving any nil, so the reply reads as empty lists
// and not as nulls.
type addReposResult struct {
	Added    []string
	Promoted []string
	Held     []string
}

// add is the unattended half of this picker: the agent names repositories and
// they are composed with no overlay and no human gate. It refuses the whole batch
// rather than part of it, so a mistyped name costs a clone nobody asked for.
func (p *Picker) add(slug string, additions []repoAddition) (addReposResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, root, err := p.root(slug)
	if err != nil {
		return addReposResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), repoRefreshTimeout)
	defer cancel()
	if err := p.repos.refreshAndWait(ctx); err != nil {
		return addReposResult{}, fmt.Errorf("%w: %w", ErrRepoRefreshFailed, err)
	}
	selections, err := p.repos.resolve(additions)
	if err != nil {
		return addReposResult{}, err
	}
	m, err := session.Load(root)
	if err != nil {
		return addReposResult{}, err
	}
	fresh, promote, held, err := partitionAdditions(m, selections)
	if err != nil {
		return addReposResult{}, err
	}
	outcome := addReposResult{Added: selectionIDs(fresh), Promoted: refIDs(promote), Held: held}
	// Nothing to compose and nothing to take up: the session already had every
	// repository named. Answering here keeps the call idempotent — CheckAdditions
	// judges an empty draft, and would refuse a no-op for holding no editing repo.
	if len(fresh) == 0 && len(promote) == 0 {
		return outcome, nil
	}
	draft := assembly.Draft{Name: m.DisplayName(), Description: m.Description, Ticket: m.TicketURL,
		Prefix: assembly.Prefixes()[0], Mode: m.EffectiveMode(), Repos: fresh, Upgrades: promote}
	if problems := assembly.CheckAdditions(m, draft); len(problems) > 0 {
		return addReposResult{}, draftRefused(problems[0])
	}
	if err := p.assembler.ConfirmForAgent(root, draft, nil); err != nil {
		return addReposResult{}, err
	}
	p.sessions.repositoriesChanged(root)
	return outcome, nil
}

// partitionAdditions is the promotion rule. A repository the session does not
// hold is composed; one it reads and is now asked to edit is taken up; anything
// else it already holds is left exactly as it is. That last case is what makes
// this promotion-only: a repo held for editing and asked for as reference lands
// there rather than being detached out from under uncommitted work.
//
// One repository named twice in two roles is refused rather than resolved. Acting
// on either would hand the agent a role it did not ask for, and a detached
// checkout it then commits into strands that work where nothing can reach it.
// Named twice in one role, it is acted on once — which is why the roles asked for
// and the names already handled are the same map.
func partitionAdditions(m session.Manifest, selections []session.RepoSelection) ([]session.RepoSelection, []session.RepoRef, []string, error) {
	byKey := make(map[string]session.ManifestRepo, len(m.Repos))
	for _, r := range m.Repos {
		byKey[repoKey(r.Org, r.Name)] = r
	}
	var fresh []session.RepoSelection
	var promote []session.RepoRef
	held := make([]string, 0, len(selections))
	asked := make(map[string]session.RepoRole, len(selections))
	for _, sel := range selections {
		key := repoKey(sel.Repo.Org, sel.Repo.Name)
		role := sel.Role.Effective()
		if previous, named := asked[key]; named {
			if previous != role {
				return nil, nil, nil, fmt.Errorf("%w: %s", ErrRepoRoleConflict,
					fmt.Sprintf(conflictingRoleFormat, sel.Repo.ID(), previous, role))
			}
			continue
		}
		asked[key] = role
		current, isHeld := byKey[key]
		switch {
		case !isHeld:
			fresh = append(fresh, sel)
		case current.Role.Effective() == session.RepoRoleReference && role == session.RepoRoleEditing:
			promote = append(promote, session.RepoRef{Org: current.Org, Name: current.Name})
		default:
			held = append(held, sel.Repo.ID())
		}
	}
	return fresh, promote, held, nil
}

func repoKey(org, name string) string {
	return strings.ToLower((github.Repo{Org: org, Name: name}).ID())
}

func selectionIDs(selections []session.RepoSelection) []string {
	ids := make([]string, 0, len(selections))
	for _, sel := range selections {
		ids = append(ids, sel.Repo.ID())
	}
	return ids
}

func refIDs(refs []session.RepoRef) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, (github.Repo{Org: ref.Org, Name: ref.Name}).ID())
	}
	return ids
}

func (p *Picker) Cancel(slug string) error {
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	if err := assembly.Cancel(root, state.pendingPicker() != nil); err != nil {
		return err
	}
	state.clearPicker()
	p.sessions.touch()
	return nil
}

// draft is the session's own description of itself plus the rows just picked: the
// picker has no name, ticket or mode field, so those come from the manifest. An
// escalation is the exception — it proposes a name for the work it is escalating
// to, and the prefix a first branch is cut with.
func (p *Picker) draft(m session.Manifest, escalation *workbench.PickerRequest, in pickerInput) assembly.Draft {
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

// queuePicker records an agent escalation on the session it names and raises
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
