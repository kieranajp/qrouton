package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/vault"
)

type ollamaEvents struct {
	mu   sync.Mutex
	seen []OllamaStatus
	ch   chan OllamaStatus
}

func newOllamaEvents() *ollamaEvents { return &ollamaEvents{ch: make(chan OllamaStatus, 1024)} }

func (e *ollamaEvents) emit(name string, data any) {
	if name != ollamaEvent {
		return
	}
	e.mu.Lock()
	if len(e.seen) > 0 && len(e.seen)%4096 == 0 {
		e.seen = e.seen[len(e.seen)-1:]
	}
	e.seen = append(e.seen, data.(OllamaStatus))
	e.mu.Unlock()
	select {
	case e.ch <- data.(OllamaStatus):
	default:
	}
}

func (e *ollamaEvents) until(t *testing.T, state string) OllamaStatus {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case s := <-e.ch:
			if s.State == state {
				return s
			}
		case <-timeout:
			t.Fatalf("no %s status arrived", state)
		}
	}
}

func (e *ollamaEvents) all() []OllamaStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]OllamaStatus(nil), e.seen...)
}

// ollamaStub answers /api/tags with the model once pulled is set, and /api/pull with pull.
type ollamaStub struct {
	pulled atomic.Bool
	pulls  atomic.Int32
	pull   func(w http.ResponseWriter, r *http.Request)
}

func (s *ollamaStub) serve(t *testing.T, installed bool) (*Ollama, *ollamaEvents) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			if s.pulled.Load() {
				fmt.Fprintf(w, `{"models":[{"name":%q,"digest":"abc"}]}`, vault.OllamaModel)
				return
			}
			_, _ = w.Write([]byte(`{"models":[]}`))
		case "/api/pull":
			s.pulls.Add(1)
			s.pull(w, r)
		}
	}))
	t.Cleanup(server.Close)
	events := newOllamaEvents()
	client := vault.NewOllama()
	client.URL = server.URL
	return newOllama(client, vault.OllamaModel, events.emit, func() bool { return installed }), events
}

func pullLines(w http.ResponseWriter, lines ...string) {
	for _, line := range lines {
		_, _ = w.Write([]byte(line + "\n"))
		w.(http.Flusher).Flush()
	}
}

func TestOllamaCheckTellsUnreachableMissingAndReady(t *testing.T) {
	for _, installed := range []bool{true, false} {
		closed := httptest.NewServer(http.NotFoundHandler())
		closed.Close()
		client := vault.NewOllama()
		client.URL = closed.URL
		o := newOllama(client, vault.OllamaModel, func(string, any) {}, func() bool { return installed })
		if got := o.Check(); got.State != ollamaStateUnreachable || got.Installed != installed {
			t.Fatalf("no server answered %+v, want unreachable with installed=%v", got, installed)
		}
	}

	stub := &ollamaStub{}
	o, _ := stub.serve(t, true)
	if got := o.Check(); got.State != ollamaStateMissing || got.Installed {
		t.Fatalf("tags without the model answered %+v", got)
	}
	stub.pulled.Store(true)
	if got := o.Check(); got.State != ollamaStateReady || got.Model != vault.OllamaModel {
		t.Fatalf("tags with the model answered %+v", got)
	}
}

func TestOllamaPullReportsRisingProgressThenReady(t *testing.T) {
	stub := &ollamaStub{}
	stub.pull = func(w http.ResponseWriter, r *http.Request) {
		pullLines(w, `{"status":"pulling manifest"}`)
		for done := 0; done <= 1000; done += 2 {
			pullLines(w, fmt.Sprintf(`{"status":"pulling abc","total":1000,"completed":%d}`, done))
		}
		stub.pulled.Store(true)
		pullLines(w, `{"status":"success"}`)
	}
	o, events := stub.serve(t, true)
	o.Pull()
	events.until(t, ollamaStateReady)

	var last int64 = -1
	pulling := 0
	for _, s := range events.all() {
		if s.State != ollamaStatePulling {
			continue
		}
		pulling++
		if s.Completed < last {
			t.Fatalf("progress went backwards: %d after %d", s.Completed, last)
		}
		last = s.Completed
	}
	if last != 1000 {
		t.Fatalf("last progress = %d, want 1000", last)
	}
	if pulling > 105 {
		t.Fatalf("emitted %d progress events for 502 lines, want one per percent at most", pulling)
	}
}

func TestOllamaPullFailsWithTheStreamError(t *testing.T) {
	stub := &ollamaStub{}
	stub.pull = func(w http.ResponseWriter, r *http.Request) {
		pullLines(w, `{"status":"pulling manifest"}`, `{"error":"pull model manifest: file does not exist"}`)
	}
	o, events := stub.serve(t, true)
	o.Pull()
	got := events.until(t, ollamaStateFailed)
	if !strings.Contains(got.Reason, "file does not exist") {
		t.Fatalf("failed with %q", got.Reason)
	}
}

func TestOllamaCancelPullStopsTheRequestAndAnswersMissing(t *testing.T) {
	stub := &ollamaStub{}
	started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	stub.pull = func(w http.ResponseWriter, r *http.Request) {
		pullLines(w, `{"status":"pulling abc","total":100,"completed":1}`)
		close(started)
		select {
		case <-r.Context().Done():
			close(stopped)
		case <-release:
		}
	}
	o, events := stub.serve(t, true)
	defer close(release)
	o.Pull()
	<-started
	o.Pull()

	done := make(chan OllamaStatus, 1)
	go func() { done <- o.snapshot() }()
	select {
	case got := <-done:
		if got.State != ollamaStatePulling {
			t.Fatalf("status during a stalled pull = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("a stalled stream held the service lock")
	}
	if got := o.Check(); got.State != ollamaStatePulling {
		t.Fatalf("Check during a pull answered %+v", got)
	}

	o.CancelPull()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling did not stop the request")
	}
	if got := o.snapshot(); got.State != ollamaStateMissing {
		t.Fatalf("status after cancel = %+v", got)
	}
	events.until(t, ollamaStateMissing)
	if n := stub.pulls.Load(); n != 1 {
		t.Fatalf("a second Pull during one made %d requests", n)
	}
}

// fakePull streams progress until cancelled and answers with ctx.Err(), as a
// loopback client does.
type fakePull struct{ pulls atomic.Int32 }

func (f *fakePull) Model(context.Context) (vault.Model, error) {
	return vault.Model{}, vault.ErrModelMissing
}

func (f *fakePull) Pull(ctx context.Context, onProgress func(vault.PullProgress)) error {
	f.pulls.Add(1)
	for done := int64(0); ctx.Err() == nil; done = (done + 1) % 100 {
		onProgress(vault.PullProgress{Status: "pulling abc", Completed: done, Total: 100})
	}
	return ctx.Err()
}

func TestOllamaCancelAlwaysEndsMissingAndTheLastEventMatches(t *testing.T) {
	for i := 0; i < 2000; i++ {
		events := newOllamaEvents()
		o := newOllama(&fakePull{}, vault.OllamaModel, events.emit, nil)
		o.Pull()
		events.until(t, ollamaStatePulling)
		o.CancelPull()
		time.Sleep(time.Millisecond)
		seen := events.all()
		if got := o.snapshot(); got.State != ollamaStateMissing || seen[len(seen)-1] != got {
			t.Fatalf("run %d: status %+v, last event %+v", i, got, seen[len(seen)-1])
		}
	}
}

func TestOllamaPullAfterACancelIsNotOverwrittenByTheOldOne(t *testing.T) {
	client := &fakePull{}
	events := newOllamaEvents()
	o := newOllama(client, vault.OllamaModel, events.emit, nil)
	o.Pull()
	o.CancelPull()
	o.Pull()
	time.Sleep(10 * time.Millisecond)
	if got := o.snapshot(); got.State != ollamaStatePulling {
		t.Fatalf("the second pull reads %+v", got)
	}
	if n := client.pulls.Load(); n != 2 {
		t.Fatalf("made %d pull requests, want 2", n)
	}
	o.CancelPull()
}
