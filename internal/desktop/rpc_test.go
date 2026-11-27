package desktop

import (
	"context"
	"net"
	"sync"
	"testing"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// rpcRecorder is a server that records which RPC each adapter call reached,
// and answers each with err.
type rpcRecorder struct {
	qroutonv1.UnimplementedOllamaServiceServer

	mu    sync.Mutex
	calls []string
	err   error
}

func (r *rpcRecorder) record(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
	return r.err
}

func (r *rpcRecorder) reached(t *testing.T, want string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) != 1 || r.calls[0] != want {
		t.Fatalf("the adapter reached %v, want only %s", r.calls, want)
	}
}

func (r *rpcRecorder) Check(context.Context, *qroutonv1.CheckRequest) (*qroutonv1.CheckResponse, error) {
	if err := r.record("Ollama.Check"); err != nil {
		return nil, err
	}
	return &qroutonv1.CheckResponse{Status: &qroutonv1.OllamaStatus{Model: "recorded"}}, nil
}

func (r *rpcRecorder) Pull(context.Context, *qroutonv1.PullRequest) (*qroutonv1.PullResponse, error) {
	return &qroutonv1.PullResponse{}, r.record("Ollama.Pull")
}

func (r *rpcRecorder) CancelPull(context.Context, *qroutonv1.CancelPullRequest) (*qroutonv1.CancelPullResponse, error) {
	return &qroutonv1.CancelPullResponse{}, r.record("Ollama.CancelPull")
}

// recorded serves a fresh recorder on an in-memory listener for one test.
func recorded(t *testing.T) (*rpcRecorder, grpc.ClientConnInterface) {
	t.Helper()
	rec := &rpcRecorder{}
	listener := bufconn.Listen(1 << 16)
	server := grpc.NewServer()
	qroutonv1.RegisterOllamaServiceServer(server, rec)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///recorder",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		server.Stop()
	})
	return rec, conn
}
