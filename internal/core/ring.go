package core

import (
	"encoding/base64"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// Ring keeps the last limit bytes written to it.
type Ring struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func NewRing(limit int) *Ring { return &Ring{limit: limit} }

func (r *Ring) Write(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, b...)
	if len(r.buf) > r.limit {
		r.buf = r.buf[len(r.buf)-r.limit:]
	}
}

func (r *Ring) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.buf...)
}

// Text renders the buffer as the agent reads it: escape sequences stripped, and
// each line reduced to what a carriage return last left on it.
func (r *Ring) Text(full bool) string {
	r.mu.Lock()
	raw := string(r.buf)
	r.mu.Unlock()

	lines := strings.Split(strings.ReplaceAll(ansi.Strip(raw), "\r\n", "\n"), "\n")
	for i, line := range lines {
		if at := strings.LastIndex(line, "\r"); at >= 0 {
			lines[i] = line[at+1:]
		}
	}
	if !full && len(lines) > screenLines {
		lines = lines[len(lines)-screenLines:]
	}
	return strings.Join(lines, "\n")
}

// Chunk is one stretch of a terminal's output as the page receives it.
type Chunk struct {
	Encoded string `json:"encoded"`
	Replay  bool   `json:"replay,omitempty"`
	// First marks a running process's first replay, whose queries the page
	// answers as if live.
	First bool `json:"first,omitempty"`
}

// EncodeChunk is base64 because a raw PTY chunk is not valid UTF-8 at its
// boundary and JSON marshalling corrupts it.
func EncodeChunk(b []byte) Chunk {
	return Chunk{Encoded: base64.StdEncoding.EncodeToString(b)}
}

func DecodeInput(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

func WithTerminalEnv(env []string) []string {
	env = workbench.WithoutEnv(env, noColorEnvVar)
	return workbench.WithEnv(workbench.WithEnv(env, termEnvVar, termValue), colorTermEnvVar, colorTermValue)
}
