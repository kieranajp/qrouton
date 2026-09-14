package workbench

import (
	"context"
	"time"
)

type BugReportHost interface {
	QueueBugReport(context.Context, BugReportRequest) (BugReport, error)
	BugReportStatus(context.Context, string) (BugReport, error)
	CancelBugReport(context.Context, string) (BugReport, error)
}

type BugReportRequest struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	Deadline time.Time `json:"deadline"`
}

type BugReport struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
	Number  int    `json:"number,omitempty"`
}

func (r BugReport) Terminal() bool {
	switch r.Status {
	case BugReportCreated, BugReportCancelled, BugReportExpired, BugReportFailed, BugReportUnknown:
		return true
	}
	return false
}

func (c *client) QueueBugReport(ctx context.Context, req BugReportRequest) (BugReport, error) {
	return c.reportCall(ctx, Request{Op: OpQueueBugReport, ID: req.ID, BugReport: &req})
}

func (c *client) BugReportStatus(ctx context.Context, id string) (BugReport, error) {
	return c.reportCall(ctx, Request{Op: OpBugReportStatus, ID: id})
}

func (c *client) CancelBugReport(ctx context.Context, id string) (BugReport, error) {
	return c.reportCall(ctx, Request{Op: OpCancelBugReport, ID: id})
}

func (c *client) reportCall(ctx context.Context, req Request) (BugReport, error) {
	ctx, cancel := context.WithTimeout(ctx, BugReportCallTimeout)
	defer cancel()
	res, err := c.call(ctx, req)
	if err != nil {
		return BugReport{}, err
	}
	r := res.BugReport
	if r == nil || r.ID != req.ID || r.Message == "" || (!r.Terminal() && r.Status != BugReportPending && r.Status != BugReportPosting) || (r.Status == BugReportCreated && (r.Number <= 0 || r.URL == "")) {
		return BugReport{}, ErrBugReportUnavailable
	}
	return *r, nil
}
