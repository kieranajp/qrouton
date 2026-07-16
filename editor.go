package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/shlex"
)

type editorCommand struct {
	Argv     []string `json:"argv"`
	Template bool     `json:"template"`
}

func resolveEditor(configured []string) (editorCommand, error) {
	if len(configured) > 0 {
		paths := 0
		for _, arg := range configured {
			paths += strings.Count(arg, "{path}")
		}
		if paths != 1 {
			return editorCommand{}, fmt.Errorf("editor must contain exactly one {path} placeholder")
		}
		if _, err := exec.LookPath(configured[0]); err != nil {
			return editorCommand{}, fmt.Errorf("editor %q is not installed", configured[0])
		}
		return editorCommand{Argv: append([]string(nil), configured...), Template: true}, nil
	}
	for _, key := range []string{"VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			argv, err := shlex.Split(value)
			if err != nil || len(argv) == 0 {
				return editorCommand{}, fmt.Errorf("%s is not a valid editor command: %w", key, err)
			}
			if _, err := exec.LookPath(argv[0]); err != nil {
				return editorCommand{}, fmt.Errorf("%s editor %q is not installed", key, argv[0])
			}
			return editorCommand{Argv: argv}, nil
		}
	}
	for _, name := range []string{"nvim", "vim", "vi"} {
		if path, err := exec.LookPath(name); err == nil {
			return editorCommand{Argv: []string{path, "+{line}", "{path}"}, Template: true}, nil
		}
	}
	return editorCommand{}, fmt.Errorf("no terminal editor found; set editor in %s, or set $VISUAL/$EDITOR", configPath())
}

func (e editorCommand) args(path string, line int) []string {
	if line < 1 {
		line = 1
	}
	if !e.Template {
		return append(append([]string(nil), e.Argv...), path)
	}
	out := make([]string, len(e.Argv))
	for i, arg := range e.Argv {
		out[i] = strings.ReplaceAll(strings.ReplaceAll(arg, "{path}", path), "{line}", strconv.Itoa(line))
	}
	return out
}

func (e editorCommand) marshal() string { b, _ := json.Marshal(e); return string(b) }

func resolveSessionFile(root, name string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	p := name
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	p, err = filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("file %q does not exist", name)
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file %q is outside the qrouton session", name)
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("file %q is not a regular file", name)
	}
	return real, nil
}
