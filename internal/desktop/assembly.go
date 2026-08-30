package desktop

import (
	"context"
	"net/http"
	"sync"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

// draftInput is the overlay's form on the wire. Repos names repositories rather
// than carrying them, so resolving the names against the live list is what drops
// anything a refresh has taken away.
type draftInput struct {
	Name        string     `json:"name"`
	Entropy     string     `json:"entropy"`
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

// AssemblySeed is the external ticket, if any, claimed by an overlay mount.
type AssemblySeed struct {
	Ticket     string `json:"ticket"`
	Entropy    string `json:"entropy"`
	Generation uint64 `json:"generation"`
}

type linearSeed struct {
	ticket string
	prompt string
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

	mu         sync.Mutex
	pending    linearSeed
	draftOpen  bool
	external   linearSeed
	entropy    string
	generation uint64
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
	ctx, cancel := context.WithTimeout(context.Background(), ticketFetchTimeout)
	defer cancel()
	loaded, err := ticket.Fetch(ctx, http.DefaultClient, url)
	if err != nil {
		return ticketFields{}, err
	}
	return ticketFields{Title: loaded.Title, Body: loaded.Body}, nil
}

// Pending is the external ticket waiting for the page to open an overlay.
func (a *Assembly) Pending() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pending.ticket
}

// pendingLinear supplies the full external request to a replacement workbench.
// The prompt stays out of the frontend seed and is carried only to session
// creation, where the runner consumes it once.
func (a *Assembly) pendingLinear() (string, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pending.ticket, a.pending.prompt
}

// Begin claims the pending external ticket and owns the draft until End.
func (a *Assembly) Begin() AssemblySeed {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.generation++
	if !a.draftOpen {
		a.external = a.pending
		a.pending = linearSeed{}
		a.entropy = session.NewEntropy()
	}
	a.draftOpen = true
	return AssemblySeed{Ticket: a.external.ticket, Entropy: a.entropy, Generation: a.generation}
}

// End releases only the overlay generation that still owns the draft.
func (a *Assembly) End(generation uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.generation {
		return
	}
	a.draftOpen = false
	a.external = linearSeed{}
	a.entropy = ""
}

func (a *Assembly) offer(raw, prompt string) (string, error) {
	canonical, err := ticket.CanonicalLinearURL(raw)
	if err != nil {
		return "", err
	}
	if outcome, err := a.held(canonical); outcome != "" || err != nil {
		return outcome, err
	}
	if a.cfg == nil || a.sessions == nil {
		return "", ErrNoConfig
	}
	manifests, err := session.Scan(a.cfg.Root)
	if err != nil {
		return "", err
	}
	matching := make([]session.Manifest, 0, len(manifests))
	for _, manifest := range manifests {
		persisted, err := ticket.CanonicalLinearURL(manifest.TicketURL)
		if err == nil && persisted == canonical {
			matching = append(matching, manifest)
		}
	}
	if preferred, ok := session.Preferred(a.cfg.Root, matching); ok {
		if err := a.sessions.Show(preferred.Slug); err != nil {
			return "", err
		}
		return assemblyOutcomeExisting, nil
	}
	outcome, taken, err := a.takePending(linearSeed{ticket: canonical, prompt: prompt})
	if err != nil {
		return "", err
	}
	if taken && a.emit != nil {
		a.emit(assemblyRequestedEvent, canonical)
	}
	return outcome, nil
}

// held is the answer the draft state alone gives an offered ticket. An empty
// outcome and no error means nothing holds the assembly.
func (a *Assembly) held(canonical string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.heldLocked(canonical)
}

// takePending queues the seed, deciding again because the scan it followed ran
// off the lock: an offer of another issue that landed meanwhile keeps its claim
// and this one is refused. taken is false when the answer came from that claim
// rather than this seed.
func (a *Assembly) takePending(seed linearSeed) (string, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if outcome, err := a.heldLocked(seed.ticket); outcome != "" || err != nil {
		return outcome, false, err
	}
	a.pending = seed
	return assemblyOutcomeQueued, true, nil
}

func (a *Assembly) heldLocked(canonical string) (string, error) {
	if a.draftOpen {
		if a.external.ticket == canonical {
			return assemblyOutcomeDraft, nil
		}
		return "", ErrAssemblyDraftConflict
	}
	if a.pending.ticket != "" {
		if a.pending.ticket == canonical {
			return assemblyOutcomeQueued, nil
		}
		return "", ErrAssemblyDraftConflict
	}
	return "", nil
}

func (a *Assembly) initialPrompt() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.external.prompt
}

// Create assembles the session and puts it on screen. Adoption is in process
// because a webview overlay has no PTY to hand over: the session boots itself,
// on a socket of its own.
func (a *Assembly) Create(in draftInput) error {
	draft := a.draft(in)
	if draft.Entropy == "" {
		draft.Entropy = session.NewEntropy()
	}
	if problems := assembly.Check(draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	if problems := a.assembler.CheckSlug(draft); len(problems) > 0 {
		return draftRefused(problems[0])
	}
	slug := draft.Slug()
	progress := func(p session.Progress) { a.emit(assemblyProgressEvent, newProgressEvent(slug, p)) }
	root, err := session.Create(a.cfg, session.CreateRequest{
		Name: draft.Name, Slug: slug, Description: draft.Description, Ticket: draft.Ticket,
		InitialPrompt: a.initialPrompt(), Prefix: draft.Prefix, Mode: draft.Mode, Runner: in.Runner,
		Repos: draft.Repos,
	}, progress)
	if err != nil {
		return err
	}
	return a.sessions.adopt(root, in.Runner)
}

func (a *Assembly) draft(in draftInput) assembly.Draft {
	return assembly.Draft{Name: in.Name, Entropy: in.Entropy, Description: in.Description, Ticket: in.Ticket,
		Prefix: in.Prefix, Mode: session.SessionMode(in.Mode), Repos: a.repos.Select(in.Repos)}
}
