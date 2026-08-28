package session

import "github.com/kieranajp/qrouton/internal/github"

// reporter emits one repository's progress through the assembly steps. The
// started/completed/failed triple is the same at every call site, and forgetting
// the failed half leaves a bar that never resolves.
type reporter struct {
	fn   ProgressFunc
	repo *github.Repo
	role RepoRole
}

// step reports the start of one step, runs it, then reports how it ended. work
// is handed the callback git's own clone and fetch progress goes to; that
// callback is nil when nobody is listening, which is what tells gitOK's callers
// to ask git for --quiet rather than --progress.
func (r reporter) step(step ProgressStep, work func(advance func(phase string, percent int)) error) error {
	var advance func(string, int)
	if r.fn != nil {
		advance = func(phase string, percent int) {
			r.emit(Progress{Step: step, Status: ProgressAdvanced, Phase: phase, Percent: percent})
		}
	}
	r.emit(Progress{Step: step, Status: ProgressStarted})
	if err := work(advance); err != nil {
		r.emit(Progress{Step: step, Status: ProgressFailed, Err: err})
		return err
	}
	r.emit(Progress{Step: step, Status: ProgressCompleted})
	return nil
}

// emit fills in the repository this reporter speaks for and delivers the event.
func (r reporter) emit(event Progress) {
	if r.fn == nil {
		return
	}
	event.Repo, event.Role = r.repo, r.role
	r.fn(event)
}
