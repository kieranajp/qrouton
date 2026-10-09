package desktop

import (
	"fmt"
	"os"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/status"
)

// specWindow is the only kind of window the page may write a file through.
func specWindow(window *agentWindow) (*documentContent, string, error) {
	rendered, ok := window.document()
	path := window.sourcePath()
	if !ok || path == "" || status.DocumentKind(window.opts.Source) != status.KindSpec {
		return nil, "", ErrNotASpec
	}
	return rendered, path, nil
}

// SaveSpec replaces a spec's text when the file still hashes to what the page
// last read, and returns the new hash.
func (w *Windows) SaveSpec(id, hash, text string) (string, error) {
	var saved string
	err := w.with(id, func(window *agentWindow) error {
		rendered, path, err := specWindow(window)
		if err != nil {
			return err
		}
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if contentHash(string(current)) != hash {
			return fmt.Errorf("%w: %s", ErrDocumentChanged, window.opts.Source)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := atomicfile.Replace(path, []byte(text), info.Mode().Perm()); err != nil {
			return err
		}
		if info, err = os.Stat(path); err == nil {
			rendered.read.at, rendered.read.size = info.ModTime(), info.Size()
		}
		window.opts.Content = text
		saved = contentHash(text)
		return nil
	})
	return saved, err
}
