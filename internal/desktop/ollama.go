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
// goroutine, so a stalled stream never holds the lock Status takes.
type Ollama struct {
	client    ollamaClient
	model     string
	emit      emitter
	installed func() bool

	mu     sync.Mutex
	parent context.Context
	status OllamaStatus
	pull   *ollamaPull
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
	parent, pulling := o.parent, o.pull != nil
	o.mu.Unlock()
	if !pulling {
		o.publish(nil, o.probe(parent), false)
	}
	return o.snapshot()
}

func (o *Ollama) Pull() {
	o.mu.Lock()
	if o.pull != nil {
		o.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(o.parent)
	run := &ollamaPull{cancel: cancel, percent: -1}
	o.pull = run
	o.mu.Unlock()
	o.publish(run, OllamaStatus{State: ollamaStatePulling, Model: o.model}, false)
	go o.run(ctx, run)
}

func (o *Ollama) CancelPull() {
	o.mu.Lock()
	run := o.pull
	o.mu.Unlock()
	if run == nil {
		return
	}
	run.cancel()
	o.publish(run, OllamaStatus{State: ollamaStateMissing, Model: o.model}, true)
}

func (o *Ollama) run(ctx context.Context, run *ollamaPull) {
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
		o.publish(run, OllamaStatus{State: ollamaStatePulling, Model: o.model, Detail: p.Status,
			Completed: run.completed, Total: run.total}, false)
	})
	next := OllamaStatus{State: ollamaStateFailed, Model: o.model}
	if err != nil {
		next.Reason = err.Error()
	} else {
		next = o.probe(ctx)
	}
	o.publish(run, next, true)
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

// publish applies next only while owner is still the pull in flight, nil
// meaning none, so a cancelled pull cannot overwrite what replaced it.
func (o *Ollama) publish(owner *ollamaPull, next OllamaStatus, done bool) {
	o.mu.Lock()
	if o.pull != owner {
		o.mu.Unlock()
		return
	}
	if done {
		o.pull = nil
	}
	changed := o.status != next
	o.status = next
	o.mu.Unlock()
	if changed {
		o.emit(ollamaEvent, next)
	}
}
