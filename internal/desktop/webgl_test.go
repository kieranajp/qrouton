package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
)

// Breaks if a report stops being appended as one JSON line carrying the
// session, the terminal, its pane, the live count and the created total.
func TestWebGLLogAppendsOneJSONLinePerReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "webgl.jsonl")
	log := newWebGLLog(path, func(terminal string) string { return "slug-of-" + terminal })

	log.Record("2026-10-27T09:00:00.000Z", "created", "term-1", "conversation", 1, 1)
	log.Record("2026-10-27T09:00:01.000Z", "lost", "term-1", "conversation", 1, 3)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("two reports wrote %d lines:\n%s", len(lines), data)
	}
	var entry webglEntry
	if err := json.Unmarshal([]byte(lines[1]), &entry); err != nil {
		t.Fatal(err)
	}
	want := webglEntry{
		Time: "2026-10-27T09:00:01.000Z", Event: "lost", Session: "slug-of-term-1",
		Terminal: "term-1", Pane: "conversation", Live: 1, Created: 3,
	}
	if entry != want {
		t.Fatalf("entry = %+v, want %+v", entry, want)
	}
}

// Breaks if the log is written without QROUTON_INSPECT, or not written with it.
func TestWebGLLogWritesOnlyWhenInspecting(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	session := func(string) string { return "" }

	newWebGLLog(webglLogPath(false), session).Record("2026-10-27T09:00:00.000Z", "created", "term-1", "tab", 1, 1)
	if _, err := os.Stat(config.WebGLLogPath()); !os.IsNotExist(err) {
		t.Fatalf("a workbench not inspecting wrote the WebGL log: %v", err)
	}

	newWebGLLog(webglLogPath(true), session).Record("2026-10-27T09:00:00.000Z", "created", "term-1", "tab", 1, 1)
	if _, err := os.Stat(config.WebGLLogPath()); err != nil {
		t.Fatalf("an inspecting workbench did not write the WebGL log: %v", err)
	}
}
