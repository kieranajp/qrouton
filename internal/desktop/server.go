package desktop

import (
	"bufio"
	"encoding/json"
	"net"
	"os"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// controlHooks is what the socket may change about the running session that is
// not a window.
type controlHooks struct {
	adopt     func(sessionRoot string)
	attention func(activity string)
}

// control serves the workbench port over a unix socket: one request per
// connection, newline-delimited JSON.
type control struct {
	listener net.Listener
	socket   string
	windows  *Windows
	hooks    controlHooks
}

func serveControl(socket string, windows *Windows, hooks controlHooks) (*control, error) {
	// A stale socket from a process that died without unlinking would otherwise
	// make every later run fail to bind.
	_ = os.Remove(socket)
	listener, err := net.Listen(socketNetwork, socket)
	if err != nil {
		return nil, err
	}
	c := &control{listener: listener, socket: socket, windows: windows, hooks: hooks}
	go c.accept()
	return c, nil
}

func (c *control) accept() {
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			return
		}
		go c.handle(conn)
	}
}

func (c *control) handle(conn net.Conn) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return
	}
	var req workbench.Request
	if err := json.Unmarshal(line, &req); err != nil {
		c.answer(conn, workbench.Response{Error: err.Error()})
		return
	}
	c.answer(conn, c.dispatch(req))
}

func (c *control) answer(conn net.Conn, res workbench.Response) {
	body, err := json.Marshal(res)
	if err != nil {
		body, _ = json.Marshal(workbench.Response{Error: err.Error()})
	}
	_, _ = conn.Write(append(body, '\n'))
}

func (c *control) dispatch(req workbench.Request) workbench.Response {
	switch req.Op {
	case workbench.OpOpen:
		if req.Options == nil {
			return workbench.Response{Error: ErrNoWindowOptions.Error()}
		}
		id, err := c.windows.openWindow(*req.Options)
		if err != nil {
			return workbench.Response{Error: err.Error()}
		}
		return workbench.Response{ID: id}
	case workbench.OpClose:
		if err := c.windows.Close(req.ID); err != nil {
			return workbench.Response{Error: err.Error()}
		}
		return workbench.Response{ID: req.ID}
	case workbench.OpRead:
		text, err := c.windows.readWindow(req.ID, req.Full)
		if err != nil {
			return workbench.Response{Error: err.Error()}
		}
		return workbench.Response{ID: req.ID, Text: text}
	case workbench.OpExists:
		return workbench.Response{ID: req.ID, Exists: c.windows.exists(req.ID)}
	case workbench.OpList:
		return workbench.Response{IDs: c.windows.list()}
	case workbench.OpAdopt:
		if req.Root == "" {
			return workbench.Response{Error: ErrNoSessionRoot.Error()}
		}
		if c.hooks.adopt != nil {
			c.hooks.adopt(req.Root)
		}
		return workbench.Response{}
	case workbench.OpAttention:
		if c.hooks.attention != nil {
			c.hooks.attention(req.Activity)
		}
		return workbench.Response{}
	default:
		return workbench.Response{Error: unknownOperation(req.Op).Error()}
	}
}

func (c *control) Close() error {
	err := c.listener.Close()
	_ = os.Remove(c.socket)
	return err
}
