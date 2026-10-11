// Package lineartools installs qrouton into coding-tools.json, the file Linear's
// desktop app reads custom scripts from. Install owns one key, openIssue, and
// leaves every other key in the file as it found it.
package lineartools

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/config"
)

// Tools is one coding-tools.json: where it lives, and the command Install
// points Linear at.
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

// Snippet is the document Install writes into a file that has nothing else.
func (t Tools) Snippet() (string, error) {
	entry, err := t.entry()
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(map[string]json.RawMessage{openIssueKey: entry}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Replaces reports whether the file already holds an openIssue that Install
// would overwrite with a different one.
func (t Tools) Replaces() (bool, error) {
	doc, err := t.read()
	if err != nil {
		return false, err
	}
	current, ok := doc[openIssueKey]
	if !ok {
		return false, nil
	}
	entry, err := t.entry()
	if err != nil {
		return false, err
	}
	return !sameJSON(current, entry), nil
}

// Install sets openIssue, creating the file and its directory when absent.
func (t Tools) Install() error {
	entry, err := t.entry()
	if err != nil {
		return err
	}
	doc, err := t.read()
	if err != nil {
		return err
	}
	doc[openIssueKey] = entry
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(t.File), dirMode); err != nil {
		return err
	}
	return atomicfile.Replace(t.File, append(b, '\n'), fileMode)
}

func (t Tools) entry() (json.RawMessage, error) {
	if len(t.Command) == 0 {
		return nil, ErrNoCommand
	}
	args := append(slices.Clone(t.Command[1:]), issueTemplate)
	return json.Marshal(command{Path: t.Command[0], Args: args, Env: slices.Clone(t.Env)})
}

// read answers the file's top-level keys, or none when there is no file.
func (t Tools) read() (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(t.File)
	if errors.Is(err, fs.ErrNotExist) || err == nil && len(bytes.TrimSpace(b)) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNotAnObject
	}
	return doc, nil
}

func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

type command struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
	Env  []string `json:"env,omitempty"`
}
