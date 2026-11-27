package desktop

import (
	"sync"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
)

// terminals is this client's half of TerminalService. A page mount no event
// stream saw keeps the page's size here until the translator attaches it again.
type terminals struct {
	qroutonv1.TerminalServiceClient

	mu      sync.Mutex
	pending map[string]*qroutonv1.TerminalSize
}

func newTerminals(client qroutonv1.TerminalServiceClient) *terminals {
	return &terminals{TerminalServiceClient: client, pending: map[string]*qroutonv1.TerminalSize{}}
}

// holdPending keeps the size of a mount that no event stream saw.
func (t *terminals) holdPending(id string, size *qroutonv1.TerminalSize) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending[id] = size
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
