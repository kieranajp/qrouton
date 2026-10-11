package workbench

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/vault"
)

// Discovery is the one process endpoint external launchers may use. Legacy is
// true when a live socket exists without a published endpoint.
type Discovery struct {
	Socket string
	Legacy bool
}

type processDescriptor struct {
	Version int    `json:"version"`
	Socket  string `json:"socket"`
	PID     int    `json:"pid"`
}

// Request is one call on the control socket.
type AssetFocusRequest struct {
	Index int `json:"index"`
}

type Request struct {
	BugReport        *BugReportRequest          `json:"bug_report,omitempty"`
	AssetFocus       *AssetFocusRequest         `json:"asset_focus,omitempty"`
	Op               string                     `json:"op"`
	ID               string                     `json:"id,omitempty"`
	Full             bool                       `json:"full,omitempty"`
	Root             string                     `json:"root,omitempty"`
	Activity         string                     `json:"activity,omitempty"`
	Generation       uint64                     `json:"generation,omitempty"`
	Options          *WindowOptions             `json:"options,omitempty"`
	Picker           *PickerRequest             `json:"picker,omitempty"`
	Ticket           *TicketRequest             `json:"ticket,omitempty"`
	RunnerGeneration *RunnerGenerationRequest   `json:"runner_generation,omitempty"`
	Lifecycle        *DelegatedLifecycleRequest `json:"lifecycle,omitempty"`
	AgentEvent       *AgentEventRequest         `json:"agent_event,omitempty"`
	VaultSearch      *vault.Query               `json:"vault_search,omitempty"`
	VaultRead        *vault.ReadRequest         `json:"vault_read,omitempty"`
}

// Response is the desktop process's single-line answer.
type Response struct {
	BugReport      *BugReport        `json:"bug_report,omitempty"`
	AssetSelection *AssetSelection   `json:"asset_selection,omitempty"`
	ID             string            `json:"id,omitempty"`
	Text           string            `json:"text,omitempty"`
	Exists         bool              `json:"exists,omitempty"`
	IDs            []string          `json:"ids,omitempty"`
	Viewport       *DocumentViewport `json:"viewport,omitempty"`
	VaultResult    *vault.Result     `json:"vault_result,omitempty"`
	VaultExcerpt   *vault.Excerpt    `json:"vault_excerpt,omitempty"`
	Outcome        string            `json:"outcome,omitempty"`
	Error          string            `json:"error,omitempty"`
}

// TicketRequest is the canonical ticket URL and the user-level request the
// external tool generated for it.
type TicketRequest struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt,omitempty"`
}

type RunnerGenerationRequest struct {
	Provider   string `json:"provider"`
	Generation uint64 `json:"generation"`
}

type DelegatedLifecycleRequest struct {
	Provider   string    `json:"provider"`
	Generation uint64    `json:"generation"`
	Kind       string    `json:"kind"`
	ID         string    `json:"id"`
	Type       string    `json:"type,omitempty"`
	TaskName   string    `json:"task_name,omitempty"`
	ParentID   string    `json:"parent_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// AgentEventRequest is one runner hook, reduced by the hook binary to what the
// agent tree needs. Model text arrives truncated and is shown, never read.
type AgentEventRequest struct {
	Provider   string    `json:"provider"`
	Generation uint64    `json:"generation"`
	Hook       string    `json:"hook"`
	Timestamp  time.Time `json:"timestamp"`
	// AgentID is the agent the hook fired in, and empty for the root.
	AgentID   string `json:"agent_id,omitempty"`
	AgentType string `json:"agent_type,omitempty"`

	ToolUseID string `json:"tool_use_id,omitempty"`
	Tool      string `json:"tool,omitempty"`
	Target    string `json:"target,omitempty"`

	Spawn   *AgentSpawn   `json:"spawn,omitempty"`
	ChildID string        `json:"child_id,omitempty"`
	Outcome *AgentOutcome `json:"outcome,omitempty"`

	Message          string `json:"message,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`
	// Interrupt is a tool failure the user caused by interrupting the turn.
	Interrupt bool `json:"interrupt,omitempty"`

	BackgroundTasks []string           `json:"background_tasks,omitempty"`
	Notifications   []TaskNotification `json:"notifications,omitempty"`
}

// AgentSpawn is the Agent tool's input: what the caller asked for.
type AgentSpawn struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Async       bool   `json:"async,omitempty"`
}

// AgentOutcome is how a spawned agent finished, as the harness reports it.
type AgentOutcome struct {
	Status     string `json:"status"`
	Tokens     int    `json:"tokens,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	ToolUses   int    `json:"tool_uses,omitempty"`
}

// TaskNotification is a background agent's result, delivered to its caller as
// a prompt.
type TaskNotification struct {
	TaskID    string       `json:"task_id"`
	ToolUseID string       `json:"tool_use_id,omitempty"`
	Result    string       `json:"result,omitempty"`
	Outcome   AgentOutcome `json:"outcome"`
}

// NewSocketPath reserves an address for a desktop process's control socket.
// macOS caps a unix socket path at 104 bytes, so it is keyed on a token under a
// per-uid directory rather than on the session — which also makes the handle
// valid before onboarding has chosen a session.
func NewSocketPath() (string, error) {
	token := make([]byte, socketTokenBytes)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	dir := socketDir()
	if err := os.MkdirAll(dir, socketDirMode); err != nil {
		return "", err
	}
	return filepath.Join(dir, hex.EncodeToString(token)+socketSuffix), nil
}

func socketDir() string {
	return filepath.Join(socketRoot, strconv.Itoa(os.Getuid()))
}

// Answered reports whether something is listening on socket.
func Answered(socket string) bool {
	conn, err := net.Dial(socketNetwork, socket)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Published reports whether socket is the live process endpoint named by the
// active descriptor.
func Published(socket string) bool { return published(socketDir(), socket) }

func published(dir, socket string) bool {
	descriptor, _, ok := readDescriptor(dir)
	return ok && descriptor.Socket == socket && validDescriptor(dir, descriptor) && Answered(socket)
}

// Discover returns the published process endpoint, or reports a live workbench
// from before endpoint publication was introduced.
func Discover() Discovery { return discover(socketDir()) }

func discover(dir string) Discovery {
	active := ""
	_ = withFileLock(dir, descriptorLockName, func() error {
		descriptor, _, ok := readDescriptor(dir)
		if !ok {
			return nil
		}
		if validDescriptor(dir, descriptor) && Answered(descriptor.Socket) {
			active = descriptor.Socket
			return nil
		}
		err := os.Remove(filepath.Join(dir, activeName))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	})
	if active != "" {
		return Discovery{Socket: active}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Discovery{}
	}
	up := false
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), socketSuffix) {
			continue
		}
		socket := filepath.Join(dir, entry.Name())
		if Answered(socket) {
			up = true
			continue
		}
		// An address is reserved without creating a file, so a socket file that
		// does not answer belonged to a process that is gone.
		_ = os.Remove(socket)
	}
	return Discovery{Legacy: up}
}

// Publish makes socket the only process endpoint external launchers may use.
func Publish(socket string) error {
	descriptor := processDescriptor{Version: descriptorVersion, Socket: socket, PID: os.Getpid()}
	return publish(socketDir(), descriptor)
}

func publish(dir string, descriptor processDescriptor) error {
	if !validDescriptor(dir, descriptor) {
		return ErrInvalidProcessDescriptor
	}
	return withFileLock(dir, descriptorLockName, func() error {
		body, err := json.Marshal(descriptor)
		if err != nil {
			return err
		}
		file, err := os.CreateTemp(dir, temporaryPattern)
		if err != nil {
			return err
		}
		tmp := file.Name()
		defer os.Remove(tmp)
		if err := file.Chmod(descriptorMode); err != nil {
			file.Close()
			return err
		}
		if _, err := file.Write(body); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		return os.Rename(tmp, filepath.Join(dir, activeName))
	})
}

// Unpublish removes this process's descriptor only while it still names socket.
func Unpublish(socket string) {
	unpublish(socketDir(), processDescriptor{Version: descriptorVersion, Socket: socket, PID: os.Getpid()})
}

func unpublish(dir string, expected processDescriptor) {
	_ = withFileLock(dir, descriptorLockName, func() error {
		descriptor, _, ok := readDescriptor(dir)
		if !ok || descriptor != expected {
			return nil
		}
		err := os.Remove(filepath.Join(dir, activeName))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	})
}

func readDescriptor(dir string) (processDescriptor, []byte, bool) {
	body, err := os.ReadFile(filepath.Join(dir, activeName))
	if err != nil {
		return processDescriptor{}, nil, false
	}
	var descriptor processDescriptor
	if err := json.Unmarshal(body, &descriptor); err != nil {
		return processDescriptor{}, body, true
	}
	return descriptor, body, true
}

func validDescriptor(dir string, descriptor processDescriptor) bool {
	return descriptor.Version == descriptorVersion && descriptor.PID > 0 &&
		filepath.IsAbs(descriptor.Socket) && filepath.Dir(filepath.Clean(descriptor.Socket)) == filepath.Clean(dir) &&
		strings.HasSuffix(descriptor.Socket, socketSuffix)
}

// WithLaunchLock serializes workbench discovery and launch across processes.
func WithLaunchLock(fn func() error) error {
	return withLaunchLock(socketDir(), fn)
}

func withLaunchLock(dir string, fn func() error) error {
	return withFileLock(dir, launchLockName, fn)
}

func withFileLock(dir, name string, fn func() error) error {
	if err := os.MkdirAll(dir, socketDirMode); err != nil {
		return err
	}
	return atomicfile.WithLock(filepath.Join(dir, name), descriptorMode, fn)
}

// ProcessLog is where a workbench process's stdio lands before it belongs to a
// session, keyed on the same process token as its socket. A session that has
// been chosen owns its own log inside it.
func ProcessLog(socket string) string {
	return strings.TrimSuffix(socket, socketSuffix) + logSuffix
}

// Attention carries what the runner's own hooks said about its state. No tool
// exposes it, so nothing reads model output to work it out.
func (h Handle) Attention(ctx context.Context, activity string, generation uint64) error {
	_, err := h.client().call(ctx, Request{Op: OpAttention, Activity: activity, Generation: generation})
	return err
}

func (h Handle) RunnerGeneration(ctx context.Context, provider string, generation uint64) error {
	_, err := h.client().call(ctx, Request{Op: OpRunnerGeneration,
		RunnerGeneration: &RunnerGenerationRequest{Provider: provider, Generation: generation}})
	return err
}

func (h Handle) AgentEvent(ctx context.Context, event AgentEventRequest) error {
	_, err := h.client().call(ctx, Request{Op: OpAgentEvent, AgentEvent: &event})
	return err
}

func (h Handle) DelegatedLifecycle(ctx context.Context, event DelegatedLifecycleRequest) error {
	_, err := h.client().call(ctx, Request{Op: OpDelegatedLifecycle, Lifecycle: &event})
	return err
}

// OpenTicket offers one canonical ticket and its generated prompt to the
// published process endpoint.
func OpenTicket(ctx context.Context, socket, url, prompt string) (string, error) {
	res, err := (&client{socket: socket}).call(ctx, Request{
		Op: OpOpenTicket, Ticket: &TicketRequest{URL: url, Prompt: prompt},
	})
	return res.Outcome, err
}

type client struct {
	socket string
}

func newClient(socket string) WindowHost { return &client{socket: socket} }

func (h Handle) client() *client { return &client{socket: h.Socket} }

func (c *client) Open(ctx context.Context, opts WindowOptions) (string, error) {
	res, err := c.call(ctx, Request{Op: OpOpen, Options: &opts})
	if err != nil {
		return "", err
	}
	if res.ID == "" {
		return "", ErrWindowIDUnavailable
	}
	return res.ID, nil
}

func (c *client) Close(ctx context.Context, id string) error {
	_, err := c.call(ctx, Request{Op: OpClose, ID: id})
	return err
}

func (c *client) Read(ctx context.Context, id string, full bool) (string, error) {
	res, err := c.call(ctx, Request{Op: OpRead, ID: id, Full: full})
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

func (c *client) Viewport(ctx context.Context, id string) (*DocumentViewport, error) {
	res, err := c.call(ctx, Request{Op: OpViewport, ID: id})
	return res.Viewport.Measured(), err
}

func (c *client) Exists(ctx context.Context, id string) (bool, error) {
	res, err := c.call(ctx, Request{Op: OpExists, ID: id})
	if err != nil {
		return false, err
	}
	return res.Exists, nil
}

func (c *client) List(ctx context.Context) ([]string, error) {
	res, err := c.call(ctx, Request{Op: OpList})
	if err != nil {
		return nil, err
	}
	return res.IDs, nil
}

func (c *client) Picker(ctx context.Context, req PickerRequest) error {
	_, err := c.call(ctx, Request{Op: OpPicker, Root: req.SessionRoot, Picker: &req})
	return err
}

// call sends one request and reads its answer. The connection is the framing.
func (c *client) call(ctx context.Context, req Request) (Response, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, socketNetwork, c.socket)
	if err != nil {
		return Response{}, fmt.Errorf("%w: %w", ErrWorkbenchUnreachable, err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(CallTimeout))
	}
	line, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return Response{}, fmt.Errorf("%w: %w", ErrWorkbenchUnreachable, err)
	}
	answer, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return Response{}, fmt.Errorf("%w: %w", ErrWorkbenchUnreachable, err)
	}
	var res Response
	if err := json.Unmarshal(answer, &res); err != nil {
		return Response{}, err
	}
	if res.Error != "" {
		switch req.Op {
		case OpQueueBugReport, OpBugReportStatus, OpCancelBugReport:
			return Response{}, fmt.Errorf("%w: %s", ErrWorkbenchRefused, res.Error)
		default:
			return Response{}, errors.New(res.Error)
		}
	}
	return res, nil
}

func (c *client) FocusAsset(ctx context.Context, id string, index int) (AssetSelection, error) {
	res, err := c.call(ctx, Request{Op: OpFocusAsset, ID: id, AssetFocus: &AssetFocusRequest{Index: index}})
	if err != nil {
		return AssetSelection{}, err
	}
	selection := res.AssetSelection
	if selection == nil || selection.CurrentIndex < 1 || selection.CurrentIndex > selection.Count || selection.Revision == 0 {
		return AssetSelection{}, ErrAssetSelectionUnavailable
	}
	return *selection, nil
}
