package desktop

import (
	"sync"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
)

// terminals is this client's half of TerminalService. It remembers which
// terminals its page has mounted, and the size of a mount no event stream saw.
type terminals struct {
	qroutonv1.TerminalServiceClient

	mu      sync.Mutex
	mounted map[string]bool
	pending map[string]*qroutonv1.TerminalSize
}

func newTerminals(client qroutonv1.TerminalServiceClient) *terminals {
	return &terminals{TerminalServiceClient: client, mounted: map[string]bool{}, pending: map[string]*qroutonv1.TerminalSize{}}
}

func (t *terminals) mount(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mounted[id] = true
}

func (t *terminals) unmount(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.mounted, id)
}

// mountedOf keeps only the listed terminals, since the rest have closed, and
// returns which of them the page has mounted.
func (t *terminals) mountedOf(listed []*qroutonv1.SessionTerminal) map[string]bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := map[string]bool{}
	for _, terminal := range listed {
		if id := terminal.GetTerminalId(); t.mounted[id] {
			kept[id] = true
		}
	}
	t.mounted = kept
	return kept
}

// holdPending keeps the size of a mount that no event stream saw.
func (t *terminals) holdPending(id string, size *qroutonv1.TerminalSize) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending[id] = size
}

func (t *terminals) dropPending(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pending, id)
}

// resizePending follows the page's resize of a terminal still pending.
func (t *terminals) resizePending(id string, size *qroutonv1.TerminalSize) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.pending[id]; ok {
		t.pending[id] = size
	}
}

// takePending hands over every pending size.
func (t *terminals) takePending() map[string]*qroutonv1.TerminalSize {
	t.mu.Lock()
	defer t.mu.Unlock()
	taken := t.pending
	t.pending = map[string]*qroutonv1.TerminalSize{}
	return taken
}

// keepPending puts a size back unless a newer mount has recorded one.
func (t *terminals) keepPending(id string, size *qroutonv1.TerminalSize) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.pending[id]; !ok {
		t.pending[id] = size
	}
}
