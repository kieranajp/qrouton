package vault

import (
	"context"
	"fmt"
	"math"
)

type Fingerprint struct {
	Model         string `json:"model"`
	Digest        string `json:"digest"`
	Tokenizer     string `json:"tokenizer"`
	Preprocessing string `json:"preprocessing"`
	Dimensions    int    `json:"dimensions"`
	MaxTokens     int    `json:"max_tokens"`
}

type Provider interface {
	Probe(context.Context) (Fingerprint, error)
	Embed(context.Context, Fingerprint, []string) ([][]float32, error)
}

type PullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

func NormalizeVector(vector []float32) error {
	if len(vector) != EmbeddingDimensions {
		return ErrInvalidVector
	}
	var squared float64
	for _, v := range vector {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return ErrInvalidVector
		}
		squared += float64(v) * float64(v)
	}
	if squared == 0 {
		return fmt.Errorf("%w: zero norm", ErrInvalidVector)
	}
	norm := math.Sqrt(squared)
	for i := range vector {
		vector[i] = float32(float64(vector[i]) / norm)
	}
	return nil
}
