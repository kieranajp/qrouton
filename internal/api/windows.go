package api

import (
	"context"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
)

type windowService struct {
	qroutonv1.UnimplementedWindowServiceServer
	windows *core.Windows
}

func (w *windowService) ListTerminals(_ context.Context, req *qroutonv1.ListTerminalsRequest) (*qroutonv1.ListTerminalsResponse, error) {
	resp := &qroutonv1.ListTerminalsResponse{}
	for _, terminal := range w.windows.Terminals(req.GetSession()) {
		listed := &qroutonv1.SessionTerminal{TerminalId: terminal.ID, Kind: terminalKind(terminal.Kind)}
		if terminal.Size != nil {
			listed.Size = &qroutonv1.TerminalSize{Cols: int32(terminal.Size.Cols), Rows: int32(terminal.Size.Rows)}
		}
		resp.Terminals = append(resp.Terminals, listed)
	}
	return resp, nil
}
