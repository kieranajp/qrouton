package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/ticket"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// controlHooks is what the socket may change about the running session that is
// not a window.
type controlHooks struct {
	picker     func(req workbench.PickerRequest) error
	attention  func(activity string, generation uint64)
	generation func(req workbench.RunnerGenerationRequest)
	lifecycle  func(req workbench.DelegatedLifecycleRequest)
	openTicket func(url, prompt string) (string, error)
	focus      func()
}

// control serves the workbench port over a unix socket: one request per
// connection, newline-delimited JSON.
type control struct {
	listener net.Listener
	socket   string
	windows  *Windows
	// owner is the session this listener speaks for. A handler bound to a
	// listener cannot address another session.
	owner *sessionState
	hooks controlHooks
}

func serveControl(socket string, windows *Windows, owner *sessionState, hooks controlHooks) (*control, error) {
	// A stale socket from a process that died without unlinking would otherwise
	// make every later run fail to bind.
	_ = os.Remove(socket)
	listener, err := net.Listen(socketNetwork, socket)
	if err != nil {
		return nil, err
	}
	c := &control{listener: listener, socket: socket, windows: windows, owner: owner, hooks: hooks}
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

// guard is a precondition on a request, checked in the order its operation
// lists them.
type guard func(*control, workbench.Request) error

type handler struct {
	guards []guard
	run    func(*control, workbench.Request) (workbench.Response, error)
}

// handlers is every operation the control socket serves. An operation is one
// entry: its guards, and a body that may assume they passed. A returned error
// becomes the response's Error; dispatch is the only place that conversion
// happens.
var handlers = map[string]handler{
	workbench.OpOpen: {
		guards: []guard{needsOptions, needsSession},
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			id, err := c.windows.openWindow(c.owner, *req.Options)
			return workbench.Response{ID: id}, err
		},
	},
	workbench.OpClose: {
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			return workbench.Response{ID: req.ID}, c.windows.Close(req.ID)
		},
	},
	workbench.OpRead: {
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			text, err := c.windows.readWindow(req.ID, req.Full)
			return workbench.Response{ID: req.ID, Text: text}, err
		},
	},
	workbench.OpViewport: {
		guards: []guard{needsSession},
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			viewport, err := c.windows.viewport(c.owner, req.ID)
			return workbench.Response{ID: req.ID, Viewport: viewport}, err
		},
	},
	workbench.OpExists: {
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			return workbench.Response{ID: req.ID, Exists: c.windows.exists(req.ID)}, nil
		},
	},
	workbench.OpList: {
		run: func(c *control, _ workbench.Request) (workbench.Response, error) {
			return workbench.Response{IDs: c.windows.list()}, nil
		},
	},
	workbench.OpPicker: {
		guards: []guard{needsPickerRequest},
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			if c.hooks.picker == nil {
				return workbench.Response{}, nil
			}
			return workbench.Response{}, c.hooks.picker(*req.Picker)
		},
	},
	workbench.OpAttention: {
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			if c.hooks.attention != nil {
				c.hooks.attention(req.Activity, req.Generation)
			}
			return workbench.Response{}, nil
		},
	},
	workbench.OpRunnerGeneration: {
		guards: []guard{needsSession, needsRunnerGeneration},
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			if c.hooks.generation != nil {
				c.hooks.generation(*req.RunnerGeneration)
			}
			return workbench.Response{}, nil
		},
	},
	workbench.OpDelegatedLifecycle: {
		guards: []guard{needsSession, needsLifecycle},
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			if c.hooks.lifecycle != nil {
				c.hooks.lifecycle(*req.Lifecycle)
			}
			return workbench.Response{}, nil
		},
	},
	workbench.OpOpenTicket: {
		guards: []guard{needsProcessIngress, needsTicket},
		run: func(c *control, req workbench.Request) (workbench.Response, error) {
			canonical, err := ticket.Canonical(req.Ticket.URL)
			if err != nil {
				return workbench.Response{}, err
			}
			outcome, err := c.hooks.openTicket(canonical, req.Ticket.Prompt)
			if c.hooks.focus != nil && (err == nil || errors.Is(err, assembly.ErrDraftConflict)) {
				c.hooks.focus()
			}
			return workbench.Response{Outcome: outcome}, err
		},
	},
}

func needsSession(c *control, _ workbench.Request) error {
	if c.owner == nil {
		return ErrNoSession
	}
	return nil
}

// needsProcessIngress admits only the published process endpoint: a session's
// listener has an owner, and the hook is installed on that one endpoint alone.
func needsProcessIngress(c *control, _ workbench.Request) error {
	if c.owner != nil || c.hooks.openTicket == nil {
		return ErrProcessIngressOnly
	}
	return nil
}

func needsOptions(_ *control, req workbench.Request) error {
	if req.Options == nil {
		return ErrNoWindowOptions
	}
	return nil
}

func needsPickerRequest(_ *control, req workbench.Request) error {
	if req.Root == "" || req.Picker == nil {
		return ErrNoSessionRoot
	}
	return nil
}

func needsRunnerGeneration(_ *control, req workbench.Request) error {
	if req.RunnerGeneration == nil || req.RunnerGeneration.Generation == 0 {
		return ErrNoRunnerGeneration
	}
	return nil
}

func needsLifecycle(_ *control, req workbench.Request) error {
	if req.Lifecycle == nil {
		return ErrNoDelegatedLifecycle
	}
	return nil
}

func needsTicket(_ *control, req workbench.Request) error {
	if req.Ticket == nil || req.Ticket.URL == "" {
		return ErrNoTicket
	}
	return nil
}

func (c *control) dispatch(req workbench.Request) workbench.Response {
	h, ok := handlers[req.Op]
	if !ok {
		return workbench.Response{Error: unknownOperation(req.Op).Error()}
	}
	for _, check := range h.guards {
		if err := check(c, req); err != nil {
			return workbench.Response{Error: err.Error()}
		}
	}
	res, err := h.run(c, req)
	if err != nil {
		return workbench.Response{Error: err.Error()}
	}
	return res
}

func (c *control) Close() error {
	err := c.listener.Close()
	_ = os.Remove(c.socket)
	return err
}
