package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const corpusDir = "testdata/corpus"

func search(t *testing.T, s *Service, q Query) Result {
	t.Helper()
	res, err := s.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func hitFor(res Result, name string) (Hit, bool) {
	for _, h := range res.Hits {
		if strings.HasSuffix(h.Ref, refSeparator+name) {
			return h, true
		}
	}
	return Hit{}, false
}

func TestSyntheticCorpusRanksEveryPresentTopic(t *testing.T) {
	s := newTestService(t, copyTree(t, corpusDir), &stubEmbedder{})
	s.Warm(context.Background())
	var runs []evalRun
	for _, q := range loadQueries(t, "testdata/queries.json").Queries {
		run := runQuery(t, s, q)
		runs = append(runs, run)
		if len(q.Relevant) == 0 {
			if len(run.returned) == 0 {
				t.Errorf("%s: an absent topic returned nothing, but search has no floor to hide neighbours behind", q.ID)
			}
			continue
		}
		if got := recallAt(run.returnedNames(), q.Relevant, 0); got != 1 {
			t.Errorf("%s: returned %v, missing %v", q.ID, run.returnedNames(), q.Relevant)
		}
	}
	summary := summarise(runs)
	summary.log(t)
	if summary.denseR5 <= summary.lexicalR5 {
		t.Errorf("dense R@5 %.3f should beat BM25 %.3f on the paraphrased queries", summary.denseR5, summary.lexicalR5)
	}
}

func TestSearchReportsRawChannelEvidence(t *testing.T) {
	s := newTestService(t, copyTree(t, corpusDir), &stubEmbedder{})
	res := search(t, s, Query{Text: "ZXing decoder latency"})
	if res.Status != (Status{Lexical: StatusReady, Semantic: StatusReady}) {
		t.Fatalf("status = %+v", res.Status)
	}
	top := res.Hits[0]
	if top.ID != "pantry-sync/R1-2026-01-10-barcode-decoder-latency" || top.Kind != KindResearch || top.Profile != "test" {
		t.Fatalf("top hit = %+v", top)
	}
	if top.BM25 == nil || top.BM25.Rank != 1 || top.Dense == nil || top.FusedRank != 1 || top.AdmittedBy != AdmittedRRF {
		t.Fatalf("top hit evidence = %+v bm25=%+v dense=%+v", top, top.BM25, top.Dense)
	}
	if top.Breadcrumb != "Barcode decoder latency > Summary" || top.Line != 15 || !strings.Contains(top.Snippet, "ZXing") {
		t.Fatalf("top hit location = %q lines %d-%d snippet %q", top.Breadcrumb, top.Line, top.Through, top.Snippet)
	}
}

func TestKindsNarrowBothChannels(t *testing.T) {
	s := newTestService(t, copyTree(t, corpusDir), &stubEmbedder{})
	res := search(t, s, Query{Text: "release rollout checklist calendar", Kinds: []string{KindResearch}})
	if len(res.Hits) == 0 {
		t.Fatal("no research hits")
	}
	for _, h := range res.Hits {
		if h.Kind != KindResearch {
			t.Errorf("research-only search returned %s (%q)", h.ID, h.Kind)
		}
	}
	notes := search(t, s, Query{Text: "pantry glossary household", Kinds: []string{"note"}})
	if len(notes.Hits) != 1 || notes.Hits[0].ID != "pantry-sync/notes/pantry-glossary" {
		t.Fatalf("note search = %+v", notes.Hits)
	}
}

func TestFuseAdmitsEachChannelsTopThree(t *testing.T) {
	got := fuse([]int{20, 21, 22}, []int{1, 2, 3, 4, 5, 6}, 2)
	want := []admission{
		{entry: 1, fused: 1, by: AdmittedRRF},
		{entry: 20, fused: 2, by: AdmittedRRF},
		{entry: 21, fused: 4, by: AdmittedBM25},
		{entry: 22, fused: 6, by: AdmittedBM25},
		{entry: 2, fused: 3, by: AdmittedDense},
		{entry: 3, fused: 5, by: AdmittedDense},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("fuse = %+v\nwant %+v", got, want)
	}
	if got := fuse([]int{0, 1, 2}, []int{0, 1, 2}, 5); len(got) != 3 {
		t.Fatalf("agreeing channels admitted extras: %+v", got)
	}
}

func TestFreshnessFollowsEditsDeletesAndAdds(t *testing.T) {
	root := copyTree(t, corpusDir)
	s := newTestService(t, root, &stubEmbedder{})
	s.Warm(context.Background())
	name := "pantry-sync/research/R1-2026-01-10-barcode-decoder-latency.md"
	path := filepath.Join(root, name)
	b, _ := os.ReadFile(path)
	writeFile(t, path, strings.ReplaceAll(string(b), "ZXing", "Quagga2"))
	hit, ok := hitFor(search(t, s, Query{Text: "Quagga2 decoder"}), name)
	if !ok || !strings.Contains(hit.Snippet, "Quagga2") || hit.BM25 == nil || hit.BM25.Rank != 1 {
		t.Fatalf("edited document hit = %+v (found %v)", hit, ok)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := hitFor(search(t, s, Query{Text: "Quagga2 decoder"}), name); ok {
		t.Fatal("a deleted document is still returned")
	}
	if _, err := s.Read(ReadRequest{Ref: "test:" + name}); !errors.Is(err, ErrUnknownDocument) {
		t.Fatalf("read of a deleted document = %v", err)
	}

	added := "pantry-sync/research/R3-2026-01-20-freezer-inventory.md"
	writeFile(t, filepath.Join(root, added), "# Freezer inventory\n\nThe freezer drawer tracks frozen peas.\n")
	res := search(t, s, Query{Text: "frozen peas freezer"})
	hit, ok = hitFor(res, added)
	if !ok || hit.Dense == nil || res.Status.Semantic != StatusReady {
		t.Fatalf("added document hit = %+v status %+v", hit, res.Status)
	}
}

func TestWalkIsDebounced(t *testing.T) {
	root := copyTree(t, corpusDir)
	s := newTestService(t, root, &stubEmbedder{})
	s.interval = time.Hour
	search(t, s, Query{Text: "anything"})
	writeFile(t, filepath.Join(root, "new.md"), "# Porcupine\n")
	if _, ok := hitFor(search(t, s, Query{Text: "porcupine"}), "new.md"); ok {
		t.Fatal("a second search inside the interval walked the vault again")
	}
}

func TestSearchFallsBackToBM25AndRecovers(t *testing.T) {
	e := &stubEmbedder{down: true}
	s := newTestService(t, copyTree(t, corpusDir), e)
	res := search(t, s, Query{Text: "Stripe chargeback webhooks"})
	if res.Status.Lexical != StatusReady || res.Status.Semantic != StatusUnavailable || res.Status.Reason == "" {
		t.Fatalf("status = %+v", res.Status)
	}
	if len(res.Hits) == 0 || res.Hits[0].ID != "billing/R1-2026-05-01-chargeback-webhooks" {
		t.Fatalf("BM25-only hits = %+v", res.Hits)
	}
	for _, h := range res.Hits {
		if h.Dense != nil || h.AdmittedBy == AdmittedDense {
			t.Fatalf("BM25-only hit carries dense evidence: %+v", h)
		}
	}

	e.set(func(e *stubEmbedder) { e.down = false })
	res = search(t, s, Query{Text: "Stripe chargeback webhooks"})
	if res.Status.Semantic != StatusReady || res.Hits[0].Dense == nil {
		t.Fatalf("search did not pick Ollama back up: %+v", res.Status)
	}
}

func TestLargeBacklogBuildsInTheBackground(t *testing.T) {
	root := t.TempDir()
	for i := range 80 {
		writeFile(t, filepath.Join(root, fmt.Sprintf("note-%02d.md", i)), fmt.Sprintf("# Note %d\n\nParagraph about topic %d.\n", i, i))
	}
	gate := make(chan struct{})
	e := &stubEmbedder{gate: gate}
	s := newTestService(t, root, e)
	res := search(t, s, Query{Text: "topic 7"})
	if res.Status.Semantic != StatusPartial || res.Status.Pending != 80 {
		t.Fatalf("status during build = %+v", res.Status)
	}
	if len(res.Hits) == 0 {
		t.Fatal("a partial build answered nothing; BM25 should still rank")
	}
	close(gate)
	deadline := time.Now().Add(5 * time.Second)
	for {
		res = search(t, s, Query{Text: "topic 7"})
		if res.Status.Semantic == StatusReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background build never finished: %+v", res.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestEmbeddingCacheSurvivesRestartAndCorruption(t *testing.T) {
	root := copyTree(t, corpusDir)
	cacheDir := t.TempDir()
	profile := Profile{ID: "test", Root: root}
	first, _ := New(profile, &stubEmbedder{}, cacheDir)
	first.Warm(context.Background())

	counting := &stubEmbedder{}
	second, _ := New(profile, counting, cacheDir)
	second.Warm(context.Background())
	if calls, _ := counting.count(); calls != 0 {
		t.Fatalf("a warm cache still embedded %d batches", calls)
	}

	if err := os.WriteFile(filepath.Join(cacheDir, "test"+cacheExtension), []byte("not gob"), 0o644); err != nil {
		t.Fatal(err)
	}
	rebuilt := &stubEmbedder{}
	third, _ := New(profile, rebuilt, cacheDir)
	third.Warm(context.Background())
	if _, inputs := rebuilt.count(); inputs == 0 {
		t.Fatal("a corrupt cache was trusted instead of rebuilt")
	}

	redigested := &stubEmbedder{digest: "other"}
	fourth, _ := New(profile, redigested, cacheDir)
	fourth.Warm(context.Background())
	if _, inputs := redigested.count(); inputs == 0 {
		t.Fatal("vectors from another model digest were reused")
	}
}

func TestVaultStaysInsideItsRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.md"), "# Secret\n\nplatypus\n")
	writeFile(t, filepath.Join(outside, "dir", "hidden.md"), "# Hidden\n\nplatypus\n")
	writeFile(t, filepath.Join(root, "real.md"), "# Real\n\nplatypus inside\n")
	for link, target := range map[string]string{
		"leak.md":  filepath.Join(outside, "secret.md"),
		"dirlink":  filepath.Join(outside, "dir"),
		"alias.md": "real.md",
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	s := newTestService(t, root, &stubEmbedder{})
	var refs []string
	for _, h := range search(t, s, Query{Text: "platypus"}).Hits {
		refs = append(refs, h.Ref)
	}
	slices.Sort(refs)
	if want := []string{"test:alias.md", "test:real.md"}; !slices.Equal(refs, want) {
		t.Fatalf("hits = %v, want %v", refs, want)
	}
	for _, ref := range []string{"test:leak.md", "test:../" + filepath.Base(outside) + "/secret.md", "test:dirlink/hidden.md"} {
		if _, err := s.Read(ReadRequest{Ref: ref}); !errors.Is(err, ErrUnknownDocument) {
			t.Errorf("read %s = %v", ref, err)
		}
	}
	for _, name := range []string{"leak.md", "../secret.md", "dirlink/hidden.md", "/etc/hosts"} {
		if _, err := readDocument(root, name); err == nil {
			t.Errorf("readDocument(%q) escaped the root", name)
		}
	}
}

func TestMissingRootLeavesBothChannelsUnavailable(t *testing.T) {
	s := newTestService(t, filepath.Join(t.TempDir(), "gone"), &stubEmbedder{})
	res := search(t, s, Query{Text: "anything"})
	if res.Status.Lexical != StatusUnavailable || len(res.Hits) != 0 {
		t.Fatalf("missing root = %+v", res)
	}
}

func TestReadAnswersRangesByRefOrID(t *testing.T) {
	root := copyTree(t, corpusDir)
	s := newTestService(t, root, &stubEmbedder{})
	whole, err := s.Read(ReadRequest{Ref: "login-flow/R1-2026-02-02-token-refresh-races"})
	if err != nil {
		t.Fatal(err)
	}
	if whole.Ref != "test:login-flow/research/R1-2026-02-02-token-refresh-races.md" || whole.Line != 1 || whole.Through != whole.Lines || !strings.HasPrefix(whole.Content, "---\n") {
		t.Fatalf("whole read = %+v", whole)
	}
	part, err := s.Read(ReadRequest{Ref: whole.Ref, Line: 13, Through: 15})
	if err != nil {
		t.Fatal(err)
	}
	if part.Content != "# Token refresh races\n\n## Symptom\n" || part.Line != 13 || part.Through != 15 {
		t.Fatalf("range read = %q (%d-%d)", part.Content, part.Line, part.Through)
	}

	writeFile(t, filepath.Join(root, "long.md"), "# Long\n\n"+strings.Repeat("word ", readLimit))
	long, err := s.Read(ReadRequest{Ref: "test:long.md"})
	if err != nil || !long.Truncated || len(long.Content) > readLimit {
		t.Fatalf("long read truncated=%v len=%d err=%v", long.Truncated, len(long.Content), err)
	}
	full, err := s.Read(ReadRequest{Ref: "test:long.md", Full: true})
	if err != nil || full.Truncated || len(full.Content) <= readLimit {
		t.Fatalf("full read truncated=%v len=%d err=%v", full.Truncated, len(full.Content), err)
	}
}

func TestNewRejectsUnsafeProfileIDs(t *testing.T) {
	for _, id := range []string{"", "../x", "a/b", ".hidden"} {
		if _, err := New(Profile{ID: id, Root: "/tmp"}, &stubEmbedder{}, t.TempDir()); !errors.Is(err, ErrInvalidProfile) {
			t.Errorf("New(%q) = %v", id, err)
		}
	}
}

func TestSearchRejectsUnknownKindsAndCapsTheLimit(t *testing.T) {
	s := newTestService(t, copyTree(t, corpusDir), &stubEmbedder{})
	if _, err := s.Search(context.Background(), Query{Text: "x", Kinds: []string{"reseach"}}); !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("misspelt kind = %v", err)
	}
	if res := search(t, s, Query{Text: "the", Limit: 1000}); len(res.Hits) > maxLimit+2*channelTop {
		t.Fatalf("limit 1000 returned %d hits", len(res.Hits))
	}
	if _, err := New(Profile{ID: "p", Root: "relative/root"}, &stubEmbedder{}, t.TempDir()); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("relative root = %v", err)
	}
}

func TestAWindowThatKeepsFailingIsSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "good.md"), "# Good\n\nwombat\n")
	writeFile(t, filepath.Join(root, "bad.md"), "# Bad\n\npoisoned wombat\n")
	s := newTestService(t, root, &stubEmbedder{poison: "poisoned"})
	var res Result
	for range maxFailures {
		res = search(t, s, Query{Text: "wombat"})
	}
	if res.Status.Semantic != StatusReady || res.Status.Skipped != 1 || res.Status.Pending != 0 {
		t.Fatalf("status after repeated failures = %+v", res.Status)
	}
	if hit, ok := hitFor(res, "good.md"); !ok || hit.Dense == nil {
		t.Fatalf("the good window lost its embedding: %+v", res.Hits)
	}
}
