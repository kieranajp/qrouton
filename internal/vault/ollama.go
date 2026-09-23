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
	return &Ollama{URL: OllamaURL, Name: OllamaModel, Client: &http.Client{Timeout: requestTimeout}}
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
		return fmt.Errorf("%w: %s %s %s", ErrRejected, route, res.Status, failure.Error)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// embedAll returns every vector for each window. A batch Ollama rejects is
// retried one window at a time, and a window it rejects alone gets nil, so one
// bad window cannot sink its batch. Failing to reach Ollama fails the call.
func embedAll(ctx context.Context, e Embedder, windows []window) ([][][]float32, error) {
	inputs := make([]string, len(windows))
	for i, w := range windows {
		inputs[i] = w.input
	}
	vectors, err := e.Embed(ctx, inputs)
	out := make([][][]float32, len(windows))
	if err == nil {
		for i, v := range vectors {
			out[i] = [][]float32{v}
		}
		return out, nil
	}
	if !rejected(err) {
		return nil, err
	}
	for i, w := range windows {
		if out[i], err = embedWindow(ctx, e, w); err != nil && !rejected(err) {
			return nil, err
		}
	}
	return out, nil
}

func rejected(err error) bool {
	return errors.Is(err, ErrRejected) || errors.Is(err, ErrContextOverflow)
}

// embedWindow halves a window that overflows the model, keeping its
// breadcrumb on both halves, until each part fits.
func embedWindow(ctx context.Context, e Embedder, w window) ([][]float32, error) {
	vectors, err := e.Embed(ctx, []string{w.input})
	if err == nil {
		return vectors, nil
	}
	first, second := halves(w.text)
	if !errors.Is(err, ErrContextOverflow) || first == "" || second == "" {
		return nil, err
	}
	a, err := embedWindow(ctx, e, newWindow(w.crumb, first))
	if err != nil {
		return nil, err
	}
	b, err := embedWindow(ctx, e, newWindow(w.crumb, second))
	if err != nil {
		return nil, err
	}
	return append(a, b...), nil
}
