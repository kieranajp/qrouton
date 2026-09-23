package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

const stubDimension = 512

// concepts lets the stub embedder see through the synthetic corpus's
// paraphrases, the way a real model would, while BM25 cannot.
var concepts = map[string]string{
	"kick": "logout", "logg": "logout", "renew": "refresh", "credential": "token",
	"call": "request", "once": "concurrent", "parallel": "concurrent",
	"ship": "release", "build": "version", "slice": "percentage", "customer": "user",
	"phone": "device", "handset": "device", "signal": "offline",
	"use": "expiry", "best": "expiry", "stamp": "date", "packag": "label",
	"healthy": "crash",
}

var errStubDown = errors.New("stub embedder is down")
var errPoisoned = fmt.Errorf("%w: stub embedder rejects this input", ErrRejected)

type stubEmbedder struct {
	mu     sync.Mutex
	limit  int
	down   bool
	digest string
	gate   chan struct{}
	poison string
	seen   []string
	calls  int
	inputs int
}

func (e *stubEmbedder) Model(context.Context) (Model, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.down {
		return Model{}, errStubDown
	}
	digest := e.digest
	if digest == "" {
		digest = "stub"
	}
	return Model{Name: OllamaModel, Digest: digest}, nil
}

func (e *stubEmbedder) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	e.mu.Lock()
	gate, down, limit := e.gate, e.down, e.limit
	e.calls++
	e.inputs += len(inputs)
	e.seen = append(e.seen, inputs...)
	poison := e.poison
	e.mu.Unlock()
	if gate != nil && len(inputs) > 1 {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if down {
		return nil, errStubDown
	}
	out := make([][]float32, len(inputs))
	for i, input := range inputs {
		if poison != "" && strings.Contains(input, poison) {
			return nil, errPoisoned
		}
		if limit > 0 && len(input) > limit {
			return nil, ErrContextOverflow
		}
		out[i] = conceptVector(input)
	}
	return out, nil
}

func (e *stubEmbedder) set(fn func(*stubEmbedder)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(e)
}

func (e *stubEmbedder) count() (calls, inputs int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls, e.inputs
}

func conceptVector(text string) []float32 {
	v := make([]float32, stubDimension)
	for _, t := range tokens(text) {
		if c, ok := concepts[t]; ok {
			t = c
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(t))
		v[h.Sum32()%stubDimension]++
	}
	return v
}

// copyTree copies the Markdown under src into a fresh directory, skipping the
// named top-level directories.
func copyTree(t *testing.T, src string, excluded ...string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if entry.IsDir() {
			if slices.Contains(excluded, rel) {
				return fs.SkipDir
			}
			return nil
		}
		if !markdownFile(path) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dst, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func newTestService(t *testing.T, root string, e Embedder) *Service {
	t.Helper()
	s, err := New(Profile{ID: "test", Root: root}, e, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.interval = 0
	return s
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

type evalQuery struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Text     string   `json:"text"`
	Relevant []string `json:"relevant"`
}

type evalFile struct {
	ExcludedDirs []string    `json:"excluded_dirs"`
	Queries      []evalQuery `json:"queries"`
}

func loadQueries(t *testing.T, path string) evalFile {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f evalFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// evalRun is one query's rankings: each channel's full document order, RRF's
// order, and the documents search_thoughts actually returned.
type evalRun struct {
	query                 evalQuery
	lexical, dense, fused []string
	returned              []Hit
}

func runQuery(t *testing.T, s *Service, q evalQuery) evalRun {
	t.Helper()
	ctx := context.Background()
	res, err := s.Search(ctx, Query{Text: q.Text})
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := s.embedder.Embed(ctx, []string{q.Text})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lexical, dense := s.channelsLocked(Query{Text: q.Text}, vectors[0])
	names := func(ids []int) []string {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = s.entries[id].name
		}
		return out
	}
	return evalRun{query: q, lexical: names(entries(lexical)), dense: names(entries(dense)),
		fused: names(rrfOrder(entries(lexical), entries(dense))), returned: res.Hits}
}

func (r evalRun) returnedNames() []string {
	out := make([]string, len(r.returned))
	for i, h := range r.returned {
		_, out[i], _ = strings.Cut(h.Ref, refSeparator)
	}
	return out
}

func recallAt(list []string, relevant []string, k int) float64 {
	found := 0
	for _, want := range relevant {
		if i := slices.Index(list, want); i >= 0 && (k == 0 || i < k) {
			found++
		}
	}
	return float64(found) / float64(len(relevant))
}

func reciprocalRank(list []string, relevant []string) float64 {
	for i, name := range list {
		if slices.Contains(relevant, name) {
			return 1 / float64(i+1)
		}
	}
	return 0
}

type evalSummary struct {
	queries                          int
	lexicalR5, denseR5, rrfR5, fused float64
	lexicalMRR, denseMRR, rrfMRR     float64
	returned                         float64
}

func summarise(runs []evalRun) evalSummary {
	var s evalSummary
	for _, r := range runs {
		if len(r.query.Relevant) == 0 {
			continue
		}
		rel := r.query.Relevant
		s.queries++
		s.lexicalR5 += recallAt(r.lexical, rel, 5)
		s.denseR5 += recallAt(r.dense, rel, 5)
		s.rrfR5 += recallAt(r.fused, rel, 5)
		s.fused += recallAt(r.returnedNames(), rel, 0)
		s.lexicalMRR += reciprocalRank(r.lexical, rel)
		s.denseMRR += reciprocalRank(r.dense, rel)
		s.rrfMRR += reciprocalRank(r.fused, rel)
		s.returned += float64(len(r.returned))
	}
	if n := float64(s.queries); n > 0 {
		for _, f := range []*float64{&s.lexicalR5, &s.denseR5, &s.rrfR5, &s.fused, &s.lexicalMRR, &s.denseMRR, &s.rrfMRR, &s.returned} {
			*f /= n
		}
	}
	return s
}

func (s evalSummary) log(t *testing.T) {
	t.Helper()
	t.Logf("%d present-topic queries, %.1f documents returned on average", s.queries, s.returned)
	t.Logf("%-7s %6s %6s", "channel", "R@5", "MRR")
	t.Logf("%-7s %6.3f %6.3f", "bm25", s.lexicalR5, s.lexicalMRR)
	t.Logf("%-7s %6.3f %6.3f", "dense", s.denseR5, s.denseMRR)
	t.Logf("%-7s %6.3f %6.3f", "rrf", s.rrfR5, s.rrfMRR)
	t.Logf("fused (rrf top-5 + channel top-3) R@5 %.3f", s.fused)
}
