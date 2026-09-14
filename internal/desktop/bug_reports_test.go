package desktop

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func bugReportSetup(t *testing.T) (*BugReports, *sessionState, *chromeClock) {
	t.Helper()
	clock := &chromeClock{at: time.Now()}
	reg := newSessionsWithActivity(clock.now, time.Minute)
	s := reg.add(t.TempDir(), nil, nil)
	b := newBugReports(reg)
	b.write = func(context.Context, string, string) (github.Issue, error) {
		t.Error("unexpected write")
		return github.Issue{}, nil
	}
	return b, s, clock
}

func queueTestBugReport(t *testing.T, b *BugReports, s *sessionState, id string) workbench.BugReportRequest {
	t.Helper()
	req := workbench.BugReportRequest{ID: id, Title: "  broken preview  ", Body: "\n exact body \n", Deadline: b.now().Add(time.Minute)}
	if r, err := b.queue(s, req); err != nil || r.Status != workbench.BugReportPending {
		t.Fatalf("queue = %+v, %v", r, err)
	}
	return req
}

func awaitTestBugReport(t *testing.T, b *BugReports, s *sessionState, id string) workbench.BugReport {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r, err := b.status(s, id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Terminal() {
			return r
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("report did not finish")
	return workbench.BugReport{}
}

func TestBugReportNeedsConfirmationAndPostsOnce(t *testing.T) {
	b, s, _ := bugReportSetup(t)
	var calls atomic.Int32
	release, entered := make(chan struct{}), make(chan struct{})
	b.write = func(ctx context.Context, title, body string) (github.Issue, error) {
		if title != "broken preview" || body != "\n exact body \n" {
			t.Errorf("payload %q %q", title, body)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("write has no timeout")
		}
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return github.Issue{Number: 42, URL: github.IssueListURL + "/42"}, nil
	}
	req := queueTestBugReport(t, b, s, "one")
	if _, err := b.queue(s, req); err != nil {
		t.Fatal(err)
	}
	preview, err := b.Load(s.slug(), req.ID)
	if err != nil || preview.Title != "broken preview" || preview.Body != req.Body || preview.Destination != github.IssueRepository || calls.Load() != 0 {
		t.Fatalf("preview=%+v err=%v calls=%d", preview, err, calls.Load())
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := b.Confirm(s.slug(), req.ID); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	<-entered
	if r, err := b.Cancel(s.slug(), req.ID); err != nil || r.Status != workbench.BugReportPosting {
		t.Fatalf("cancel during write = %+v, %v", r, err)
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	close(release)
	r := awaitTestBugReport(t, b, s, req.ID)
	if r.Status != workbench.BugReportCreated || r.Number != 42 {
		t.Fatal(r)
	}
	if again, err := b.Confirm(s.slug(), req.ID); err != nil || again != r || calls.Load() != 1 {
		t.Fatalf("repeat = %+v, %v", again, err)
	}
}

func TestBugReportCancellationExpiryReplacementAndStop(t *testing.T) {
	for _, action := range []string{"cancel", "expire", "stop"} {
		t.Run(action, func(t *testing.T) {
			b, s, clock := bugReportSetup(t)
			req := queueTestBugReport(t, b, s, "one")
			changed := req
			changed.Body = "different"
			if _, err := b.queue(s, changed); !errors.Is(err, ErrBugReportInvalid) {
				t.Fatal(err)
			}
			changed.ID = "two"
			if _, err := b.queue(s, changed); !errors.Is(err, ErrBugReportBusy) {
				t.Fatal(err)
			}
			switch action {
			case "cancel":
				_, _ = b.Cancel(s.slug(), req.ID)
			case "expire":
				clock.advance(time.Minute)
			case "stop":
				s.stop()
			}
			r, err := b.Confirm(s.slug(), req.ID)
			if action == "stop" {
				if err == nil {
					t.Fatal("stopped session confirmed")
				}
				return
			}
			want := workbench.BugReportCancelled
			if action == "expire" {
				want = workbench.BugReportExpired
			}
			if err != nil || r.Status != want {
				t.Fatalf("confirm=%+v err=%v", r, err)
			}
			queueTestBugReport(t, b, s, "two")
			if _, err := b.Confirm(s.slug(), "one"); !errors.Is(err, ErrBugReportStale) {
				t.Fatal(err)
			}
			if retained, err := b.status(s, "one"); err != nil || retained.Status != want {
				t.Fatalf("retained=%+v %v", retained, err)
			}
			clock.advance(bugReportRetention)
			if _, err := b.status(s, "one"); !errors.Is(err, ErrBugReportStale) {
				t.Fatal(err)
			}
		})
	}
}

func TestBugReportConfirmCancelRace(t *testing.T) {
	for range 50 {
		b, s, _ := bugReportSetup(t)
		var calls atomic.Int32
		b.write = func(context.Context, string, string) (github.Issue, error) {
			calls.Add(1)
			return github.Issue{Number: 1, URL: github.IssueListURL + "/1"}, nil
		}
		queueTestBugReport(t, b, s, "race")
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _, _ = b.Confirm(s.slug(), "race") }()
		go func() { defer wg.Done(); <-start; _, _ = b.Cancel(s.slug(), "race") }()
		close(start)
		wg.Wait()
		r := awaitTestBugReport(t, b, s, "race")
		if (r.Status == workbench.BugReportCancelled && calls.Load() != 0) || (r.Status == workbench.BugReportCreated && calls.Load() != 1) {
			t.Fatalf("result=%+v calls=%d", r, calls.Load())
		}
	}
}

func TestBugReportWriterOutcomes(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want string
	}{
		{github.ErrNoToken, workbench.BugReportFailed}, {github.ErrIssueRejected, workbench.BugReportFailed},
		{github.ErrIssueUnknown, workbench.BugReportUnknown}, {errors.New("secret payload"), workbench.BugReportUnknown},
	} {
		b, s, _ := bugReportSetup(t)
		b.write = func(context.Context, string, string) (github.Issue, error) { return github.Issue{}, tt.err }
		queueTestBugReport(t, b, s, "one")
		_, _ = b.Confirm(s.slug(), "one")
		if r := awaitTestBugReport(t, b, s, "one"); r.Status != tt.want || r.Message == "secret payload" {
			t.Fatal(r)
		}
	}
}

type bugReportTransport func(*http.Request) (*http.Response, error)

func (f bugReportTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBugReportMissingCredentialMakesNoRequest(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	old := http.DefaultClient
	var calls atomic.Int32
	http.DefaultClient = &http.Client{Transport: bugReportTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("unexpected request")
	})}
	t.Cleanup(func() { http.DefaultClient = old })
	b, s, _ := bugReportSetup(t)
	b.write = writeBugReport
	queueTestBugReport(t, b, s, "one")
	_, _ = b.Confirm(s.slug(), "one")
	r := awaitTestBugReport(t, b, s, "one")
	if r.Status != workbench.BugReportFailed || calls.Load() != 0 {
		t.Fatalf("%+v calls=%d", r, calls.Load())
	}
}

func TestBugReportChromeHydratesAndExpiresOnlyOwnedPreview(t *testing.T) {
	clock := &chromeClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now, time.Minute)
	first := reg.add(sessionDir(t, root, "first"), nil, nil)
	second := reg.add(sessionDir(t, root, "second"), nil, nil)
	b := newBugReports(reg)
	queueTestBugReport(t, b, first, "one")
	reg.reveal(second)
	chrome := newChrome(func(string, any) {})
	push := func() status.Fields { pushChrome(reg, root, nil, nil, nil, chrome.publish); return chrome.Snapshot() }
	if push().BugReportID != "" {
		t.Fatal("foreign report in chrome")
	}
	reg.reveal(first)
	if fields := push(); fields.BugReportID != "one" || fields.BugReportStatus != workbench.BugReportPending {
		t.Fatal(fields)
	}
	clock.advance(time.Minute)
	if push().BugReportStatus != workbench.BugReportExpired {
		t.Fatal("expiry not published")
	}
	reg.reveal(second)
	clock.advance(bugReportRetention)
	push()
	if _, err := b.status(first, "one"); !errors.Is(err, ErrBugReportStale) {
		t.Fatal(err)
	}
}
