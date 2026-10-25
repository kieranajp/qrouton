package vault

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestTokensMeetAcrossWordForms(t *testing.T) {
	for _, pair := range [][2]string{{"running", "run"}, {"planned", "plan"}, {"stopped", "stop"}, {"queries", "query"}} {
		a, b := tokens(pair[0]), tokens(pair[1])
		if !slices.Equal(a, b) {
			t.Errorf("tokens(%q) = %v, tokens(%q) = %v", pair[0], a, pair[1], b)
		}
	}
}

func TestExplainersCarryTheirKind(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "s", "shared", "explainers", "E1-cache.md"), "# How the cache works\n\nVectors are keyed by content hash.\n")
	writeFile(t, filepath.Join(root, "s", "shared", "plans", "P1-cache.md"), "# Cache plan\n\nVectors are keyed by content hash.\n")
	s := newTestService(t, root, &stubEmbedder{})
	res := search(t, s, Query{Text: "vectors keyed content hash", Kinds: []string{"explainer"}})
	if len(res.Hits) != 1 || res.Hits[0].ID != "s/shared/explainers/E1-cache" || res.Hits[0].Kind != "explainer" {
		t.Fatalf("explainer search = %+v", res.Hits)
	}
}
