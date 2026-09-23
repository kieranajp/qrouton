package mcpserver

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type reportHost struct {
	fakeHost
	reportMu  sync.Mutex
	requests  []workbench.BugReportRequest
	status    string
	polls     int
	lostPoll  bool
	cancelled bool
	queued    chan struct{}
}

func (h *reportHost) QueueBugReport(_ context.Context, req workbench.BugReportRequest) (workbench.BugReport, error) {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	if len(h.requests) > 0 && h.status == workbench.BugReportPending {
		return workbench.BugReport{}, workbench.ErrWorkbenchRefused
	}
	h.requests = append(h.requests, req)
	if h.queued != nil {
		close(h.queued)
		h.queued = nil
	}
	return workbench.BugReport{ID: req.ID, Status: workbench.BugReportPending, Message: "Review"}, nil
}
func (h *reportHost) BugReportStatus(_ context.Context, id string) (workbench.BugReport, error) {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	h.polls++
	if h.lostPoll && h.polls == 1 {
		return workbench.BugReport{}, errors.New("lost reply")
	}
	r := workbench.BugReport{ID: id, Status: h.status, Message: "Outcome"}
	if r.Status == workbench.BugReportCreated {
		r.Number, r.URL = 42, github.IssueListURL+"/42"
	}
	return r, nil
}
func (h *reportHost) CancelBugReport(_ context.Context, id string) (workbench.BugReport, error) {
	h.reportMu.Lock()
	defer h.reportMu.Unlock()
	h.cancelled = true
	state := workbench.BugReportCancelled
	if h.status == workbench.BugReportPosting {
		state = workbench.BugReportPosting
	}
	return workbench.BugReport{ID: id, Status: state, Message: "Cancelled before posting"}, nil
}

func connectBugReportClient(t *testing.T, host workbench.WindowHost, mode session.SessionMode) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(t.TempDir(), testEditor, host, mode)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close(); _ = ss.Close() })
	return cs
}

func TestBugReportMCPContractInBothModes(t *testing.T) {
	for _, mode := range []session.SessionMode{session.ModeRPI, session.ModeAssistant} {
		t.Run(string(mode), func(t *testing.T) {
			host := &reportHost{status: workbench.BugReportCreated, lostPoll: true}
			tool := listedTools(t, newMCPServer(t.TempDir(), testEditor, host, mode))[toolReportBug]
			schema := structuredOutput(t, tool.InputSchema)
			properties := schema["properties"].(map[string]any)
			if len(properties) != 2 || properties["title"] == nil || properties["body"] == nil || len(schema["required"].([]any)) != 2 {
				t.Fatal(schema)
			}
			cs := connectBugReportClient(t, host, mode)
			out, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: toolReportBug, Arguments: map[string]any{"title": " title ", "body": "\n body \n"}})
			if err != nil || out.IsError {
				t.Fatalf("call = %+v, %v", out, err)
			}
			result := structuredOutput(t, out.StructuredContent)
			if result["status"] != workbench.BugReportCreated || result["url"] != github.IssueListURL+"/42" {
				t.Fatal(result)
			}
			host.reportMu.Lock()
			defer host.reportMu.Unlock()
			if len(host.requests) != 1 || host.requests[0].Title != "title" || host.requests[0].Body != "\n body \n" || host.polls != 2 {
				t.Fatalf("requests=%+v polls=%d", host.requests, host.polls)
			}
			if len(host.opens) != 1 || host.opens[0].Select {
				t.Fatalf("notification=%+v", host.opens)
			}
		})
	}
}

func TestBugReportMCPRejectsApprovalAndMissingInput(t *testing.T) {
	host := &reportHost{status: workbench.BugReportCreated}
	cs := connectBugReportClient(t, host, session.ModeRPI)
	for _, args := range []map[string]any{{"title": "x"}, {"title": " ", "body": "x"}, {"title": "x", "body": "x", "approved": true}} {
		out, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: toolReportBug, Arguments: args})
		if err == nil && !out.IsError {
			t.Fatalf("accepted %+v: %+v", args, out)
		}
	}
	if len(host.requests) != 0 {
		t.Fatal(host.requests)
	}
	m := newWindowManager(t.TempDir(), testEditor, &fakeHost{})
	if _, err := m.reportBug(context.Background(), reportBugInput{Title: "x", Body: "x"}); !errors.Is(err, ErrBugReportUnsupported) {
		t.Fatal(err)
	}
}

func TestBugReportWaitDoesNotHoldWindowLockAndCancelsPromptly(t *testing.T) {
	queued := make(chan struct{})
	host := &reportHost{status: workbench.BugReportPending, queued: queued}
	m := newWindowManager(t.TempDir(), testEditor, host)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan reportBugOutput, 1)
	go func() { out, _ := m.reportBug(ctx, reportBugInput{Title: "x", Body: "x"}); done <- out }()
	<-queued
	listed := make(chan struct{})
	go func() { m.list(context.Background()); close(listed) }()
	select {
	case <-listed:
	case <-time.After(time.Second):
		t.Fatal("review blocked windows")
	}
	if r, err := m.reportBug(context.Background(), reportBugInput{Title: "competing", Body: "x"}); err != nil || r.Status != workbench.BugReportFailed {
		t.Fatalf("competing=%+v %v", r, err)
	}
	cancel()
	select {
	case out := <-done:
		if out.Status != workbench.BugReportCancelled {
			t.Fatal(out)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation hung")
	}
	if !host.cancelled || len(host.requests) != 1 {
		t.Fatalf("cancelled=%v requests=%+v", host.cancelled, host.requests)
	}
}

func TestBugReportReturnsEveryTerminalOutcome(t *testing.T) {
	for _, state := range []string{workbench.BugReportCancelled, workbench.BugReportExpired, workbench.BugReportFailed, workbench.BugReportUnknown} {
		host := &reportHost{status: state}
		m := newWindowManager(t.TempDir(), testEditor, host)
		out, err := m.reportBug(context.Background(), reportBugInput{Title: "x", Body: "x"})
		if err != nil || out.Status != state {
			t.Fatalf("%s: %+v %v", state, out, err)
		}
	}
	host := &reportHost{status: workbench.BugReportPosting}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := awaitBugReport(ctx, host, "one", time.Now().Add(time.Minute))
	if err != nil || out.Status != workbench.BugReportUnknown {
		t.Fatalf("cancel after posting=%+v %v", out, err)
	}
}

func TestBugReportNotificationFailureKeepsWaiting(t *testing.T) {
	host := &reportHost{status: workbench.BugReportCreated}
	host.openErr = errors.New("notification unavailable")
	m := newWindowManager(t.TempDir(), testEditor, host)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := m.reportBug(ctx, reportBugInput{Title: "title", Body: "body"})
	if err != nil || out.Status != workbench.BugReportCreated || len(host.requests) != 1 || host.cancelled || host.polls != 1 {
		t.Fatalf("result=%+v err=%v host=%+v", out, err, host)
	}
	if deadline, ok := ctx.Deadline(); !ok || !host.requests[0].Deadline.Equal(deadline) {
		t.Fatal("earlier caller deadline was not retained")
	}
}
