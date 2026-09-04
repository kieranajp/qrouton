package desktop

import (
	"context"
	"net/http"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

// draftInput is the overlay's form on the wire. Repos names repositories rather
// than carrying them, so resolving the names against the live list is what drops
// anything a refresh has taken away.
type draftInput struct {
	Name              string     `json:"name"`
	BranchDescription string     `json:"branchDescription"`
	Entropy           string     `json:"entropy"`
	Description       string     `json:"description"`
	Ticket            string     `json:"ticket"`
	Prefix            string     `json:"prefix"`
	Mode              string     `json:"mode"`
	Runner            string     `json:"runner"`
	Repos             []repoPick `json:"repos"`
}

// repoPick is one row's answer, in the order it was picked: first picked means
// worked in most.
type repoPick struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// ticketFields is a fetched ticket on the wire. The domain type knows nothing
// about pages and keeps it that way.
type ticketFields struct {
	Title             string `json:"title"`
	Body              string `json:"body"`
	BranchDescription string `json:"branchDescription"`
}

type AssemblySeed struct {
	Ticket     string `json:"ticket"`
	Entropy    string `json:"entropy"`
	Generation uint64 `json:"generation"`
}

type Assembly struct {
	cfg       *config.Config
	repos     *Repositories
	sessions  *Sessions
	emit      emitter
	assembler assembly.Assembler
	runners   func() ([]assembly.Runner, error)
	offers    assembly.Offers
}

func newAssembly(cfg *config.Config, repos *Repositories, reg *Sessions, emit emitter,
	signal func(string), runners func() ([]assembly.Runner, error)) *Assembly {
	return &Assembly{cfg: cfg, repos: repos, sessions: reg, emit: emit,
		assembler: assembly.Assembler{Cfg: cfg, Signal: signal}, runners: runners}
}

func (a *Assembly) Prefixes() []string { return assembly.Prefixes() }

func (a *Assembly) Runners() ([]assembly.Runner, error) {
	if a.runners == nil {
		return nil, ErrNoAgentCommand
	}
	all, err := a.runners()
	if err != nil {
		return nil, err
	}
	return assembly.Installed(all), nil
}

func (a *Assembly) Check(in draftInput) []assembly.Problem {
	return assembly.Check(a.draft(in))
}

func (a *Assembly) CheckSlug(in draftInput) []assembly.Problem {
	assembler := a.assembler
	assembler.Cfg = a.cfg.Snapshot()
	return assembler.CheckSlug(a.draft(in))
}

func (a *Assembly) Preview(in draftInput) string { return assembly.Preview(a.draft(in)) }

func (a *Assembly) Fetch(url string) (ticketFields, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ticketFetchTimeout)
	defer cancel()
	loaded, err := ticket.Fetch(ctx, http.DefaultClient, url)
	if err != nil {
		return ticketFields{}, err
	}
	return ticketFields{Title: loaded.Title, Body: loaded.Body,
		BranchDescription: assembly.SuggestBranchDescription(loaded.Title)}, nil
}

func (a *Assembly) Pending() string { return a.offers.Pending().Ticket }

// pendingTicket supplies the full external request to a replacement workbench.
// The prompt stays out of the frontend seed and is carried only to session
// creation, where the runner consumes it once.
func (a *Assembly) pendingTicket() (string, string) {
	pending := a.offers.Pending()
	return pending.Ticket, pending.Prompt
}

func (a *Assembly) Begin() AssemblySeed {
	claim := a.offers.Begin()
	return AssemblySeed{Ticket: claim.Seed.Ticket, Entropy: claim.Entropy, Generation: claim.Generation}
}

func (a *Assembly) End(generation uint64) { a.offers.End(generation) }

// offer routes an externally opened ticket: to the session that already holds
// it, or into the overlay's queue for a session that does not exist yet.
func (a *Assembly) offer(raw, prompt string) (string, error) {
	canonical, err := ticket.Canonical(raw)
	if err != nil {
		return "", err
	}
	if outcome, err := a.offers.Held(canonical); outcome != "" || err != nil {
		return outcome, err
	}
	if a.cfg == nil || a.sessions == nil {
		return "", ErrNoConfig
	}
	preferred, found, err := assembly.SessionFor(a.cfg.Snapshot(), canonical)
	if err != nil {
		return "", err
	}
	if found {
		if err := a.sessions.Show(preferred.Slug); err != nil {
			return "", err
		}
		return assembly.OutcomeExisting, nil
	}
	outcome, taken, err := a.offers.Take(assembly.Seed{Ticket: canonical, Prompt: prompt})
	if err != nil {
		return "", err
	}
	if taken && a.emit != nil {
		a.emit(assemblyRequestedEvent, canonical)
	}
	return outcome, nil
}

func (a *Assembly) Create(in draftInput) error {
	cfg := a.cfg.Snapshot()
	draft := a.draft(in)
	if draft.Entropy == "" {
		draft.Entropy = session.NewEntropy()
	}
	if problems := assembly.Check(draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	assembler := a.assembler
	assembler.Cfg = cfg
	if problems := assembler.CheckSlug(draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	slug := draft.Slug()
	progress := func(p session.Progress) { a.emit(assemblyProgressEvent, newProgressEvent(slug, p)) }
	root, err := session.Create(cfg, session.CreateRequest{
		Name: draft.Name, Slug: slug, Description: draft.Description, Ticket: draft.Ticket,
		InitialPrompt: a.offers.Prompt(), Prefix: draft.Prefix, Mode: draft.Mode, Runner: in.Runner,
		Repos: draft.Repos,
	}, progress)
	if err != nil {
		return err
	}
	return a.sessions.adopt(root, in.Runner)
}

func (a *Assembly) draft(in draftInput) assembly.Draft {
	return assembly.Draft{Name: in.Name, BranchDescription: in.BranchDescription, Entropy: in.Entropy,
		Description: in.Description, Ticket: in.Ticket, Prefix: in.Prefix,
		Mode: session.SessionMode(in.Mode), Repos: a.repos.Select(in.Repos)}
}
