// Package lineartools reads and writes coding-tools.json, the custom-script
// file Linear's desktop app runs an issue through. The document is Linear's
// format: an absent file is answered with a generated starter, and one the user
// already has is carried verbatim.
package lineartools

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/kieranajp/qrouton/internal/config"
)

// Tools is one coding-tools.json: where it lives, and the command a generated
// starter points Linear at.
type Tools struct {
	File    string
	Command []string
	Env     []string
}

func New(command, env []string) Tools {
	return Tools{
		File:    filepath.Clean(config.ExpandHome(ConfigPath)),
		Command: slices.Clone(command),
		Env:     slices.Clone(env),
	}
}

// Load answers the file's own text, or the starter to offer in its place when
// there is no file yet.
func (t Tools) Load() (string, error) {
	b, err := os.ReadFile(t.File)
	if err == nil {
		return string(b), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if len(t.Command) == 0 {
		return "", ErrNoCommand
	}
	args := append(slices.Clone(t.Command[1:]), issueTemplate)
	b, err = json.MarshalIndent(document{
		OpenIssue: command{Path: t.Command[0], Args: args, Env: slices.Clone(t.Env)},
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Validate answers the body to write for raw, refusing anything that is not a
// JSON object.
func Validate(raw string) ([]byte, error) {
	trimmed := bytes.TrimSpace([]byte(raw))
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		return nil, ErrNotAnObject
	}
	return append(trimmed, '\n'), nil
}

// Save writes body, leaving a file whose content already matches untouched so
// the user's own formatting survives a save that changed nothing.
func (t Tools) Save(body []byte) error {
	current, err := os.ReadFile(t.File)
	if err == nil && bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(body)) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(t.File), dirMode); err != nil {
		return err
	}
	return os.WriteFile(t.File, body, fileMode)
}

// document is coding-tools.json as the starter generates it. A file the user
// already has is never read through this.
type document struct {
	OpenIssue command `json:"openIssue"`
}

type command struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
	Env  []string `json:"env,omitempty"`
}
