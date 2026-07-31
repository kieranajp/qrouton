package dock

import (
	"strings"
	"testing"
)

func TestEmptyDockExplainsItsPurpose(t *testing.T) {
	frame := strings.Join(statusLines(), "\n")
	for _, want := range []string{"dock", "Agent panes minimise here", "Ask the agent to restore one"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("dock copy missing %q", want)
		}
	}
}
