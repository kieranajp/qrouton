package api

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
	"google.golang.org/grpc"
)

type fakeClient struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeClient) Emit(event string, _ any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

func (f *fakeClient) Focused() bool                    { return false }
func (f *fakeClient) Raise()                           {}
func (f *fakeClient) SessionShown(string)              {}
func (f *fakeClient) Supports(core.Capability) bool    { return false }
func (f *fakeClient) Reveal(string) error              { return core.ErrNoRevealCommand }
func (f *fakeClient) OpenText(string) error            { return core.ErrNoOpenTextCommand }
func (f *fakeClient) CopyImage(string) error           { return core.ErrNoCopyImageCommand }
func (f *fakeClient) CopyFile(string) error            { return core.ErrNoCopyFileCommand }
func (f *fakeClient) ChooseDirectory() (string, error) { return "", core.ErrNoDirectoryPicker }
func (f *fakeClient) Chime(string)                     {}

// fakeOllama has no model and holds a pull open until its context ends.
type fakeOllama struct {
	probes atomic.Int32
	pulled chan struct{}
	ended  chan struct{}
}

func newFakeOllama() *fakeOllama {
	return &fakeOllama{pulled: make(chan struct{}, 1), ended: make(chan struct{}, 1)}
}

func (f *fakeOllama) Model(context.Context) (vault.Model, error) {
	f.probes.Add(1)
	return vault.Model{}, vault.ErrModelMissing
}

func (f *fakeOllama) Pull(ctx context.Context, _ func(vault.PullProgress)) error {
	f.pulled <- struct{}{}
	<-ctx.Done()
	f.ended <- struct{}{}
	return ctx.Err()
}

// serve starts a core over ollama on a socket of its own and serves it, closing
// both when the test ends.
func serve(t *testing.T, ollama core.OllamaClient) *InProcess {
	t.Helper()
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(socket) })
	root := t.TempDir()
	kernel := core.New(core.Options{Root: root, Config: &config.Config{Root: root}, Ollama: ollama}, &fakeClient{})
	p, err := Start(kernel, socket, "test-build")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// hello identifies a client on p and returns a connection carrying its id.
func hello(t *testing.T, p *InProcess) grpc.ClientConnInterface {
	t.Helper()
	resp, err := qroutonv1.NewWorkbenchServiceClient(p.Conn).Hello(context.Background(), &qroutonv1.HelloRequest{Protocol: ProtocolVersion, ClientName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return Identified(p.Conn, resp.GetClientId())
}
