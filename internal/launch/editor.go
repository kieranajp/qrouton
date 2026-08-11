package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/shlex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type EditorCommand struct {
	Argv     []string `json:"argv"`
	Template bool     `json:"template"`
}

func ResolveEditor(configured []string) (EditorCommand, error) {
	if len(configured) > 0 {
		paths := 0
		for _, arg := range configured {
			paths += strings.Count(arg, pathPlaceholder)
		}
		if paths != 1 {
			return EditorCommand{}, ErrEditorPlaceholder
		}
		if _, err := exec.LookPath(configured[0]); err != nil {
			return EditorCommand{}, fmt.Errorf("editor %q is not installed", configured[0])
		}
		return EditorCommand{Argv: append([]string(nil), configured...), Template: true}, nil
	}
	for _, key := range editorEnvVars {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			argv, err := shlex.Split(value)
			if err != nil || len(argv) == 0 {
				return EditorCommand{}, fmt.Errorf("%s is not a valid editor command: %w", key, err)
			}
			if _, err := exec.LookPath(argv[0]); err != nil {
				return EditorCommand{}, fmt.Errorf("%s editor %q is not installed", key, argv[0])
			}
			return EditorCommand{Argv: argv}, nil
		}
	}
	for _, name := range fallbackEditors {
		if path, err := exec.LookPath(name); err == nil {
			return EditorCommand{Argv: []string{path, linePlaceholderArg, pathPlaceholder}, Template: true}, nil
		}
	}
	return EditorCommand{}, fmt.Errorf("%w; set editor in %s, or set $%s/$%s",
		ErrNoEditor, config.Path(), editorEnvVars[0], editorEnvVars[1])
}

func (e EditorCommand) Args(path string, line int) []string {
	if line < firstLine {
		line = firstLine
	}
	if !e.Template {
		return append(append([]string(nil), e.Argv...), path)
	}
	out := make([]string, len(e.Argv))
	for i, arg := range e.Argv {
		out[i] = strings.ReplaceAll(strings.ReplaceAll(arg, pathPlaceholder, path), linePlaceholder, strconv.Itoa(line))
	}
	return out
}

func (e EditorCommand) Marshal() string { b, _ := json.Marshal(e); return string(b) }

func ResolveSessionFile(root, name string) (string, error) {
	real, err := resolveWithinSession(root, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %q", ErrNotRegularFile, name)
	}
	return real, nil
}

// ResolveSessionDir resolves name to an existing directory confined to the session
// root, applying the same symlink-escape guard as ResolveSessionFile. It backs the
// cwd argument of the window-opening MCP tools.
func ResolveSessionDir(root, name string) (string, error) {
	real, err := resolveWithinSession(root, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: %q", ErrNotDirectory, name)
	}
	return real, nil
}

// resolveWithinSession returns the real, absolute path for name (relative paths
// are taken against root) only if it exists and resolves inside the session:
// under root, or under the artifact home its thoughts symlink points at. It does
// not constrain the kind of file found.
func resolveWithinSession(root, name string) (string, error) {
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
		return "", fmt.Errorf("%w: %q", ErrOutsideSessionMissing, name)
	}
	if within(root, real) {
		return real, nil
	}
	// session.Create parks documents outside the session dir so Delete keeps them.
	if home, err := filepath.EvalSymlinks(filepath.Join(root, sessionpaths.ThoughtsDirName)); err == nil && within(home, real) {
		return real, nil
	}
	return "", fmt.Errorf("%w: %q", ErrOutsideSession, name)
}

// within reports whether real is base or sits underneath it; both absolute and
// already symlink-resolved.
func within(base, real string) bool {
	rel, err := filepath.Rel(base, real)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
