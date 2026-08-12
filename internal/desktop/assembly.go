package desktop

import (
	"context"
	"net/http"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

// draftInput is the overlay's form on the wire. Repos names repositories rather
// than carrying them, so resolving the names against the live list is what drops
// anything a refresh has taken away.
type draftInput struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Ticket      string     `json:"ticket"`
	Prefix      string     `json:"prefix"`
	Mode        string     `json:"mode"`
	Runner      string     `json:"runner"`
	Repos       []repoPick `json:"repos"`
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
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Assembly is what the overlay calls: the rules, the branch preview, the ticket
// lookup, and the create that ends with the new session on screen.
type Assembly struct {
	cfg       *config.Config
	repos     *Repositories
	sessions  *Sessions
	emit      emitter
	assembler assembly.Assembler
	runners   func() ([]assembly.Runner, error)
}

func newAssembly(cfg *config.Config, repos *Repositories, reg *Sessions, emit emitter,
	signal func(string), runners func() ([]assembly.Runner, error)) *Assembly {
	return &Assembly{cfg: cfg, repos: repos, sessions: reg, emit: emit,
		assembler: assembly.Assembler{Cfg: cfg, Signal: signal}, runners: runners}
}

func (a *Assembly) Prefixes() []string { return assembly.Prefixes() }

// Runners is only the agents with a resolved path, which is what "only agents
// found on your PATH are listed" means.
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
	return a.assembler.CheckSlug(a.draft(in))
}

func (a *Assembly) Preview(in draftInput) string { return assembly.Preview(a.draft(in)) }

func (a *Assembly) Fetch(url string) (ticketFields, error) {
	loaded, err := ticket.Fetch(context.Background(), http.DefaultClient, url)
	if err != nil {
		return ticketFields{}, err
	}
	return ticketFields{Title: loaded.Title, Body: loaded.Body}, nil
}

// Create assembles the session and puts it on screen. Adoption is in process
// because a webview overlay has no PTY to hand over: the session boots itself,
// on a socket of its own.
func (a *Assembly) Create(in draftInput) error {
	draft := a.draft(in)
	if problems := assembly.Check(draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	if problems := a.assembler.CheckSlug(draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	slug := session.Slugify(draft.Name)
	progress := func(p session.Progress) { a.emit(assemblyProgressEvent, newProgressEvent(slug, p)) }
	root, err := session.Create(a.cfg, draft.Name, draft.Description, draft.Ticket,
		draft.Prefix, draft.Mode, in.Runner, draft.Repos, progress)
	if err != nil {
		return err
	}
	return a.sessions.adopt(root, in.Runner)
}

// draft resolves the picked names against the list the step was drawn from. A
// repository a refresh has dropped simply is not there any more, which is what
// leaves the draft short of an editing repo for Check to refuse.
func (a *Assembly) draft(in draftInput) assembly.Draft {
	byID := make(map[string]github.Repo)
	for _, r := range a.repos.Cached() {
		byID[r.ID()] = r
	}
	repos := make([]session.RepoSelection, 0, len(in.Repos))
	for _, pick := range in.Repos {
		repo, ok := byID[pick.ID]
		if !ok {
			continue
		}
		repos = append(repos, session.RepoSelection{Repo: repo, Role: session.RepoRole(pick.Role)})
	}
	return assembly.Draft{Name: in.Name, Description: in.Description, Ticket: in.Ticket,
		Prefix: in.Prefix, Mode: session.SessionMode(in.Mode), Repos: repos}
}
