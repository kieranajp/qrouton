package core

import (
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
)

// The events the overlay listens to. Their domain originals carry an error and
// no tags, so neither is emittable as written: an error marshals to {} and an
// untagged field reaches the page under its Go name.

// RefreshEvent is one owner's place in a repository refresh. Every event carries
// the generation it belongs to, because events broadcast process-wide and a
// superseded refresh's events are still in flight.
type RefreshEvent struct {
	Generation int           `json:"generation"`
	Owner      string        `json:"owner"`
	State      string        `json:"state"`
	Repos      []github.Repo `json:"repos,omitempty"`
	Error      string        `json:"error,omitempty"`
}

func newRefreshEvent(generation int, msg github.RefreshMsg) RefreshEvent {
	return RefreshEvent{
		Generation: generation,
		Owner:      msg.Owner,
		State:      string(msg.State),
		Repos:      msg.Repos,
		Error:      errorText(msg.Err),
	}
}

// BranchList is a repository's branches for the base menu, Default first.
// Error is a listing that failed, in which case Branches holds the default
// branch alone and the menu offers that.
type BranchList struct {
	Branches []string `json:"branches"`
	Default  string   `json:"default"`
	Error    string   `json:"error,omitempty"`
}

// ProgressEvent is one step of one session's assembly, named by the slug it
// belongs to so a page ignores another session's.
type ProgressEvent struct {
	Session string `json:"session"`
	Step    string `json:"step"`
	Status  string `json:"status"`
	Repo    string `json:"repo,omitempty"`
	Role    string `json:"role,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Percent int    `json:"percent"`
	Error   string `json:"error,omitempty"`
}

func newProgressEvent(slug string, p session.Progress) ProgressEvent {
	event := ProgressEvent{
		Session: slug,
		Step:    string(p.Step),
		Status:  string(p.Status),
		Role:    string(p.Role),
		Phase:   p.Phase,
		Percent: p.Percent,
		Error:   errorText(p.Err),
	}
	if p.Repo != nil {
		event.Repo = p.Repo.ID()
	}
	return event
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
