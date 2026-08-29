package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// DocumentWindow is how a session file reaches the user: a pane qrouton draws
// for the formats it can, and the editor for everything else. Both the agent's
// file tool and the window's own document chip come through here. span is the
// part of the file to draw the eye to; a pane marks it, an editor opens on its
// first line.
func DocumentWindow(root, name string, editor EditorCommand, span workbench.LineSpan) (workbench.WindowOptions, error) {
	path, err := ResolveSessionFile(root, name)
	if err != nil {
		return workbench.WindowOptions{}, err
	}
	rel := SessionRelative(root, path, name)
	if format, ok := workbench.FormatFor(path); ok {
		if opts, ok := documentPane(path, rel, format, span); ok {
			return opts, nil
		}
	}
	if len(editor.Argv) == 0 {
		return workbench.WindowOptions{}, ErrNoEditor
	}
	line, _, ok := span.Bounds()
	if !ok {
		line = firstLine
	}
	return workbench.WindowOptions{
		Kind:        workbench.KindTerminal,
		Label:       editorWindowLabel,
		Source:      rel,
		Cwd:         root,
		Command:     editor.Args(path, line),
		CloseOnExit: true,
	}, nil
}

// documentPane reports false when the file cannot be a pane after all, leaving
// the caller to fall back to the editor.
func documentPane(path, rel string, format workbench.DocumentFormat, span workbench.LineSpan) (workbench.WindowOptions, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > workbench.DocumentLimit {
		return workbench.WindowOptions{}, false
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return workbench.WindowOptions{}, false
	}
	first, last, ok := span.Bounds()
	if ok {
		span = workbench.LineSpan{Line: first, Through: last}
	} else {
		span = workbench.LineSpan{}
	}
	return workbench.WindowOptions{
		Kind:    workbench.KindDocument,
		Label:   documentLabel(string(text), rel),
		Source:  rel,
		Cwd:     filepath.Dir(path),
		Content: string(text),
		Format:  format,
		Span:    span,
	}, true
}

// documentLabel names the pane by what the document calls itself, since a tab
// has room for a title and not for a path.
func documentLabel(text, rel string) string {
	if title, ok := markdown.Title(text); ok {
		return fmt.Sprintf(documentLabelFormat, title)
	}
	return fmt.Sprintf(documentLabelFormat, filepath.Base(rel))
}

// SessionRelative names a file the way the session refers to it. The resolved
// path has been through EvalSymlinks and the root has not, so on a mac they
// share no prefix until both have.
func SessionRelative(root, path, name string) string {
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	if rel, ok := under(root, path); ok {
		return rel
	}
	// thoughts/ is a symlink out of the session, so every document resolves
	// outside root. Name it the way the session refers to it anyway, rather than
	// falling back to whatever absolute path the caller happened to pass.
	thoughts := filepath.Join(root, sessionpaths.ThoughtsDirName)
	if real, err := filepath.EvalSymlinks(thoughts); err == nil {
		if rel, ok := under(real, path); ok {
			return filepath.Join(sessionpaths.ThoughtsDirName, rel)
		}
	}
	return name
}

func under(dir, path string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}
