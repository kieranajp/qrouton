package vault

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Ollama struct{ client *http.Client }

func NewOllama() *Ollama {
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: 2 * time.Minute}
	return &Ollama{client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (o *Ollama) request(ctx context.Context, path string, body any) (*http.Response, error) {
	var reader io.Reader
	method := http.MethodGet
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, ollamaEndpoint+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: HTTP %d", ErrProviderResponse, resp.StatusCode)
	}
	return resp, nil
}
func (o *Ollama) fingerprint(ctx context.Context) (Fingerprint, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := o.request(ctx, "/api/tags", nil)
	if err != nil {
		return Fingerprint{}, err
	}
	defer resp.Body.Close()
	var tags struct {
		Models []struct {
			Name        string `json:"name"`
			Digest      string `json:"digest"`
			RemoteModel string `json:"remote_model"`
			RemoteHost  string `json:"remote_host"`
		} `json:"models"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&tags); err != nil {
		return Fingerprint{}, fmt.Errorf("%w: %w", ErrProviderResponse, err)
	}
	for _, m := range tags.Models {
		if m.Name == EmbeddingModel && m.Digest != "" {
			if m.RemoteModel != "" || m.RemoteHost != "" {
				return Fingerprint{}, ErrProviderResponse
			}
			return Fingerprint{Model: EmbeddingModel, Digest: m.Digest, Tokenizer: NewTokenizer().Checksum(), Preprocessing: PreprocessingVersion, Dimensions: EmbeddingDimensions, MaxTokens: EmbeddingMaxTokens}, nil
		}
	}
	return Fingerprint{}, ErrModelMissing
}
func (o *Ollama) Probe(ctx context.Context) (Fingerprint, error) {
	fp, err := o.fingerprint(ctx)
	if err != nil {
		return Fingerprint{}, err
	}
	_, err = o.Embed(ctx, fp, []string{embeddingProbe})
	return fp, err
}
func (o *Ollama) Embed(ctx context.Context, expected Fingerprint, inputs []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) > 64 {
		return nil, fmt.Errorf("%w: batch exceeds 64 inputs", ErrInputBudget)
	}
	for _, input := range inputs {
		if NewTokenizer().Count(input) > EmbeddingMaxTokens {
			return nil, ErrInputBudget
		}
	}
	current, err := o.fingerprint(ctx)
	if err != nil {
		return nil, err
	}
	if current != expected {
		return nil, ErrFingerprintChanged
	}
	resp, err := o.request(ctx, "/api/embed", struct {
		Model    string   `json:"model"`
		Input    []string `json:"input"`
		Truncate bool     `json:"truncate"`
	}{EmbeddingModel, inputs, false})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidVector, err)
	}
	if result.Error != "" || len(result.Embeddings) != len(inputs) {
		return nil, ErrInvalidVector
	}
	for _, v := range result.Embeddings {
		if err = NormalizeVector(v); err != nil {
			return nil, err
		}
	}
	current, err = o.fingerprint(ctx)
	if err != nil {
		return nil, err
	}
	if current != expected {
		return nil, ErrFingerprintChanged
	}
	return result.Embeddings, nil
}
func (o *Ollama) Pull(ctx context.Context, progress func(PullProgress)) error {
	resp, err := o.request(ctx, "/api/pull", struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}{EmbeddingModel, true})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	success := false
	for scanner.Scan() {
		var event struct {
			PullProgress
			Error string `json:"error"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("%w: %w", ErrModelPull, err)
		}
		if event.Error != "" {
			return ErrModelPull
		}
		if progress != nil {
			progress(event.PullProgress)
		}
		success = event.Status == "success"
	}
	if err = scanner.Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrModelPull, err)
	}
	if !success {
		return ErrModelPull
	}
	return nil
}

func (o *Ollama) Close() error { o.client.CloseIdleConnections(); return nil }
