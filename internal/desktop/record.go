package desktop

import (
	"sync"

	"github.com/kieranajp/qrouton/internal/session"
)

// windowRecorder keeps qrouton.json's record of the windows the agent has open,
// so the session's state is legible from the file and a later resume can rebuild
// it. Nothing replays the record yet.
type windowRecorder struct {
	windows *Windows

	// mu orders the read-modify-write, so the last change is what the manifest
	// ends up saying.
	mu sync.Mutex
}

// save is best-effort: a session whose manifest is not written yet, or a root
// onboarding has not chosen, costs nothing while nothing reads the record back.
func (r *windowRecorder) save(owner *sessionState) {
	if owner == nil {
		return
	}
	root := owner.root()
	if root == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	open := r.windows.snapshot(owner)
	records := make([]session.WindowRecord, 0, len(open))
	for _, opts := range open {
		records = append(records, session.WindowRecord{
			Kind:    string(opts.Kind),
			Label:   opts.Label,
			Cwd:     opts.Cwd,
			Command: opts.Command,
		})
	}
	_ = session.SetWindows(root, records)
}
