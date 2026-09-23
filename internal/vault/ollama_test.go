package vault

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbedAllSplitsOverflowingInputs(t *testing.T) {
	e := &stubEmbedder{limit: 300}
	long := strings.Repeat("line of text\n", 60)
	got, err := embedAll(context.Background(), e, []string{"short", long, "also short"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || len(got[0]) != 1 || len(got[2]) != 1 {
		t.Fatalf("vector counts = %d %d %d", len(got[0]), len(got[1]), len(got[2]))
	}
	if len(got[1]) < 3 {
		t.Fatalf("overflowing input produced %d vectors", len(got[1]))
	}
	if _, err := embedAll(context.Background(), &stubEmbedder{down: true}, []string{"x"}); !errors.Is(err, errStubDown) {
		t.Fatalf("other failures pass through: %v", err)
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
	if vectors := s.vectors[s.entries[0].chunks[0].windows[0].hash]; len(vectors) < 2 {
		t.Fatalf("window kept %d vectors", len(vectors))
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
