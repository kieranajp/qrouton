package desktop

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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
	// Proposed are the repositories an agent asked for, to be drawn pre-selected
	// at the roles it wants. Empty for the user's own visit to the picker, which
	// is what tells the page whose selection it is showing.
	Proposed []proposedRepo `json:"proposed"`
}

// proposedRepo is one row of an agent's proposal, keyed the way the page keys its
// rows so a pre-selection lands on the row it names.
type proposedRepo struct {
	ID   string `json:"id"`
	Role string `json:"role"`
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
	state, root, err := p.root(slug)
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
	proposed := make([]proposedRepo, 0, len(state.proposedRepos()))
	for _, row := range state.proposedRepos() {
		proposed = append(proposed, proposedRepo{ID: row.Name, Role: row.Role})
	}
	return pickerFields{Branch: m.Branch(), Repos: held, Proposed: proposed}, nil
}

func (p *Picker) Confirm(slug string, in pickerInput) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	state, root, err := p.root(slug)
	if err != nil {
		return err
	}
	pending := state.pendingPicker()
	// A pending request carrying repositories is an agent's proposal, not an
	// escalation. Reading it as one would switch the session to RPI mode because
	// the agent asked for a checkout.
	proposal := pending != nil && len(pending.Repos) > 0
	var asked []workbench.RepoAddition
	if proposal {
		asked = pending.Repos
	}
	before, err := session.Load(root)
	if err == nil {
		err = p.compose(root, before, pending, proposal, in)
	}
	// Whatever happened, an agent blocked on this proposal is told — including a
	// refusal, or it would wait out its whole deadline for an answer that already
	// exists. The user who pressed the button still sees the error too.
	if proposal {
		p.answerProposal(state, root, before, asked, err)
	}
	if err != nil {
		return err
	}
	state.clearPicker()
	p.sessions.repositoriesChanged(root)
	return nil
}

// compose validates the draft and lands it. A proposal goes through
// ConfirmForAgent: the agent is blocked on this decision, so signalling would
// SIGTERM the process waiting for it, and a notice would tell the user what they
// just confirmed on screen.
func (p *Picker) compose(root string, before session.Manifest,
	pending *workbench.PickerRequest, proposal bool, in pickerInput) error {
	draft := p.draft(before, pending, in)
	if proposal {
		// A proposal of only what the session already holds asks for nothing, so
		// there is nothing for the editing-repo rule to judge and nothing to write.
		// Refusing it would make re-proposing a held repository an error, which is
		// the retry an agent reaches for after a partial failure.
		if len(draft.Repos) == 0 && len(draft.Upgrades) == 0 {
			return nil
		}
	}
	if problems := assembly.CheckAdditions(before, draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	if proposal {
		return p.assembler.ConfirmForAgent(root, draft, nil)
	}
	return p.assembler.Confirm(root, draft, pending != nil, nil)
}

// answerProposal delivers a proposal's outcome to the agent blocked on it. A
// compose that failed travels as the error it was, so the agent gets the refusal
// rather than an empty result that reads as "nothing to do".
func (p *Picker) answerProposal(state *sessionState, root string,
	before session.Manifest, asked []workbench.RepoAddition, composeErr error) {
	answer := state.takeAnswer()
	if answer == nil {
		return
	}
	if composeErr != nil {
		answer <- addReposDecision{Err: composeErr}
		return
	}
	after, err := session.Load(root)
	if err != nil {
		answer <- addReposDecision{Err: err}
		return
	}
	answer <- addReposDecision{
		Status:  workbench.AddReposConfirmed,
		Outcome: addReposOutcome(asked, before, after),
	}
}

// addReposOutcome is the session diffed across the user's answer. Added is
// everything newly present, the user's own choices included, because that is what
// the workspace now holds; Dropped is what the agent asked for and did not get.
func addReposOutcome(asked []workbench.RepoAddition, before, after session.Manifest) addReposResult {
	was := rolesByKey(before)
	now := rolesByKey(after)
	out := addReposResult{
		Status:   workbench.AddReposConfirmed,
		Added:    []string{},
		Promoted: []string{},
		Held:     []string{},
		Dropped:  []string{},
	}
	for _, r := range after.Repos {
		key := repoKey(r.Org, r.Name)
		previous, existed := was[key]
		id := (github.Repo{Org: r.Org, Name: r.Name}).ID()
		switch {
		case !existed:
			out.Added = append(out.Added, id)
		case previous == session.RepoRoleReference && r.Role.Effective() == session.RepoRoleEditing:
			out.Promoted = append(out.Promoted, id)
		}
	}
	for _, want := range asked {
		key := strings.ToLower(want.Name)
		if _, present := now[key]; !present {
			out.Dropped = append(out.Dropped, want.Name)
			continue
		}
		// Present and neither added nor promoted by this confirm: the session
		// already held it in the role asked for.
		if was[key] == now[key] {
			out.Held = append(out.Held, want.Name)
		}
	}
	return out
}

func rolesByKey(m session.Manifest) map[string]session.RepoRole {
	roles := make(map[string]session.RepoRole, len(m.Repos))
	for _, r := range m.Repos {
		roles[repoKey(r.Org, r.Name)] = r.Role.Effective()
	}
	return roles
}

// addReposHook adapts the picker to the control socket. A workbench built
// without a picker serves no add rather than panicking on one.
func addReposHook(p *Picker) func(context.Context, string, []repoAddition, time.Time) (addReposResult, error) {
	if p == nil {
		return nil
	}
	return p.add
}

// addReposResult is what became of a proposal, as a diff of the session across
// the user's answer. Added can name a repository the user chose and the agent
// never asked for; Dropped names one it asked for that they declined. The lists
// are filled rather than left nil, so a reply reads as empty lists not nulls.
type addReposResult struct {
	Status   string
	Added    []string
	Promoted []string
	Held     []string
	Dropped  []string
}

// addReposDecision is one answer to one proposal, travelling in memory from the
// overlay's confirm back to the blocked tool call. Status is the authority on how
// it ended; Outcome carries the diff only when something was composed.
type addReposDecision struct {
	Status  string
	Outcome addReposResult
	Err     error
}

// add proposes repositories to the user and waits for the answer. It takes no
// lock: the wait is on a human, and holding p.mu across it would block the very
// Confirm the wait exists for. Names are resolved first so a mistyped one is
// refused before anybody is interrupted, and nothing is composed here — that
// happens on the confirm path, which is what makes an unanswered proposal a
// guaranteed no-op.
func (p *Picker) add(ctx context.Context, slug string, additions []repoAddition, deadline time.Time) (addReposResult, error) {
	state, root, err := p.root(slug)
	if err != nil {
		return addReposResult{}, err
	}
	refreshCtx, cancel := context.WithTimeout(ctx, repoRefreshTimeout)
	defer cancel()
	if err := p.repos.refreshAndWait(refreshCtx); err != nil {
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
	// Partitioned before proposing only to refuse a batch the rules reject —
	// a role conflict, say — rather than drawing an overlay for it.
	if _, _, _, err := partitionAdditions(m, selections); err != nil {
		return addReposResult{}, err
	}
	answer, err := p.sessions.proposeRepos(workbench.PickerRequest{
		SessionRoot: root, Deadline: deadline, Repos: proposalFor(selections),
	})
	if err != nil {
		return addReposResult{}, err
	}
	select {
	case decision := <-answer:
		if decision.Err != nil {
			return addReposResult{}, decision.Err
		}
		return settled(decision), nil
	case <-ctx.Done():
		// Nobody answered, so nothing was composed: the confirm path is the only
		// one that writes, and it would have delivered a decision here.
		state.clearPicker()
		state.takeAnswer()
		return settled(addReposDecision{Status: workbench.AddReposExpired}), nil
	}
}

// settled stamps the decision's status onto its outcome and fills the lists a
// refusal never built, so every answer reads the same shape.
func settled(decision addReposDecision) addReposResult {
	out := decision.Outcome
	out.Status = decision.Status
	for _, list := range []*[]string{&out.Added, &out.Promoted, &out.Held, &out.Dropped} {
		if *list == nil {
			*list = []string{}
		}
	}
	return out
}

// proposalFor is the resolved selection as the overlay's pre-seed, carrying the
// canonical org/name so a bare name the agent gave matches a drawn row.
func proposalFor(selections []session.RepoSelection) []workbench.RepoAddition {
	out := make([]workbench.RepoAddition, 0, len(selections))
	for _, sel := range selections {
		out = append(out, workbench.RepoAddition{
			Name: sel.Repo.ID(), Role: string(sel.Role.Effective()),
		})
	}
	return out
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
	pending := state.pendingPicker()
	// A declined proposal writes no cancellation stanza: only an escalation has a
	// caller polling the manifest for one.
	proposal := pending != nil && len(pending.Repos) > 0
	if err := assembly.Cancel(root, pending != nil && !proposal); err != nil {
		return err
	}
	if answer := state.takeAnswer(); answer != nil {
		answer <- addReposDecision{Status: workbench.AddReposDeclined}
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
	return s.queue(req, nil)
}

// proposeRepos queues an agent's repository proposal and hands back the channel
// its outcome arrives on. Same queue and same overlay as an escalation; what
// differs is that somebody is waiting for the answer.
func (s *Sessions) proposeRepos(req workbench.PickerRequest) (chan addReposDecision, error) {
	answer := make(chan addReposDecision, 1)
	if err := s.queue(req, answer); err != nil {
		return nil, err
	}
	return answer, nil
}

func (s *Sessions) queue(req workbench.PickerRequest, answer chan addReposDecision) error {
	slug := slugFor(req.SessionRoot)
	state := s.bySlug(slug)
	if state == nil {
		return unknownSession(slug)
	}
	state.requestPicker(req, answer)
	s.touch()
	return nil
}
