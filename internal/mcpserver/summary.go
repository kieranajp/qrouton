package mcpserver

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
)

func setSummary(root string, input setSummaryInput) (string, error) {
	text := strings.TrimSpace(input.Summary)
	count := utf8.RuneCountInString(text)
	switch {
	case count == 0:
		return "", ErrSummaryRequired
	case count > summaryLimit:
		return "", summaryTooLong(count)
	}
	err := session.UpdateManifest(root, func(m session.Manifest) (session.Manifest, error) {
		m.Summary = session.Summary{Text: text, Stage: status.StageFor(root, m), At: time.Now().UTC()}
		return m, nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(summarySetFormat, count, summaryLimit), nil
}
