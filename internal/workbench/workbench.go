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

// DocumentLimit is where a document stops being one. Above it the editor gets
// the file rather than a window holding a copy of it, and a window already
// holding one stops following it.
const DocumentLimit = 1 << 20

// panes name the pane a file opens in. A file with no pane opens in the editor.
var panes = map[string]DocumentFormat{
	".md":       FormatMarkdown,
	".markdown": FormatMarkdown,
}

func FormatFor(name string) (DocumentFormat, bool) {
	format, ok := panes[strings.ToLower(filepath.Ext(name))]
	return format, ok
}

// WindowOptions describes a window the agent opens. Command belongs to a
// terminal window and Content to a document one. CloseOnExit closes a terminal
// window whose process exits zero; a non-zero exit keeps it open regardless.
// Attention marks a window that needs the user's eye without taking focus.
// Source names the session file the window shows, relative to the session root,
// so a second request for that file selects this window instead of opening
// another. Badge leads the tab in the artifact's own colour, ahead of Label.
type WindowOptions struct {
	Kind    WindowKind     `json:"kind"`
	Label   string         `json:"label"`
	Badge   string         `json:"badge,omitempty"`
	Source  string         `json:"source,omitempty"`
	Cwd     string         `json:"cwd,omitempty"`
	Command []string       `json:"command,omitempty"`
	Content string         `json:"content,omitempty"`
	Format  DocumentFormat `json:"format,omitempty"`
	Span    LineSpan       `json:"span,omitzero"`
	// Select changes the session's selected tab without requesting native focus.
	Select      bool `json:"select,omitempty"`
	Attention   bool `json:"attention,omitempty"`
	CloseOnExit bool `json:"close_on_exit,omitempty"`
}

// LineSpan is the part of a document the user should be looking at, in
// one-based source lines. A zero Line asks for the top of the file, and a
// Through below Line spans that line alone.
type LineSpan struct {
	Line    int `json:"line,omitempty"`
	Through int `json:"through,omitempty"`
}

// LineInterval is a source-mapped block visible in a rendered document.
type LineInterval struct {
	Line int `json:"line"`
	To   int `json:"to"`
}

// DocumentViewport is the measured source view of a rendered Markdown tab.
// Available distinguishes a measured empty viewport from one without geometry.
type DocumentViewport struct {
	Source    string         `json:"source"`
	Available bool           `json:"available"`
	Selected  bool           `json:"selected"`
	Intervals []LineInterval `json:"intervals"`
}

// Bounds reports the span as a closed line range, and false when it names no
// line at all.
func (s LineSpan) Bounds() (first, last int, ok bool) {
	if s.Line < 1 {
		return 0, 0, false
	}
	if s.Through < s.Line {
		return s.Line, s.Line, true
	}
	return s.Line, s.Through, true
}

// WindowHost opens and inspects the tabs a session shows the user. Agent opens
// leave keyboard focus with the conversation.
type WindowHost interface {
	Open(ctx context.Context, opts WindowOptions) (id string, err error)
	Close(ctx context.Context, id string) error
	// Read returns a terminal window's output with escape sequences stripped, or
	// a document window's content; full is the whole buffer, not the last screen.
	Read(ctx context.Context, id string, full bool) (string, error)
	// Viewport returns nil for windows without a source-mapped Markdown view.
	Viewport(ctx context.Context, id string) (*DocumentViewport, error)
	// Exists reports whether a window is still open, so a caller's registry can
	// tell "you closed it" from a transport failure.
	Exists(ctx context.Context, id string) (bool, error)
	List(ctx context.Context) ([]string, error)
	// Picker asks for the repository picker over a session. It queues on that
	// session rather than raising anything, so a background agent's escalation
	// does not take the screen from whatever the user is watching.
	Picker(ctx context.Context, req PickerRequest) error
}

// PickerRequest is an escalation waiting on a session. Name is what the agent
// proposes to call the work and Prefix the prefix a branch is cut with, both of
// which a session with repositories already has answers for. A caller's
// deadline keeps a stale request from being drawn; zero is a picker the user
// opened directly and remains live until they answer it.
type PickerRequest struct {
	SessionRoot string    `json:"session_root"`
	Name        string    `json:"name,omitempty"`
	Prefix      string    `json:"prefix,omitempty"`
	Deadline    time.Time `json:"deadline,omitempty"`
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

// WindowHost resolves the handle to a control-socket client. Nothing is dialled
// here, so there is nothing to fail: ParseHandle already refused a handle
// without a socket.
func (h Handle) WindowHost() WindowHost {
	return newClient(h.Socket)
}

// WithEnv returns env with key set to value, replacing any existing entry.
func WithEnv(env []string, key, value string) []string {
	out := WithoutEnv(env, key)
	return append(out, key+envKeyValueSep+value)
}

func WithoutEnv(env []string, key string) []string {
	prefix := key + envKeyValueSep
	out := make([]string, 0, len(env))
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}
