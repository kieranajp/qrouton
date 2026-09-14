package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type reportBugInput struct {
	Title string `json:"title" jsonschema:"Short title describing the observed qrouton defect"`
	Body  string `json:"body" jsonschema:"Markdown source with observed behavior, expected behavior, reproduction steps, and relevant evidence"`
}

type reportBugOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
	Number  int    `json:"number,omitempty"`
}

func reportOutput(r workbench.BugReport) reportBugOutput {
	return reportBugOutput{Status: r.Status, Message: r.Message, URL: r.URL, Number: r.Number}
}

func unknownBugReport() reportBugOutput {
	return reportBugOutput{Status: workbench.BugReportUnknown, Message: github.ErrIssueUnknown.Error()}
}

func (m *windowManager) reportBug(ctx context.Context, input reportBugInput) (reportBugOutput, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" || strings.TrimSpace(input.Body) == "" {
		return reportBugOutput{}, ErrBugReportInput
	}
	host, ok := m.host.(workbench.BugReportHost)
	if !ok {
		return reportBugOutput{}, ErrBugReportUnsupported
	}
	token := make([]byte, bugReportIDBytes)
	if _, err := rand.Read(token); err != nil {
		return reportBugOutput{}, err
	}
	id := hex.EncodeToString(token)
	deadline := time.Now().Add(workbench.BugReportReviewTimeout)
	if earlier, ok := ctx.Deadline(); ok && earlier.Before(deadline) {
		deadline = earlier
	}
	callCtx, cancel := context.WithTimeout(ctx, workbench.BugReportCallTimeout)
	r, err := host.QueueBugReport(callCtx, workbench.BugReportRequest{ID: id, Title: input.Title, Body: input.Body, Deadline: deadline})
	cancel()
	if err != nil {
		if errors.Is(err, workbench.ErrWorkbenchRefused) {
			return reportBugOutput{Status: workbench.BugReportFailed, Message: err.Error()}, nil
		}
		cancelCtx, stop := context.WithTimeout(context.Background(), workbench.BugReportCallTimeout)
		defer stop()
		if outcome, cancelErr := host.CancelBugReport(cancelCtx, id); cancelErr == nil && outcome.Terminal() {
			return reportOutput(outcome), nil
		}
		return unknownBugReport(), nil
	}
	if r.Terminal() {
		return reportOutput(r), nil
	}
	callCtx, cancel = context.WithTimeout(ctx, workbench.BugReportCallTimeout)
	_, _ = m.notify(callCtx, notifyInput{Message: bugReportNotification})
	cancel()
	return awaitBugReport(ctx, host, id, deadline)
}

func awaitBugReport(ctx context.Context, host workbench.BugReportHost, id string, deadline time.Time) (reportBugOutput, error) {
	ticker := time.NewTicker(bugReportPollInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Until(deadline.Add(bugReportFinishAllowance)))
	defer timeout.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			cancelCtx, cancel := context.WithTimeout(context.Background(), workbench.BugReportCallTimeout)
			defer cancel()
			r, err := host.CancelBugReport(cancelCtx, id)
			if err == nil && r.Terminal() {
				return reportOutput(r), nil
			}
			return unknownBugReport(), nil
		case <-timeout.C:
			return unknownBugReport(), nil
		case <-ticker.C:
			callCtx, cancel := context.WithTimeout(ctx, workbench.BugReportCallTimeout)
			r, err := host.BugReportStatus(callCtx, id)
			cancel()
			if err != nil {
				failures++
				if failures >= bugReportPollFailures {
					return unknownBugReport(), nil
				}
				continue
			}
			failures = 0
			if r.Terminal() {
				return reportOutput(r), nil
			}
		}
	}
}
