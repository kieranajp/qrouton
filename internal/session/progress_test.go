package session

import (
	"errors"
	"testing"

	"github.com/kieranajp/qrouton/internal/github"
)

func collect(events *[]Progress) ProgressFunc {
	return func(p Progress) { *events = append(*events, p) }
}

func statuses(events []Progress) []ProgressStatus {
	out := make([]ProgressStatus, 0, len(events))
	for _, event := range events {
		out = append(out, event.Status)
	}
	return out
}

func TestStepBracketsTheWorkItRuns(t *testing.T) {
	repo := github.Repo{Org: "org", Name: "svc"}
	var events []Progress
	rep := reporter{fn: collect(&events), repo: &repo, role: RepoRoleEditing}

	ran := false
	if err := rep.step(ProgressMirror, func(advance func(string, int)) error {
		// The bracket has to be open by the time the work runs: a bar that
		// starts after the clone is a bar nobody sees.
		if got := statuses(events); len(got) != 1 || got[0] != ProgressStarted {
			t.Errorf("statuses before the work ran = %v, want just started", got)
		}
		advance("Receiving objects", 47)
		ran = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("step did not run its work")
	}
	if got := statuses(events); len(got) != 3 ||
		got[0] != ProgressStarted || got[1] != ProgressAdvanced || got[2] != ProgressCompleted {
		t.Fatalf("statuses = %v, want started, advanced, completed", got)
	}
	for i, event := range events {
		if event.Step != ProgressMirror {
			t.Errorf("event %d names step %q", i, event.Step)
		}
		if event.Repo != &repo || event.Role != RepoRoleEditing {
			t.Errorf("event %d lost the repository it speaks for: %+v", i, event)
		}
	}
	if events[1].Phase != "Receiving objects" || events[1].Percent != 47 {
		t.Errorf("advanced event = %+v, want git's own phase and percentage", events[1])
	}
}

// A step that fails has to say so. The whole reason for the type is that a new
// error path cannot forget this emit.
func TestStepReportsAFailureAndReturnsIt(t *testing.T) {
	var events []Progress
	rep := reporter{fn: collect(&events), role: RepoRoleReference}
	boom := errors.New("mirror is unreachable")

	err := rep.step(ProgressWorktree, func(func(string, int)) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("step error = %v, want the work's own", err)
	}
	if got := statuses(events); len(got) != 2 || got[0] != ProgressStarted || got[1] != ProgressFailed {
		t.Fatalf("statuses = %v, want started then failed", got)
	}
	if !errors.Is(events[1].Err, boom) {
		t.Fatalf("failed event carried %v", events[1].Err)
	}
}

// gitSlow asks git for --progress or --quiet purely on whether its callback is
// nil (see verbosityFlag). So a reporter with nobody listening must hand the
// work a nil advance, not a closure that discards — otherwise every clone and
// fetch starts spraying progress at a stderr no one reads.
func TestAReporterWithNoListenerHandsTheWorkNoCallback(t *testing.T) {
	rep := reporter{}
	var advance func(string, int)
	handed := false
	if err := rep.step(ProgressMirror, func(a func(string, int)) error {
		advance, handed = a, true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !handed {
		t.Fatal("step did not run its work")
	}
	if advance != nil {
		t.Fatal("a reporter with no listener handed the work a callback, so git would be asked for --progress")
	}
	if got := verbosityFlag(advance); got != quietLongFlag {
		t.Fatalf("git would be asked for %q, want %q", got, quietLongFlag)
	}
}

// The mirror step is where this matters in practice, so pin the pairing rather
// than only the reporter in isolation.
func TestAListeningReporterAsksGitForProgress(t *testing.T) {
	rep := reporter{fn: func(Progress) {}}
	var advance func(string, int)
	if err := rep.step(ProgressMirror, func(a func(string, int)) error {
		advance = a
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := verbosityFlag(advance); got != progressFlag {
		t.Fatalf("git would be asked for %q, want %q", got, progressFlag)
	}
}
