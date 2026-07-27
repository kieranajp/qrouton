package paneui

import (
	"strings"
	"testing"
)

// A watch pane repaints forever in a fixed number of rows, so a frame must
// never scroll: no newline after the last line, and autowrap disabled.
func TestFrameNeverScrollsThePane(t *testing.T) {
	frame := Frame([]string{"first", "second"})
	if strings.HasSuffix(strings.TrimSuffix(frame, eraseBelow), crlf) {
		t.Errorf("frame ends with a newline, which scrolls a full pane: %q", frame)
	}
	if !strings.HasPrefix(frame, autowrapOff) {
		t.Errorf("frame does not disable autowrap: %q", frame)
	}
	if strings.Count(frame, crlf) != 1 {
		t.Errorf("want one line break between two lines, got %d: %q", strings.Count(frame, crlf), frame)
	}
}
