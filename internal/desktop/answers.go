package desktop

import (
	"fmt"
	"os"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/kinds"
	"github.com/kieranajp/qrouton/internal/status"
)

// answerableWindow is the only kind of window the page may write a file
// through: a spec the user answers or an audit the user triages. It returns
// the document's kind as well.
func answerableWindow(window *agentWindow) (*documentContent, string, string, error) {
	rendered, ok := window.document()
	path := window.sourcePath()
	if !ok || path == "" {
		return nil, "", "", ErrNotAnswerable
	}
	kind := status.DocumentKind(window.opts.Source)
	if kind != kinds.Spec.Label() && kind != kinds.Audit.Label() {
		return nil, "", "", ErrNotAnswerable
	}
	return rendered, path, kind, nil
}

// SaveAnswers replaces an answerable document's text when the file still
// hashes to what the page last read, and returns the new hash.
func (w *Windows) SaveAnswers(id, hash, text string) (string, error) {
	var saved string
	err := w.with(id, func(window *agentWindow) error {
		rendered, path, _, err := answerableWindow(window)
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

func fixedLine(format, source string) []byte {
	return []byte(pasteStart + fmt.Sprintf(format, source) + pasteEnd + conversationSubmit)
}

// SendSpecAnswers pastes one fixed line into the spec's conversation and
// submits it. Only the window id comes from the page.
func (w *Windows) SendSpecAnswers(id string) error {
	return w.sendFixedLine(id, kinds.Spec.Label(), specAnswersInFormat)
}

// SendAuditTriage pastes one fixed line into the audit's conversation and
// submits it. Only the window id comes from the page.
func (w *Windows) SendAuditTriage(id string) error {
	return w.sendFixedLine(id, kinds.Audit.Label(), auditTriageInFormat)
}

func (w *Windows) sendFixedLine(id, want, format string) error {
	var owner *sessionState
	var source string
	err := w.with(id, func(window *agentWindow) error {
		_, _, kind, err := answerableWindow(window)
		if err != nil {
			return err
		}
		if kind != want {
			return fmt.Errorf("%w: %s", ErrWrongSend, window.opts.Source)
		}
		owner, source = window.session, window.opts.Source
		return nil
	})
	if err != nil {
		return err
	}
	return owner.write(fixedLine(format, source))
}
