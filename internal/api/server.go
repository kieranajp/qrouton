// Package api serves the core as gRPC services, and is the only way a client
// reaches it.
package api

import (
	"context"
	"net"
	"sync"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const (
	maxMessage     = 32 << 20
	bufferSize     = 1 << 20
	inMemoryTarget = "passthrough:///qrouton"
)

// InProcess is the core served on an in-memory listener and a connection to it.
// Core stays reachable for the adapters that do not call through Conn yet.
type InProcess struct {
	Core *core.Core
	Conn *grpc.ClientConn

	hub    *Hub
	server *grpc.Server
	served chan struct{}
	once   sync.Once
}

// Start starts the core on the process socket and serves it, with hub as the
// core's Events. A core that fails to start is shut down before Start returns.
func Start(kernel *core.Core, hub *Hub, socket, build string) (*InProcess, error) {
	if err := kernel.Start(socket); err != nil {
		kernel.Shutdown()
		return nil, err
	}
	listener := bufconn.Listen(bufferSize)
	ids := &clients{byID: map[string]client{}}
	server := grpc.NewServer(
		grpc.WaitForHandlers(true),
		grpc.MaxRecvMsgSize(maxMessage),
		grpc.MaxSendMsgSize(maxMessage),
		grpc.UnaryInterceptor(ids.unary),
		grpc.StreamInterceptor(ids.stream),
	)
	qroutonv1.RegisterWorkbenchServiceServer(server, &workbenchService{build: build, clients: ids})
	qroutonv1.RegisterEventServiceServer(server, &eventService{hub: hub})
	if kernel.Ollama != nil {
		qroutonv1.RegisterOllamaServiceServer(server, &ollamaService{ollama: kernel.Ollama})
	}
	p := &InProcess{Core: kernel, hub: hub, server: server, served: make(chan struct{})}
	go func() {
		defer close(p.served)
		_ = server.Serve(listener)
	}()
	conn, err := grpc.NewClient(inMemoryTarget,
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessage), grpc.MaxCallSendMsgSize(maxMessage)),
	)
	if err != nil {
		p.Close()
		return nil, err
	}
	p.Conn = conn
	return p, nil
}

// Close stops serving before it stops the core, so no call lands on a core
// that has gone. Only the first call does anything.
func (p *InProcess) Close() {
	p.once.Do(func() {
		if p.Conn != nil {
			_ = p.Conn.Close()
		}
		p.server.Stop()
		<-p.served
		p.Core.Shutdown()
	})
}
