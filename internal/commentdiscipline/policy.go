package commentdiscipline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type Policy struct {
	SchemaVersion    int      `json:"schemaVersion"`
	MaxCommentRun    int      `json:"maxCommentRun"`
	NarrationPhrases []string `json:"narrationPhrases"`
	PathExtensions   []string `json:"pathExtensions"`
}

func LoadPolicy(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var policy Policy
	if err := decoder.Decode(&policy); err != nil {
		return Policy{}, fmt.Errorf("decode policy: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Policy{}, fmt.Errorf("decode policy: multiple JSON values")
		}
		return Policy{}, fmt.Errorf("decode policy: %w", err)
	}
	if policy.SchemaVersion != 1 {
		return Policy{}, fmt.Errorf("schemaVersion must be 1")
	}
	if policy.MaxCommentRun < 1 {
		return Policy{}, fmt.Errorf("maxCommentRun must be at least 1")
	}
	if err := validateList("narrationPhrases", policy.NarrationPhrases, false); err != nil {
		return Policy{}, err
	}
	if err := validateList("pathExtensions", policy.PathExtensions, true); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func validateList(name string, values []string, extension bool) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", name)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) || value != strings.ToLower(value) {
			return fmt.Errorf("%s entries must be non-empty, trimmed, and lowercase", name)
		}
		if extension && (strings.HasPrefix(value, ".") || strings.ContainsAny(value, `/\\`)) {
			return fmt.Errorf("pathExtensions entries must omit dots and path separators")
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%s contains duplicate %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
