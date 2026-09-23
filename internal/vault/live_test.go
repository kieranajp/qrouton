package vault

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	liveEnv        = "QROUTON_VAULT_LIVE"
	evalQueriesEnv = "QROUTON_VAULT_EVAL_QUERIES"
	evalRootEnv    = "QROUTON_VAULT_EVAL_ROOT"
	evalFloor      = 0.90
)

func TestLiveOllama(t *testing.T) {
	if os.Getenv(liveEnv) == "" {
		t.Skipf("set %s=1 to talk to a local Ollama", liveEnv)
	}
	ctx := context.Background()
	o := NewOllama()
	m, err := o.Model(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("model %s digest %s", m.Name, m.Digest)
	vectors, err := o.Embed(ctx, []string{"barcode scanner", "token refresh", "staged rollout"})
	if err != nil || len(vectors) != 3 || len(vectors[0]) == 0 {
		t.Fatalf("batch embed = %d vectors, %v", len(vectors), err)
	}
	t.Logf("batch of 3 embedded at dimension %d", len(vectors[0]))
	oversized := strings.Repeat("oversized window text\n", 150)
	if _, err := o.Embed(ctx, []string{"fits", oversized}); !errors.Is(err, ErrContextOverflow) {
		t.Fatalf("oversized input with truncate false = %v, want overflow", err)
	}
	split, err := embedAll(ctx, o, []string{"fits", oversized})
	if err != nil {
		t.Fatal(err)
	}
	if len(split[0]) != 1 || len(split[1]) < 2 {
		t.Fatalf("split recovery gave %d and %d vectors", len(split[0]), len(split[1]))
	}
	t.Logf("oversized input recovered as %d vectors", len(split[1]))
}

// TestLiveRetrievalEval is a regression guard against a private labelled
// corpus kept outside the repository, not a relevance gate.
func TestLiveRetrievalEval(t *testing.T) {
	queries, root := os.Getenv(evalQueriesEnv), os.Getenv(evalRootEnv)
	if queries == "" || root == "" {
		t.Skipf("set %s and %s to run the live retrieval eval", evalQueriesEnv, evalRootEnv)
	}
	file := loadQueries(t, queries)
	s := newTestService(t, copyTree(t, root, file.ExcludedDirs...), NewOllama())
	start := time.Now()
	s.Warm(context.Background())
	s.mu.Lock()
	split := 0
	for _, vectors := range s.vectors {
		if len(vectors) > 1 {
			split++
		}
	}
	t.Logf("indexed %d documents, %d chunks, %d windows (%d split to fit the model) in %v; %d pending",
		len(s.entries), len(s.chunkAt), len(s.vectors), split, time.Since(start), len(s.pendingLocked()))
	s.mu.Unlock()
	var runs []evalRun
	for _, q := range file.Queries {
		run := runQuery(t, s, q)
		runs = append(runs, run)
		if len(q.Relevant) > 0 {
			t.Logf("%s %-11s fused recall %.2f over %d hits", q.ID, q.Kind, recallAt(run.returnedNames(), q.Relevant, 0), len(run.returned))
		} else if len(run.returned) > 0 {
			t.Logf("%s %-11s top hit %s", q.ID, q.Kind, run.returnedNames()[0])
		}
	}
	summary := summarise(runs)
	summary.log(t)
	if summary.fused < evalFloor {
		t.Fatalf("fused R@5 %.3f is below the %.2f regression floor", summary.fused, evalFloor)
	}
}
