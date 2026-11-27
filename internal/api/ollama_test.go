package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/api/pagejson"
	"github.com/kieranajp/qrouton/internal/api/pagetest"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/vault"
)

func arrives(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal(what)
	}
}

// Catches an RPC that forwards to the wrong core method, or to none.
func TestEachOllamaRPCReachesItsOwnCoreMethod(t *testing.T) {
	ollama := newFakeOllama()
	client := qroutonv1.NewOllamaServiceClient(hello(t, serve(t, ollama)))
	ctx := context.Background()

	resp, err := client.Check(ctx, &qroutonv1.CheckRequest{})
	if err != nil || ollama.probes.Load() != 1 || resp.GetStatus().GetModel() != vault.OllamaModel || resp.GetStatus().GetState() != "missing" {
		t.Fatalf("Check = %+v, %v after %d probes", resp, err, ollama.probes.Load())
	}
	if _, err := client.Pull(ctx, &qroutonv1.PullRequest{}); err != nil {
		t.Fatal(err)
	}
	arrives(t, ollama.pulled, "Pull started no pull")
	if _, err := client.CancelPull(ctx, &qroutonv1.CancelPullRequest{}); err != nil {
		t.Fatal(err)
	}
	arrives(t, ollama.ended, "CancelPull left the pull running")
}

// Catches a mapper or schema edit that changes a key or a value the page reads
// from Ollama.Check, such as a 64-bit count protojson would quote.
func TestOllamaStatusPageJSONMatchesTheCoreType(t *testing.T) {
	var status core.OllamaStatus
	pagetest.FillGo(&status)
	want, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	pagetest.Same(t, pagejson.Marshal(ollamaStatus(status)), want)
}
