package api

import (
	"context"
	"os"
	"testing"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/workbench"
	"go.uber.org/goleak"
)

// Catches a goroutine from the server, its connection, a live subscriber or the
// core that outlives Close, free to write after the workbench has gone.
func TestCloseLeavesNoGoroutineBehind(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(socket)
	root := t.TempDir()
	ollama := newFakeOllama()
	hub := NewHub()
	kernel := core.New(core.Options{Root: root, Config: &config.Config{Root: root}, Ollama: ollama}, &fakeClient{}, hub)
	p, err := Start(kernel, hub, socket, "test-build")
	if err != nil {
		t.Fatal(err)
	}
	client := qroutonv1.NewOllamaServiceClient(hello(t, p))
	if _, err := client.Pull(context.Background(), &qroutonv1.PullRequest{}); err != nil {
		t.Fatal(err)
	}
	arrives(t, ollama.pulled, "the pull never started")
	stream, err := qroutonv1.NewEventServiceClient(hello(t, p)).Subscribe(context.Background(), &qroutonv1.SubscribeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Header(); err != nil {
		t.Fatal(err)
	}
	p.Close()
	p.Close()
}
