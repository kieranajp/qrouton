package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// DocumentWindow opens supported formats in a pane and everything else in the editor.
// A pane marks the full span; an editor opens at its first line.
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

func documentPane(path, rel string, format workbench.DocumentFormat, span workbench.LineSpan) (workbench.WindowOptions, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > workbench.DocumentLimit {
		return workbench.WindowOptions{}, false
	}
	read, err := os.ReadFile(path)
	if err != nil {
		return workbench.WindowOptions{}, false
	}
	text := string(read)
	first, last, ok := span.Bounds()
	if ok {
		span = workbench.LineSpan{Line: first, Through: last}
	} else {
		span = workbench.LineSpan{}
	}
	// A tab leads with the artifact's id, so its number is how the reader tells
	// one from another. A document without one, a loose note, simply has none.
	// The page draws it as a filled block, which is the bracketing the id used to
	// carry for itself.
	id := status.ArtifactID(rel)
	return workbench.WindowOptions{
		Kind:    workbench.KindDocument,
		Label:   documentLabel(text, rel, id),
		Badge:   id,
		Source:  rel,
		Cwd:     filepath.Dir(path),
		Content: text,
		Format:  format,
		Span:    span,
		Deck:    markdown.Marp(text),
	}, true
}

// documentLabel names the pane by what the document calls itself, since a tab
// has room for a title and not for a path. A tab already leading with the
// artifact's id neither repeats it nor needs the diamond.
func documentLabel(text, rel, id string) string {
	name, titled := markdown.Title(text)
	if !titled {
		name = strings.TrimLeft(filepath.Base(rel)[len(id):], "-_.")
	}
	if id != "" {
		return name
	}
	return fmt.Sprintf(documentLabelFormat, name)
}

// SessionRelative names a file the way the session refers to it. The resolved
// path has been through EvalSymlinks and the root has not, so on a mac they
// share no prefix until both have.
func SessionRelative(root, path, name string) string {
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	if rel, ok := sessionpaths.Within(root, path); ok {
		return rel
	}
	// thoughts/ is a symlink out of the session, so every document resolves
	// outside root. Name it the way the session refers to it anyway, rather than
	// falling back to whatever absolute path the caller happened to pass.
	thoughts := filepath.Join(root, sessionpaths.ThoughtsDirName)
	if real, err := filepath.EvalSymlinks(thoughts); err == nil {
		if rel, ok := sessionpaths.Within(real, path); ok {
			return filepath.Join(sessionpaths.ThoughtsDirName, rel)
		}
	}
	return name
}
