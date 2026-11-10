package desktop

import (
	"context"

	"github.com/kieranajp/qrouton/internal/api"
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
)

// inProcess is the core served in this process, and the clients the adapters
// call it through once Hello has named this client.
type inProcess struct {
	*api.InProcess
	ollama qroutonv1.OllamaServiceClient
}

func startInProcess(kernel *core.Core, socket, version string) (*inProcess, error) {
	served, err := api.Start(kernel, socket, api.Build(version))
	if err != nil {
		return nil, err
	}
	hello, err := qroutonv1.NewWorkbenchServiceClient(served.Conn).Hello(context.Background(), &qroutonv1.HelloRequest{
		Protocol:   api.ProtocolVersion,
		Build:      api.Build(version),
		ClientName: applicationName,
	})
	if err != nil {
		served.Close()
		return nil, api.FromStatus(err)
	}
	conn := api.Identified(served.Conn, hello.GetClientId())
	return &inProcess{InProcess: served, ollama: qroutonv1.NewOllamaServiceClient(conn)}, nil
}
