package core

import (
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// Ring keeps the last limit bytes written to it, and counts every byte ever
// written, so a reader can name a place in the output by offset.
type Ring struct {
	mu    sync.Mutex
	limit int
	buf   []byte
	total uint64
}

func NewRing(limit int) *Ring { return &Ring{limit: limit} }

// Write returns the offset b starts at.
func (r *Ring) Write(b []byte) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	start := r.total
	r.total += uint64(len(b))
	r.buf = append(r.buf, b...)
	if len(r.buf) > r.limit {
		r.buf = r.buf[len(r.buf)-r.limit:]
	}
	return start
}

func (r *Ring) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.buf...)
}

// Since returns what was written from offset on. An offset the ring no longer
// holds, or one not yet written, returns the whole ring with reset set.
func (r *Ring) Since(offset uint64) (data []byte, start uint64, reset bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	first := r.total - uint64(len(r.buf))
	if offset < first || offset > r.total {
		return append([]byte(nil), r.buf...), first, true
	}
	return append([]byte(nil), r.buf[offset-first:]...), offset, false
}

// Text renders the buffer as the agent reads it: escape sequences stripped, and
// each line reduced to what a carriage return last left on it.
func (r *Ring) Text(full bool) string { return terminalText(r.Bytes(), full) }

func terminalText(raw []byte, full bool) string {
	lines := strings.Split(strings.ReplaceAll(ansi.Strip(string(raw)), "\r\n", "\n"), "\n")
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

func WithTerminalEnv(env []string) []string {
	env = workbench.WithoutEnv(env, noColorEnvVar)
	return workbench.WithEnv(workbench.WithEnv(env, termEnvVar, termValue), colorTermEnvVar, colorTermValue)
}
