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
	switch input.Stage {
	case status.StageResearch, status.StagePlan, status.StageImplement:
	default:
		return "", fmt.Errorf("%w, got %q", ErrSummaryStage, input.Stage)
	}
	text := strings.TrimSpace(input.Summary)
	count := utf8.RuneCountInString(text)
	switch {
	case count == 0:
		return "", ErrSummaryRequired
	case count > summaryLimit:
		return "", summaryTooLong(count)
	}
	err := session.UpdateManifest(root, func(m session.Manifest) (session.Manifest, error) {
		m.Summary = session.Summary{Text: text, Stage: input.Stage, At: time.Now().UTC()}
		return m, nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(summarySetFormat, count, summaryLimit), nil
}
