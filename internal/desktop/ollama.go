package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/kieranajp/qrouton/internal/vault"
)

// OllamaStatus is what the page draws semantic search from. Installed answers
// only for an unreachable Ollama.
type OllamaStatus struct {
	State     string `json:"state"`
	Model     string `json:"model"`
	Detail    string `json:"detail,omitempty"`
	Completed int64  `json:"completed,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Installed bool   `json:"installed,omitempty"`
}

type ollamaClient interface {
	Model(ctx context.Context) (vault.Model, error)
	Pull(ctx context.Context, onProgress func(vault.PullProgress)) error
}

// Ollama probes for the embedding model and pulls it. The pull runs on its own
// goroutine, so a stalled stream never holds the lock.
type Ollama struct {
	client    ollamaClient
	model     string
	emit      emitter
	installed func() bool

	mu         sync.Mutex
	parent     context.Context
	status     OllamaStatus
	pull       *ollamaPull
	generation uint64
}

type ollamaPull struct {
	cancel           context.CancelFunc
	detail           string
	percent          int64
	completed, total int64
}

func newOllama(client ollamaClient, model string, emit emitter, installed func() bool) *Ollama {
	return &Ollama{client: client, model: model, emit: emit, installed: installed,
		parent: context.Background(), status: OllamaStatus{Model: model}}
}

func ollamaInstalled() bool {
	if _, err := exec.LookPath(ollamaBinary); err == nil {
		return true
	}
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := os.Stat(ollamaMacApp)
	return err == nil
}

// bind makes ctx the parent of every later pull, so quitting ends one.
func (o *Ollama) bind(ctx context.Context) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.parent = ctx
}

func (o *Ollama) snapshot() OllamaStatus {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.status
}

// Check probes Ollama unless a pull is in flight, which reports for itself.
func (o *Ollama) Check() OllamaStatus {
	o.mu.Lock()
	parent, generation, pulling := o.parent, o.generation, o.pull != nil
	o.mu.Unlock()
	if !pulling {
		next := o.probe(parent)
		o.mu.Lock()
		if o.generation == generation {
			o.setLocked(next)
		}
		o.mu.Unlock()
	}
	return o.snapshot()
}

func (o *Ollama) Pull() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.pull != nil {
		return
	}
	ctx, cancel := context.WithCancel(o.parent)
	run := &ollamaPull{cancel: cancel, percent: -1}
	o.pull = run
	o.generation++
	o.setLocked(OllamaStatus{State: ollamaStatePulling, Model: o.model})
	go o.run(ctx, o.parent, run)
}

func (o *Ollama) CancelPull() {
	o.mu.Lock()
	run := o.pull
	if run != nil {
		o.finishLocked(OllamaStatus{State: ollamaStateMissing, Model: o.model})
	}
	o.mu.Unlock()
	if run != nil {
		run.cancel()
	}
}

func (o *Ollama) run(ctx, parent context.Context, run *ollamaPull) {
	defer run.cancel()
	err := o.client.Pull(ctx, func(p vault.PullProgress) {
		// Lines after the download carry no sizes, and the bar should not empty for them.
		if p.Total > 0 {
			run.completed, run.total = p.Completed, p.Total
		}
		percent := int64(0)
		if run.total > 0 {
			percent = run.completed * 100 / run.total
		}
		if p.Status == run.detail && percent == run.percent {
			return
		}
		run.detail, run.percent = p.Status, percent
		o.mu.Lock()
		if o.pull == run {
			o.setLocked(OllamaStatus{State: ollamaStatePulling, Model: o.model, Detail: p.Status,
				Completed: run.completed, Total: run.total})
		}
		o.mu.Unlock()
	})
	next := OllamaStatus{State: ollamaStateFailed, Model: o.model}
	if err != nil {
		next.Reason = err.Error()
	} else {
		next = o.probe(parent)
	}
	o.mu.Lock()
	if o.pull == run {
		o.finishLocked(next)
	}
	o.mu.Unlock()
}

func (o *Ollama) probe(parent context.Context) OllamaStatus {
	ctx, cancel := context.WithTimeout(parent, ollamaProbeTimeout)
	defer cancel()
	_, err := o.client.Model(ctx)
	switch {
	case err == nil:
		return OllamaStatus{State: ollamaStateReady, Model: o.model}
	case errors.Is(err, vault.ErrModelMissing):
		return OllamaStatus{State: ollamaStateMissing, Model: o.model}
	default:
		return OllamaStatus{State: ollamaStateUnreachable, Model: o.model, Installed: o.installed != nil && o.installed()}
	}
}

// setLocked emits under the lock, so events leave in the order the status changed.
func (o *Ollama) setLocked(next OllamaStatus) {
	if o.status == next {
		return
	}
	o.status = next
	o.emit(ollamaEvent, next)
}

// finishLocked ends the pull in flight. The new generation stops a Check that
// probed before it from overwriting the result.
func (o *Ollama) finishLocked(next OllamaStatus) {
	o.pull = nil
	o.generation++
	o.setLocked(next)
}
