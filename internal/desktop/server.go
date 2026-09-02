package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"

	"github.com/kieranajp/qrouton/internal/ticket"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// controlHooks is what the socket may change about the running session that is
// not a window.
type controlHooks struct {
	picker      func(req workbench.PickerRequest) error
	attention   func(activity string, generation uint64)
	generation  func(req workbench.RunnerGenerationRequest)
	lifecycle   func(req workbench.DelegatedLifecycleRequest)
	linearIssue func(ticket, prompt string) (string, error)
	focus       func()
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
	run    func(*control, workbench.Request) workbench.Response
}

// handlers is every operation the control socket serves. An operation is one
// entry: its guards, and a body that may assume they passed.
var handlers = map[string]handler{
	workbench.OpOpen: {
		guards: []guard{needsOptions, needsSession},
		run: func(c *control, req workbench.Request) workbench.Response {
			id, err := c.windows.openWindow(c.owner, *req.Options)
			if err != nil {
				return workbench.Response{Error: err.Error()}
			}
			return workbench.Response{ID: id}
		},
	},
	workbench.OpClose: {
		run: func(c *control, req workbench.Request) workbench.Response {
			if err := c.windows.Close(req.ID); err != nil {
				return workbench.Response{Error: err.Error()}
			}
			return workbench.Response{ID: req.ID}
		},
	},
	workbench.OpRead: {
		run: func(c *control, req workbench.Request) workbench.Response {
			text, err := c.windows.readWindow(req.ID, req.Full)
			if err != nil {
				return workbench.Response{Error: err.Error()}
			}
			return workbench.Response{ID: req.ID, Text: text}
		},
	},
	workbench.OpViewport: {
		guards: []guard{needsSession},
		run: func(c *control, req workbench.Request) workbench.Response {
			viewport, err := c.windows.viewport(c.owner, req.ID)
			if err != nil {
				return workbench.Response{Error: err.Error()}
			}
			return workbench.Response{ID: req.ID, Viewport: viewport}
		},
	},
	workbench.OpExists: {
		run: func(c *control, req workbench.Request) workbench.Response {
			return workbench.Response{ID: req.ID, Exists: c.windows.exists(req.ID)}
		},
	},
	workbench.OpList: {
		run: func(c *control, _ workbench.Request) workbench.Response {
			return workbench.Response{IDs: c.windows.list()}
		},
	},
	workbench.OpPicker: {
		guards: []guard{needsPickerRequest},
		run: func(c *control, req workbench.Request) workbench.Response {
			if c.hooks.picker != nil {
				if err := c.hooks.picker(*req.Picker); err != nil {
					return workbench.Response{Error: err.Error()}
				}
			}
			return workbench.Response{}
		},
	},
	workbench.OpAttention: {
		run: func(c *control, req workbench.Request) workbench.Response {
			if c.hooks.attention != nil {
				c.hooks.attention(req.Activity, req.Generation)
			}
			return workbench.Response{}
		},
	},
	workbench.OpRunnerGeneration: {
		guards: []guard{needsSession, needsRunnerGeneration},
		run: func(c *control, req workbench.Request) workbench.Response {
			if c.hooks.generation != nil {
				c.hooks.generation(*req.RunnerGeneration)
			}
			return workbench.Response{}
		},
	},
	workbench.OpDelegatedLifecycle: {
		guards: []guard{needsSession, needsLifecycle},
		run: func(c *control, req workbench.Request) workbench.Response {
			if c.hooks.lifecycle != nil {
				c.hooks.lifecycle(*req.Lifecycle)
			}
			return workbench.Response{}
		},
	},
	workbench.OpOpenLinearIssue: {
		guards: []guard{needsProcessIngress, needsLinearIssue},
		run: func(c *control, req workbench.Request) workbench.Response {
			canonical, err := ticket.Canonical(req.LinearIssue.Ticket)
			if err != nil {
				return workbench.Response{Error: err.Error()}
			}
			outcome, err := c.hooks.linearIssue(canonical, req.LinearIssue.Prompt)
			if c.hooks.focus != nil && (err == nil || errors.Is(err, ErrAssemblyDraftConflict)) {
				c.hooks.focus()
			}
			if err != nil {
				return workbench.Response{Error: err.Error()}
			}
			return workbench.Response{Outcome: outcome}
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
	if c.owner != nil || c.hooks.linearIssue == nil {
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

func needsLinearIssue(_ *control, req workbench.Request) error {
	if req.LinearIssue == nil || req.LinearIssue.Ticket == "" {
		return ErrNoLinearIssue
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
	return h.run(c, req)
}

func (c *control) Close() error {
	err := c.listener.Close()
	_ = os.Remove(c.socket)
	return err
}
