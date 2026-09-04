package assembly

import (
	"errors"
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

var ErrDraftConflict = errors.New("another New session draft is already open; finish or cancel it before opening a different ticket")

// Seed is an externally offered ticket: the canonical URL a session persists,
// and the prompt the offering tool generated for the runner's first turn.
type Seed struct {
	Ticket string
	Prompt string
}

// Claim is the overlay's answer to Begin: the ticket it opens on, the entropy
// its slug is salted with, and the generation that owns the draft until End.
type Claim struct {
	Seed       Seed
	Entropy    string
	Generation uint64
}

// Offers is the hold an external ticket has on the New session overlay. At most
// one ticket is queued and at most one draft is open; a second ticket arriving
// against either is refused rather than displacing it.
type Offers struct {
	mu         sync.Mutex
	pending    Seed
	open       bool
	claimed    Seed
	entropy    string
	generation uint64
}

func (o *Offers) Pending() Seed {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pending
}

// Prompt is the claimed ticket's generated prompt, which session creation
// consumes once and the frontend seed never carries.
func (o *Offers) Prompt() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.claimed.Prompt
}

// Begin claims the pending ticket and owns the draft until End. Reopening an
// overlay that already holds one keeps its ticket and its entropy.
func (o *Offers) Begin() Claim {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.generation++
	if !o.open {
		o.claimed = o.pending
		o.pending = Seed{}
		o.entropy = session.NewEntropy()
	}
	o.open = true
	return Claim{Seed: o.claimed, Entropy: o.entropy, Generation: o.generation}
}

// End releases only the overlay generation that still owns the draft.
func (o *Offers) End(generation uint64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if generation != o.generation {
		return
	}
	o.open = false
	o.claimed = Seed{}
	o.entropy = ""
}

// Held is the answer this hold alone gives an offered ticket. An empty outcome
// and no error means nothing holds the overlay and the caller may go on.
func (o *Offers) Held(canonical string) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.held(canonical)
}

// Take queues a ticket, deciding again because the work between Held and here
// runs off the lock: another offer that landed meanwhile keeps its claim and
// this one is refused. taken is false when the answer came from that claim.
func (o *Offers) Take(seed Seed) (outcome string, taken bool, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if outcome, err := o.held(seed.Ticket); outcome != "" || err != nil {
		return outcome, false, err
	}
	o.pending = seed
	return OutcomeQueued, true, nil
}

func (o *Offers) held(canonical string) (string, error) {
	if o.open {
		if o.claimed.Ticket == canonical {
			return OutcomeDraft, nil
		}
		return "", ErrDraftConflict
	}
	if o.pending.Ticket != "" {
		if o.pending.Ticket == canonical {
			return OutcomeQueued, nil
		}
		return "", ErrDraftConflict
	}
	return "", nil
}

// SessionFor is the session an offered ticket should reopen rather than
// assemble again: the preferred one among those whose manifest persists the
// same canonical URL.
func SessionFor(cfg *config.Config, canonical string) (session.Manifest, bool, error) {
	manifests, err := session.Scan(cfg.Root)
	if err != nil {
		return session.Manifest{}, false, err
	}
	matching := make([]session.Manifest, 0, len(manifests))
	for _, manifest := range manifests {
		if persisted, err := ticket.Canonical(manifest.TicketURL); err == nil && persisted == canonical {
			matching = append(matching, manifest)
		}
	}
	preferred, ok := session.Preferred(cfg.Root, matching)
	return preferred, ok, nil
}
