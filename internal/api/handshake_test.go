package api

import (
	"context"
	"strings"
	"testing"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Catches a core that serves a client speaking another protocol, or one whose
// refusal leaves the user unable to tell which side is out of date.
func TestHelloRefusesAnotherProtocolAndNamesBoth(t *testing.T) {
	p := serve(t, newFakeOllama())
	_, err := qroutonv1.NewWorkbenchServiceClient(p.Conn).Hello(context.Background(), &qroutonv1.HelloRequest{Protocol: ProtocolVersion + 6})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("Hello with another protocol = %v, want FailedPrecondition", err)
	}
	for _, n := range []string{"7", "1"} {
		if !strings.Contains(status.Convert(err).Message(), n) {
			t.Fatalf("refusal %q does not name protocol %s", status.Convert(err).Message(), n)
		}
	}
}

// Catches a build mismatch refused like a protocol one, which would stop a
// rebuilt desktop from reaching the core it has to offer to restart.
func TestHelloAcceptsAnotherBuildAndReportsItsOwn(t *testing.T) {
	p := serve(t, newFakeOllama())
	resp, err := qroutonv1.NewWorkbenchServiceClient(p.Conn).Hello(context.Background(), &qroutonv1.HelloRequest{Protocol: ProtocolVersion, Build: "other"})
	if err != nil || resp.GetBuild() != "test-build" || resp.GetProtocol() != ProtocolVersion || resp.GetClientId() == "" {
		t.Fatalf("Hello = %+v, %v", resp, err)
	}
}

// Catches an interceptor that lets an unidentified client past, or one that
// accepts an id Hello never issued.
func TestACallBeforeHelloFailsPrecondition(t *testing.T) {
	p := serve(t, newFakeOllama())
	for name, conn := range map[string]grpc.ClientConnInterface{"no id": p.Conn, "unknown id": Identified(p.Conn, "forged")} {
		_, err := qroutonv1.NewOllamaServiceClient(conn).Check(context.Background(), &qroutonv1.CheckRequest{})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("%s: Check = %v, want FailedPrecondition", name, err)
		}
	}
	if _, err := qroutonv1.NewOllamaServiceClient(hello(t, p)).Check(context.Background(), &qroutonv1.CheckRequest{}); err != nil {
		t.Fatalf("Check after Hello = %v", err)
	}
}
