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

// Catches a goroutine from the server, its connection or the core that
// outlives Close, free to write after the workbench has gone.
func TestCloseLeavesNoGoroutineBehind(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(socket)
	root := t.TempDir()
	ollama := newFakeOllama()
	kernel := core.New(core.Options{Root: root, Config: &config.Config{Root: root}, Ollama: ollama}, &fakeClient{})
	p, err := Start(kernel, socket, "test-build")
	if err != nil {
		t.Fatal(err)
	}
	client := qroutonv1.NewOllamaServiceClient(hello(t, p))
	if _, err := client.Pull(context.Background(), &qroutonv1.PullRequest{}); err != nil {
		t.Fatal(err)
	}
	arrives(t, ollama.pulled, "the pull never started")
	p.Close()
	p.Close()
}

// Catches a Start whose core fails to start and is left running, which nothing
// could then shut down.
func TestAStartThatFailsShutsTheCoreDown(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	root := t.TempDir()
	kernel := core.New(core.Options{Root: root, Config: &config.Config{Root: root}}, &fakeClient{})
	file := root + "/file"
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(kernel, file+"/qrouton.sock", "test-build"); err == nil {
		t.Fatal("Start on a socket under a regular file succeeded")
	}
}
