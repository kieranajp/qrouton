package desktop

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/vault"
)

// adapterOllama records which core calls reached the client behind it.
type adapterOllama struct {
	probes   atomic.Int32
	pulled   chan struct{}
	stopped  chan struct{}
	released chan struct{}
}

func newAdapterOllama() *adapterOllama {
	return &adapterOllama{pulled: make(chan struct{}, 1), stopped: make(chan struct{}, 1), released: make(chan struct{})}
}

func (a *adapterOllama) Model(context.Context) (vault.Model, error) {
	a.probes.Add(1)
	return vault.Model{}, vault.ErrModelMissing
}

func (a *adapterOllama) Pull(ctx context.Context, _ func(vault.PullProgress)) error {
	a.pulled <- struct{}{}
	select {
	case <-ctx.Done():
		a.stopped <- struct{}{}
	case <-a.released:
	}
	return ctx.Err()
}

func arrives(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal(what)
	}
}

// Catches an adapter method that forwards to the wrong core method, or to none.
// Each bound method gets a row that observes an effect only its own core method has.
func TestEachAdapterMethodReachesItsOwnCoreMethod(t *testing.T) {
	rows := []struct {
		method string
		run    func(t *testing.T)
	}{
		{"Ollama.Check", func(t *testing.T) {
			client := newAdapterOllama()
			adapter := &Ollama{ollama: core.NewOllama(client, func(string, any) {})}
			if got := adapter.Check(); client.probes.Load() != 1 || got.Model != vault.OllamaModel {
				t.Fatalf("Check answered %+v after %d probes", got, client.probes.Load())
			}
		}},
		{"Ollama.Pull", func(t *testing.T) {
			client := newAdapterOllama()
			defer close(client.released)
			adapter := &Ollama{ollama: core.NewOllama(client, func(string, any) {})}
			adapter.Pull()
			arrives(t, client.pulled, "Pull started no pull")
		}},
		{"Ollama.CancelPull", func(t *testing.T) {
			client := newAdapterOllama()
			defer close(client.released)
			ollama := core.NewOllama(client, func(string, any) {})
			adapter := &Ollama{ollama: ollama}
			ollama.Pull()
			arrives(t, client.pulled, "the pull never started")
			adapter.CancelPull()
			arrives(t, client.stopped, "CancelPull left the pull running")
		}},
	}
	for _, row := range rows {
		t.Run(row.method, row.run)
	}
}
