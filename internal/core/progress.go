package core

import "github.com/kieranajp/qrouton/internal/session"

// progressTo emits each step it is told about as assembly progress for slug.
func progressTo(events Events, slug string) session.ProgressFunc {
	return func(p session.Progress) { events.AssemblyProgress(newProgressEvent(slug, p)) }
}
