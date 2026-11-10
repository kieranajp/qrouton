package desktop

import (
	"context"

	"github.com/kieranajp/qrouton/internal/api"
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
)

// inProcess is the core served in this process, the clients the adapters call
// it through once Hello has named this client, and the translator that carries
// its events to the pages.
type inProcess struct {
	*api.InProcess
	ollama qroutonv1.OllamaServiceClient

	stop       context.CancelFunc
	translated chan struct{}
}

func startInProcess(kernel *core.Core, hub *api.Hub, socket, version string, emit emitter) (*inProcess, error) {
	served, err := api.Start(kernel, hub, socket, api.Build(version))
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
	ctx, stop := context.WithCancel(context.Background())
	events := &translator{events: qroutonv1.NewEventServiceClient(conn), emit: emit}
	stream, err := events.subscribe(ctx)
	if err != nil {
		stop()
		served.Close()
		return nil, api.FromStatus(err)
	}
	in := &inProcess{InProcess: served, ollama: qroutonv1.NewOllamaServiceClient(conn), stop: stop, translated: make(chan struct{})}
	go func() {
		defer close(in.translated)
		events.run(ctx, stream)
	}()
	return in, nil
}

// Close ends the translator before the server and the core go, so it never
// resubscribes to a server that is stopping.
func (in *inProcess) Close() {
	in.stop()
	<-in.translated
	in.InProcess.Close()
}
