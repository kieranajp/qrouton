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
)

// Request is one call on the control socket.
type Request struct {
	Op      string         `json:"op"`
	ID      string         `json:"id,omitempty"`
	Full    bool           `json:"full,omitempty"`
	Root    string         `json:"root,omitempty"`
	Options *WindowOptions `json:"options,omitempty"`
}

// Response is the desktop process's single-line answer.
type Response struct {
	ID     string   `json:"id,omitempty"`
	Text   string   `json:"text,omitempty"`
	Exists bool     `json:"exists,omitempty"`
	IDs    []string `json:"ids,omitempty"`
	Error  string   `json:"error,omitempty"`
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
	dir := filepath.Join(socketRoot, strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(dir, socketDirMode); err != nil {
		return "", err
	}
	return filepath.Join(dir, hex.EncodeToString(token)+socketSuffix), nil
}

// ProcessLog is where a workbench process's stdio lands before it belongs to a
// session, keyed on the same process token as its socket. A session that has
// been chosen owns its own log inside it.
func ProcessLog(socket string) string {
	return strings.TrimSuffix(socket, socketSuffix) + logSuffix
}

type client struct {
	socket string
}

func newClient(socket string) WindowHost { return &client{socket: socket} }

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

func (c *client) Adopt(ctx context.Context, sessionRoot string) error {
	_, err := c.call(ctx, Request{Op: OpAdopt, Root: sessionRoot})
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
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(callTimeout))
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
		return Response{}, errors.New(res.Error)
	}
	return res, nil
}
