package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
)

// WebGLLog appends one JSON line per WebGL renderer event the page reports.
type WebGLLog struct {
	path    string
	session func(terminal string) string
	mu      sync.Mutex
}

type webglEntry struct {
	Time     string `json:"time"`
	Event    string `json:"event"`
	Session  string `json:"session"`
	Terminal string `json:"terminal"`
	Pane     string `json:"pane"`
	Live     int    `json:"live"`
}

// webglLogPath is empty unless the workbench runs under QROUTON_INSPECT.
func webglLogPath(inspect bool) string {
	if !inspect {
		return ""
	}
	return config.WebGLLogPath()
}

func newWebGLLog(path string, session func(terminal string) string) *WebGLLog {
	return &WebGLLog{path: path, session: session}
}

// terminalSession names the session a conversation or terminal tab belongs to.
func terminalSession(sessions *Sessions, windows *Windows) func(string) string {
	return func(terminal string) string {
		if state, ok := sessions.byTerminal(terminal); ok {
			return state.slug()
		}
		if window, ok := windows.window(terminal); ok && window.session != nil {
			return window.session.slug()
		}
		return ""
	}
}

func (l *WebGLLog) Record(at, event, terminal, pane string, live int) {
	if l.path == "" {
		return
	}
	line, err := json.Marshal(webglEntry{
		Time: at, Event: event, Session: l.session(terminal), Terminal: terminal, Pane: pane, Live: live,
	})
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return
	}
	rotateLog(l.path)
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	_, _ = file.Write(append(line, '\n'))
}
