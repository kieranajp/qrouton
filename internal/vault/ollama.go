package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Model struct {
	Name   string
	Digest string
}

type Embedder interface {
	Model(ctx context.Context) (Model, error)
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

type Ollama struct {
	URL    string
	Name   string
	Client *http.Client
}

func NewOllama() *Ollama {
	return &Ollama{URL: OllamaURL, Name: OllamaModel, Client: http.DefaultClient}
}

func (o *Ollama) Model(ctx context.Context) (Model, error) {
	var tags struct {
		Models []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	if err := o.call(ctx, http.MethodGet, ollamaTags, nil, &tags); err != nil {
		return Model{}, err
	}
	for _, m := range tags.Models {
		if m.Name == o.Name {
			return Model{Name: m.Name, Digest: m.Digest}, nil
		}
	}
	return Model{}, fmt.Errorf("%w: %s", ErrModelMissing, o.Name)
}

// Embed asks Ollama not to truncate, so an overflowing input fails loudly and
// the caller can split it rather than embed a silently shortened window.
func (o *Ollama) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	body := map[string]any{"model": o.Name, "input": inputs, "truncate": false}
	if err := o.call(ctx, http.MethodPost, ollamaEmbed, body, &out); err != nil {
		return nil, err
	}
	if len(out.Embeddings) != len(inputs) {
		return nil, ErrEmbeddingMissing
	}
	return out.Embeddings, nil
}

func (o *Ollama) call(ctx context.Context, method, route string, body, out any) error {
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, o.URL+route, &payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ollamaContentType)
	res, err := o.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&failure)
		if strings.Contains(failure.Error, ollamaOverflow) {
			return fmt.Errorf("%w: %s", ErrContextOverflow, failure.Error)
		}
		return fmt.Errorf("ollama %s: %s %s", route, res.Status, failure.Error)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// embedAll returns every vector for each input. A batch that overflows is
// retried one input at a time, since a failed batch costs about as much as a
// good one, and an input that still overflows is halved until its parts fit.
func embedAll(ctx context.Context, e Embedder, inputs []string) ([][][]float32, error) {
	vectors, err := e.Embed(ctx, inputs)
	if err == nil {
		out := make([][][]float32, len(vectors))
		for i, v := range vectors {
			out[i] = [][]float32{v}
		}
		return out, nil
	}
	if !errors.Is(err, ErrContextOverflow) {
		return nil, err
	}
	if len(inputs) == 1 {
		first, second := halves(inputs[0])
		if first == "" || second == "" {
			return nil, err
		}
		parts, err := embedAll(ctx, e, []string{first, second})
		if err != nil {
			return nil, err
		}
		return [][][]float32{append(parts[0], parts[1]...)}, nil
	}
	out := make([][][]float32, len(inputs))
	for i, input := range inputs {
		one, err := embedAll(ctx, e, []string{input})
		if err != nil {
			return nil, err
		}
		out[i] = one[0]
	}
	return out, nil
}
