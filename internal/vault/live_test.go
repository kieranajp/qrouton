package vault

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveProvider(t *testing.T) {
	if os.Getenv("QROUTON_VAULT_LIVE") != "1" {
		t.Skip("set QROUTON_VAULT_LIVE=1 with the exact model already installed")
	}
	t.Setenv("OLLAMA_HOST", "https://example.invalid")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	provider := NewOllama()
	fp, err := provider.Probe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := provider.Embed(ctx, fp, []string{"A locally indexed document.", "Heading\n\nA different document."})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || fp.Model != EmbeddingModel || fp.Digest == "" {
		t.Fatal("invalid live provider contract")
	}
	boundary := strings.Repeat("hello ", 254)
	if NewTokenizer().Count(boundary) != 256 || NewTokenizer().Count(boundary+"hello") != 257 {
		t.Fatal("incorrect token boundary fixture")
	}
	if _, err = provider.Embed(ctx, fp, []string{boundary}); err != nil {
		t.Fatalf("256-token input: %v", err)
	}
	if _, err = provider.Embed(ctx, fp, []string{boundary + "hello"}); !errors.Is(err, ErrInputBudget) {
		t.Fatalf("257-token input: %v", err)
	}
	response, err := provider.request(ctx, "/api/embed", map[string]any{"model": EmbeddingModel, "input": []string{boundary}, "truncate": false})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var usage struct {
		Tokens int `json:"prompt_eval_count"`
	}
	if err := json.NewDecoder(response.Body).Decode(&usage); err != nil || usage.Tokens != 256 {
		t.Fatalf("model tokenizer disagrees: tokens=%d error=%v", usage.Tokens, err)
	}
	t.Logf("model=%s digest=%s tokenizer=%s dimensions=%d", fp.Model, fp.Digest, fp.Tokenizer, fp.Dimensions)
}
