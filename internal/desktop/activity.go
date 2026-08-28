package desktop

import (
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
)

// activity is what the workbench can say about the running agent. Waiting comes
// from the runner's own hook; working and idle are the PTY's timing, never its
// contents. A runner without that hook simply never reports waiting.
type activity struct {
	mu      sync.Mutex
	waiting bool
	spoke   time.Time
}

// hook records what the runner said about itself.
func (a *activity) hook(state string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waiting = state == status.ActivityWaiting
	if !a.waiting {
		a.spoke = time.Now()
	}
}

// wrote notes that the PTY produced output. Only the time is kept.
func (a *activity) wrote() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.spoke = time.Now()
}

// answered notes that the user typed. Whatever the agent was blocked on, it is
// no longer blocked on it.
func (a *activity) answered() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waiting = false
	a.spoke = time.Now()
}

func (a *activity) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waiting = false
	a.spoke = time.Time{}
}

func (a *activity) state() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.waiting:
		return status.ActivityWaiting
	case !a.spoke.IsZero() && time.Since(a.spoke) < activityQuiet:
		return status.ActivityWorking
	default:
		return status.ActivityIdle
	}
}
