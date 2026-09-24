package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		name            string
		latest, current string
		want            bool
	}{
		{"equal", "1.2.3", "1.2.3", false},
		{"patch bump", "1.2.4", "1.2.3", true},
		{"minor bump", "1.3.0", "1.2.9", true},
		{"major bump", "2.0.0", "1.9.9", true},
		{"v prefix", "v1.2.3", "1.2.2", true},
		{"two-part tag", "1.5", "1.4.0", true},
		{"older latest", "1.0.0", "2.0.0", false},
		{"non-numeric tag", "not-a-version", "1.0.0", false},
		{"empty current", "1.0.0", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := newer(c.latest, c.current); got != c.want {
				t.Fatalf("newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
			}
		})
	}
}

// recordingEmitter is a bare emitter that records every event fired at it, so
// a test can assert both that one landed and that a later check stayed quiet.
type recordingEmitter struct {
	mu     sync.Mutex
	events []UpdateStatus
}

func (r *recordingEmitter) emit(event string, payload any) {
	if event != updateEvent {
		return
	}
	status, ok := payload.(UpdateStatus)
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, status)
}

func (r *recordingEmitter) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func TestUpdatesEmptyVersionNeverRequests(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `{"tag_name":"v9.9.9"}`)
	}))
	defer server.Close()

	rec := &recordingEmitter{}
	updates := newUpdates("", rec.emit, server.Client(), server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	updates.watch(ctx, time.Millisecond)

	if hits.Load() != 0 {
		t.Fatalf("empty version made %d requests, want 0", hits.Load())
	}
	if rec.count() != 0 {
		t.Fatalf("empty version emitted %d times, want 0", rec.count())
	}
}

func TestUpdatesCheckFindsANewerRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.4.0","html_url":"https://example.com/releases/v1.4.0"}`)
	}))
	defer server.Close()

	rec := &recordingEmitter{}
	updates := newUpdates("1.0.0", rec.emit, server.Client(), server.URL)
	updates.check(context.Background())

	status := updates.Status()
	if !status.Available || status.Latest != "1.4.0" || status.Current != "1.0.0" {
		t.Fatalf("status = %+v", status)
	}
	if status.URL != "https://example.com/releases/v1.4.0" {
		t.Fatalf("status = %+v", status)
	}
	if runtime.GOOS == "darwin" {
		if status.Command != updateBrewCommand {
			t.Fatalf("darwin status = %+v", status)
		}
	} else if status.Command != "" {
		t.Fatalf("non-darwin status = %+v", status)
	}
	if rec.count() != 1 {
		t.Fatalf("emitted %d times, want 1", rec.count())
	}

	updates.check(context.Background())
	if rec.count() != 1 {
		t.Fatalf("a second identical answer emitted %d times, want 1", rec.count())
	}
}

func TestUpdatesCheckLeavesStatusOnFailure(t *testing.T) {
	seedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.4.0"}`)
	}))
	defer seedServer.Close()
	rec := &recordingEmitter{}
	updates := newUpdates("1.0.0", rec.emit, seedServer.Client(), seedServer.URL)
	updates.check(context.Background())
	want := updates.Status()
	if !want.Available {
		t.Fatalf("seed check did not register an update: %+v", want)
	}

	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"500", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }},
		{"malformed body", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{not json`) }},
		{"hangs past timeout", func(w http.ResponseWriter, r *http.Request) { time.Sleep(200 * time.Millisecond) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(c.handler)
			defer server.Close()
			updates.endpoint = server.URL
			rec.mu.Lock()
			rec.events = nil
			rec.mu.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			updates.check(ctx)

			if got := updates.Status(); got != want {
				t.Fatalf("status after %s = %+v, want unchanged %+v", c.name, got, want)
			}
			if rec.count() != 0 {
				t.Fatalf("%s emitted %d times, want 0", c.name, rec.count())
			}
		})
	}
}

func TestUpdatesWatchReturnsWhenCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v0.0.1"}`)
	}))
	defer server.Close()

	updates := newUpdates("1.0.0", func(string, any) {}, server.Client(), server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		updates.watch(ctx, time.Hour)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not return after its context was cancelled")
	}
}
