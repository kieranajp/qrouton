package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEmbedAllSplitsOverflowingWindowsUnderTheirBreadcrumb(t *testing.T) {
	e := &stubEmbedder{limit: 300}
	long := strings.Repeat("line of text\n", 60)
	got, err := embedAll(context.Background(), e, []window{newWindow("A", "short"), newWindow("Doc > Long", long), newWindow("C", "also short")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || len(got[0]) != 1 || len(got[2]) != 1 {
		t.Fatalf("vector counts = %d %d %d", len(got[0]), len(got[1]), len(got[2]))
	}
	if len(got[1]) < 3 {
		t.Fatalf("overflowing window produced %d vectors", len(got[1]))
	}
	for _, input := range e.seen {
		if strings.Contains(input, "line of text") && !strings.HasPrefix(input, "Doc > Long\n\n") {
			t.Fatalf("a split half lost its breadcrumb: %q", input[:30])
		}
	}
	if _, err := embedAll(context.Background(), &stubEmbedder{down: true}, []window{newWindow("A", "x"), newWindow("B", "y")}); !errors.Is(err, errStubDown) {
		t.Fatalf("an unreachable Ollama must fail the call: %v", err)
	}
	got, err = embedAll(context.Background(), &stubEmbedder{poison: "bad"}, []window{newWindow("A", "good"), newWindow("B", "bad")})
	if err != nil || got[0] == nil || got[1] != nil {
		t.Fatalf("one bad window = %v, %v", got, err)
	}
}

func TestOverflowingWindowsStillReachDenseRanking(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/wide.md", "# Wide\n\n"+strings.Repeat("wombat burrow ", 70)+"\n")
	s := newTestService(t, root, &stubEmbedder{limit: 400})
	res := search(t, s, Query{Text: "wombat"})
	if res.Status.Semantic != StatusReady || len(res.Hits) != 1 || res.Hits[0].Dense == nil {
		t.Fatalf("result = %+v", res)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	most := 0
	for _, w := range s.entries[0].chunks[0].windows {
		most = max(most, len(s.vectors[w.hash]))
	}
	if most < 2 {
		t.Fatal("no overflowing window was split")
	}
}

func TestOllamaClientSpeaksTheAPI(t *testing.T) {
	var truncate any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case ollamaTags:
			_, _ = w.Write([]byte(`{"models":[{"name":"other","digest":"x"},{"name":"` + OllamaModel + `","digest":"abc"}]}`))
		case ollamaEmbed:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			truncate = body["truncate"]
			inputs := body["input"].([]any)
			if strings.Contains(inputs[0].(string), "huge") {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"the input length exceeds the context length"}`))
				return
			}
			_, _ = w.Write([]byte(`{"embeddings":[[1,0],[0,1]]}`))
		}
	}))
	defer server.Close()
	o := NewOllama()
	o.URL = server.URL
	ctx := context.Background()
	m, err := o.Model(ctx)
	if err != nil || m.Digest != "abc" {
		t.Fatalf("model = %+v, %v", m, err)
	}
	v, err := o.Embed(ctx, []string{"a", "b"})
	if err != nil || len(v) != 2 || truncate != false {
		t.Fatalf("embed = %v, %v, truncate %v", v, err, truncate)
	}
	if _, err := o.Embed(ctx, []string{"huge"}); !errors.Is(err, ErrContextOverflow) {
		t.Fatalf("overflow = %v", err)
	}
	if _, err := o.Embed(ctx, []string{"a"}); !errors.Is(err, ErrEmbeddingMissing) {
		t.Fatalf("short answer = %v", err)
	}
	o.Name = "absent"
	if _, err := o.Model(ctx); !errors.Is(err, ErrModelMissing) {
		t.Fatalf("missing model = %v", err)
	}
}

func TestAStalledEmbedTimesOutAndALaterSearchRetries(t *testing.T) {
	root := t.TempDir()
	for i := range 80 {
		writeFile(t, filepath.Join(root, fmt.Sprintf("note-%02d.md", i)), fmt.Sprintf("# Note %d\n\nParagraph about topic %d.\n", i, i))
	}
	var stall atomic.Bool
	stall.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case ollamaTags:
			_, _ = w.Write([]byte(`{"models":[{"name":"` + OllamaModel + `","digest":"abc"}]}`))
		case ollamaEmbed:
			var body struct {
				Input []string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if stall.Load() {
				<-r.Context().Done()
				return
			}
			vectors := make([][]float32, len(body.Input))
			for i := range vectors {
				vectors[i] = []float32{1, float32(i)}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": vectors})
		}
	}))
	defer server.Close()
	o := NewOllama()
	if o.Client.Timeout == 0 {
		t.Fatal("Ollama requests have no timeout")
	}
	o.URL = server.URL
	o.Client.Timeout = 50 * time.Millisecond
	s := newTestService(t, root, o)

	search(t, s, Query{Text: "topic 7"})
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		building := s.building
		s.mu.Unlock()
		if !building {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a stalled embed left the build running")
		}
		time.Sleep(10 * time.Millisecond)
	}

	stall.Store(false)
	for {
		res := search(t, s, Query{Text: "topic 7"})
		if res.Status.Semantic == StatusReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("a later search never rebuilt the index: %+v", res.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
