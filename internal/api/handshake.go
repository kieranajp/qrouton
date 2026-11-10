package api

import (
	"context"
	"runtime/debug"
	"slices"
	"sync"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/randomid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ProtocolVersion is the one number both ends must agree on.
const ProtocolVersion uint32 = 1

// ClientHeader carries the id Hello issued on every later call.
const ClientHeader = "x-qrouton-client"

// Build names this binary: the packaged version, or the VCS revision of a dev build.
func Build(version string) string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return ""
}

type client struct {
	name         string
	build        string
	capabilities []string
}

type clients struct {
	mu   sync.Mutex
	byID map[string]client
}

func (c *clients) add(next client) string {
	id := randomid.Hex(16)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[id] = next
	return id
}

func clientID(ctx context.Context) string {
	ids := metadata.ValueFromIncomingContext(ctx, ClientHeader)
	if len(ids) != 1 {
		return ""
	}
	return ids[0]
}

func (c *clients) known(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.byID[clientID(ctx)]
	return ok
}

type workbenchService struct {
	qroutonv1.UnimplementedWorkbenchServiceServer
	build   string
	clients *clients
}

// Hello accepts a different build, which the client reports, but not a different protocol.
func (w *workbenchService) Hello(_ context.Context, req *qroutonv1.HelloRequest) (*qroutonv1.HelloResponse, error) {
	if req.GetProtocol() != ProtocolVersion {
		return nil, status.Error(codes.FailedPrecondition, protocolMismatch(req.GetProtocol(), ProtocolVersion).Error())
	}
	id := w.clients.add(client{name: req.GetClientName(), build: req.GetBuild(), capabilities: slices.Clone(req.GetCapabilities())})
	return &qroutonv1.HelloResponse{ClientId: id, Protocol: ProtocolVersion, Build: w.build}, nil
}

func (c *clients) unary(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if info.FullMethod != qroutonv1.WorkbenchService_Hello_FullMethodName && !c.known(ctx) {
		return nil, status.Error(codes.FailedPrecondition, ErrNoClient.Error())
	}
	resp, err := handler(ctx, req)
	return resp, toStatus(err)
}

func (c *clients) stream(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if !c.known(ss.Context()) {
		return status.Error(codes.FailedPrecondition, ErrNoClient.Error())
	}
	return toStatus(handler(srv, ss))
}

// Identified sends the client id Hello issued with every call made through conn.
func Identified(conn grpc.ClientConnInterface, id string) grpc.ClientConnInterface {
	return identified{conn: conn, id: id}
}

type identified struct {
	conn grpc.ClientConnInterface
	id   string
}

func (i identified) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return i.conn.Invoke(metadata.AppendToOutgoingContext(ctx, ClientHeader, i.id), method, args, reply, opts...)
}

func (i identified) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return i.conn.NewStream(metadata.AppendToOutgoingContext(ctx, ClientHeader, i.id), desc, method, opts...)
}
