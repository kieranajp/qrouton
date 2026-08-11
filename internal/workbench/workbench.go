// Package workbench is the port the agent's window tools are driven through,
// and the client that reaches the desktop process over its control socket.
// Nothing here links a webview.
package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// WindowKind separates a window running a command from one holding text a
// process has already finished producing.
type WindowKind string

const (
	KindTerminal WindowKind = "terminal"
	KindDocument WindowKind = "document"
)

// DocumentFormat is how a document window renders its text. The caller
// declares it; empty is plain text, and nothing infers it from the content.
type DocumentFormat string

const (
	FormatDiff     DocumentFormat = "diff"
	FormatMarkdown DocumentFormat = "markdown"
)

// panes name the pane a file opens in. A file with no pane opens in the editor.
var panes = map[string]DocumentFormat{
	".md":       FormatMarkdown,
	".markdown": FormatMarkdown,
}

// FormatFor names the pane a filename opens in.
func FormatFor(name string) (DocumentFormat, bool) {
	format, ok := panes[strings.ToLower(filepath.Ext(name))]
	return format, ok
}

// WindowOptions describes a window the agent opens. Command belongs to a
// terminal window and Content to a document one. CloseOnExit closes a terminal
// window whose process exits zero; a non-zero exit keeps it open regardless.
// TTL expires a document window. Attention marks a window that needs the
// user's eye without taking focus. Source names the session file the window
// shows, relative to the session root, so a second request for that file
// selects this window instead of opening another.
type WindowOptions struct {
	Kind        WindowKind     `json:"kind"`
	Label       string         `json:"label"`
	Source      string         `json:"source,omitempty"`
	Cwd         string         `json:"cwd,omitempty"`
	Command     []string       `json:"command,omitempty"`
	Content     string         `json:"content,omitempty"`
	Format      DocumentFormat `json:"format,omitempty"`
	Focus       bool           `json:"focus,omitempty"`
	Attention   bool           `json:"attention,omitempty"`
	CloseOnExit bool           `json:"close_on_exit,omitempty"`
	TTL         time.Duration  `json:"ttl,omitempty"`
}

// WindowHost opens and inspects the OS windows a session shows the user. The
// conversation keeps keyboard focus unless WindowOptions.Focus asks otherwise.
type WindowHost interface {
	Open(ctx context.Context, opts WindowOptions) (id string, err error)
	Close(ctx context.Context, id string) error
	// Read returns a terminal window's output with escape sequences stripped, or
	// a document window's content; full is the whole buffer, not the last screen.
	Read(ctx context.Context, id string, full bool) (string, error)
	// Exists reports whether a window is still open, so a caller's registry can
	// tell "you closed it" from a transport failure.
	Exists(ctx context.Context, id string) (bool, error)
	List(ctx context.Context) ([]string, error)
	// Adopt names the session, which onboarding chooses after the process starts.
	Adopt(ctx context.Context, sessionRoot string) error
}

// Handle identifies a running desktop process across the exec boundary.
type Handle struct {
	Socket      string `json:"socket"`
	SessionRoot string `json:"session_root"`
}

func (h Handle) Marshal() string {
	b, _ := json.Marshal(h)
	return string(b)
}

func ParseHandle(s string) (Handle, error) {
	var h Handle
	if err := json.Unmarshal([]byte(s), &h); err != nil {
		return Handle{}, fmt.Errorf("%s: %w", handleParseError, err)
	}
	if h.Socket == "" || h.SessionRoot == "" {
		return Handle{}, fmt.Errorf("%w: %q", ErrHandleIncomplete, s)
	}
	return h, nil
}

// WindowHost resolves the handle to a control-socket client.
func (h Handle) WindowHost() (WindowHost, error) {
	return newClient(h.Socket), nil
}

// WithEnv returns env with key set to value, replacing any existing entry.
func WithEnv(env []string, key, value string) []string {
	prefix := key + envKeyValueSep
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}
