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
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		// The page re-fetches after a refusal, so it must get the disk's text, not the poll's.
		if contentHash(string(current)) != hash {
			window.opts.Content = string(current)
			if info.Size() == int64(len(current)) {
				rendered.read.at, rendered.read.size = info.ModTime(), info.Size()
			}
			return fmt.Errorf("%w: %s", ErrDocumentChanged, window.opts.Source)
		}
		if err := atomicfile.Replace(path, []byte(text), info.Mode().Perm()); err != nil {
			return err
		}
		if info, err = os.Stat(path); err == nil && info.Size() == int64(len(text)) {
			rendered.read.at, rendered.read.size = info.ModTime(), info.Size()
		}
		window.opts.Content = text
		saved = contentHash(text)
		return nil
	})
	return saved, err
}

func specAnswersLine(source string) []byte {
	return []byte(fmt.Sprintf(specAnswersInFormat, source) + conversationSubmit)
}

// SendSpecAnswers types one fixed line into the spec's conversation, as if the
// user had. Only the window id comes from the page.
func (w *Windows) SendSpecAnswers(id string) error {
	var owner *sessionState
	var source string
	err := w.with(id, func(window *agentWindow) error {
		if _, _, err := specWindow(window); err != nil {
			return err
		}
		owner, source = window.session, window.opts.Source
		return nil
	})
	if err != nil {
		return err
	}
	return owner.write(specAnswersLine(source))
}
