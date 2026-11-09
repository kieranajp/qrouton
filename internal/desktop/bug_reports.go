package desktop

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type bugReportRecord struct {
	request  workbench.BugReportRequest
	outcome  workbench.BugReport
	finished time.Time
}

type BugReportPreview struct {
	Report         workbench.BugReport `json:"report"`
	Title          string              `json:"title"`
	Body           string              `json:"body"`
	Destination    string              `json:"destination"`
	ReviewLabel    string              `json:"reviewLabel"`
	CreateLabel    string              `json:"createLabel"`
	CancelLabel    string              `json:"cancelLabel"`
	CloseLabel     string              `json:"closeLabel"`
	UnknownMessage string              `json:"unknownMessage"`
}

type BugReports struct {
	sessions *Sessions
	now      func() time.Time
	write    func(context.Context, string, string) (github.Issue, error)
}

func newBugReports(sessions *Sessions) *BugReports {
	return &BugReports{sessions: sessions, now: sessions.now, write: writeBugReport}
}

func writeBugReport(ctx context.Context, title, body string) (github.Issue, error) {
	token, err := github.TokenContext(ctx)
	if err != nil {
		return github.Issue{}, err
	}
	return github.CreateIssue(ctx, http.DefaultClient, token, title, body)
}

func (s *sessionState) pruneBugReports(now time.Time) {
	for id, r := range s.bugReports {
		if r.outcome.Status == workbench.BugReportPending && (s.stopped || !now.Before(r.request.Deadline)) {
			state, message := workbench.BugReportExpired, bugReportExpiredMessage
			if s.stopped {
				state, message = workbench.BugReportCancelled, bugReportCancelledMessage
			}
			r.outcome.Status, r.outcome.Message, r.finished = state, message, now
		}
		if !r.finished.IsZero() && !now.Before(r.finished.Add(bugReportRetention)) {
			delete(s.bugReports, id)
			if s.latestBugReport == id {
				s.latestBugReport = ""
			}
		}
	}
}

func (b *BugReports) queue(s *sessionState, req workbench.BugReportRequest) (workbench.BugReport, error) {
	if s == nil {
		return workbench.BugReport{}, ErrNoSession
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.ID == "" || req.Title == "" || strings.TrimSpace(req.Body) == "" || req.Deadline.IsZero() {
		return workbench.BugReport{}, ErrBugReportInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := b.now()
	s.pruneBugReports(now)
	if s.stopped {
		return workbench.BugReport{}, ErrNoSession
	}
	if r := s.bugReports[req.ID]; r != nil {
		if r.request.Title != req.Title || r.request.Body != req.Body {
			return workbench.BugReport{}, ErrBugReportInvalid
		}
		return r.outcome, nil
	}
	for _, r := range s.bugReports {
		if !r.outcome.Terminal() {
			return workbench.BugReport{}, ErrBugReportBusy
		}
	}
	if limit := now.Add(workbench.BugReportReviewTimeout); req.Deadline.After(limit) {
		req.Deadline = limit
	}
	if s.bugReports == nil {
		s.bugReports = map[string]*bugReportRecord{}
	}
	r := &bugReportRecord{request: req, outcome: workbench.BugReport{ID: req.ID, Status: workbench.BugReportPending, Message: bugReportPendingMessage}}
	s.bugReports[req.ID], s.latestBugReport = r, req.ID
	s.pruneBugReports(now)
	b.sessions.touch()
	return r.outcome, nil
}

func (b *BugReports) status(s *sessionState, id string) (workbench.BugReport, error) {
	if s == nil {
		return workbench.BugReport{}, ErrNoSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneBugReports(b.now())
	r := s.bugReports[id]
	if r == nil {
		return workbench.BugReport{}, ErrBugReportStale
	}
	return r.outcome, nil
}

func (b *BugReports) cancel(s *sessionState, id string) (workbench.BugReport, error) {
	if s == nil {
		return workbench.BugReport{}, ErrNoSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneBugReports(b.now())
	r := s.bugReports[id]
	if r == nil {
		return workbench.BugReport{}, ErrBugReportStale
	}
	if r.outcome.Status == workbench.BugReportPending {
		r.outcome.Status, r.outcome.Message, r.finished = workbench.BugReportCancelled, bugReportCancelledMessage, b.now()
		b.sessions.touch()
	}
	return r.outcome, nil
}

func (b *BugReports) Load(slug, requestID string) (BugReportPreview, error) {
	s := b.sessions.bySlug(slug)
	if s == nil {
		return BugReportPreview{}, ErrNoSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneBugReports(b.now())
	r := s.bugReports[requestID]
	if r == nil || s.latestBugReport != requestID {
		return BugReportPreview{}, ErrBugReportStale
	}
	return BugReportPreview{Report: r.outcome, Title: r.request.Title, Body: r.request.Body, Destination: github.IssueRepository,
		ReviewLabel: bugReportReviewLabel, CreateLabel: bugReportCreateLabel, CancelLabel: bugReportCancelLabel, CloseLabel: bugReportCloseLabel, UnknownMessage: github.ErrIssueUnknown.Error()}, nil
}

func (b *BugReports) Cancel(slug, requestID string) (workbench.BugReport, error) {
	return b.cancel(b.sessions.bySlug(slug), requestID)
}

func (b *BugReports) Confirm(slug, requestID string) (workbench.BugReport, error) {
	s := b.sessions.bySlug(slug)
	if s == nil {
		return workbench.BugReport{}, ErrNoSession
	}
	s.mu.Lock()
	s.pruneBugReports(b.now())
	r := s.bugReports[requestID]
	if s.stopped || r == nil || s.latestBugReport != requestID {
		s.mu.Unlock()
		return workbench.BugReport{}, ErrBugReportStale
	}
	if r.outcome.Status != workbench.BugReportPending {
		out := r.outcome
		s.mu.Unlock()
		return out, nil
	}
	r.outcome.Status, r.outcome.Message = workbench.BugReportPosting, bugReportPostingMessage
	out, title, body := r.outcome, r.request.Title, r.request.Body
	s.mu.Unlock()
	b.sessions.touch()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), bugReportPostTimeout)
		defer cancel()
		issue, err := b.write(ctx, title, body)
		result := workbench.BugReport{ID: requestID}
		switch {
		case errors.Is(err, github.ErrNoToken), errors.Is(err, github.ErrIssueInvalid), errors.Is(err, github.ErrIssueRejected):
			result.Status, result.Message = workbench.BugReportFailed, err.Error()
		case err != nil || !github.ValidIssue(issue):
			result.Status, result.Message = workbench.BugReportUnknown, github.ErrIssueUnknown.Error()
		default:
			result.Status, result.Message = workbench.BugReportCreated, fmt.Sprintf(bugReportCreatedFormat, issue.Number)
			result.URL, result.Number = issue.URL, issue.Number
		}
		s.mu.Lock()
		r.outcome, r.finished = result, b.now()
		s.mu.Unlock()
		b.sessions.touch()
	}()
	return out, nil
}
