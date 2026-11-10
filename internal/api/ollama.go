package api

import (
	"context"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
)

type ollamaService struct {
	qroutonv1.UnimplementedOllamaServiceServer
	ollama *core.Ollama
}

func (o *ollamaService) Check(context.Context, *qroutonv1.CheckRequest) (*qroutonv1.CheckResponse, error) {
	return &qroutonv1.CheckResponse{Status: ollamaStatus(o.ollama.Check())}, nil
}

func (o *ollamaService) Pull(context.Context, *qroutonv1.PullRequest) (*qroutonv1.PullResponse, error) {
	o.ollama.Pull()
	return &qroutonv1.PullResponse{}, nil
}

func (o *ollamaService) CancelPull(context.Context, *qroutonv1.CancelPullRequest) (*qroutonv1.CancelPullResponse, error) {
	o.ollama.CancelPull()
	return &qroutonv1.CancelPullResponse{}, nil
}

func ollamaStatus(s core.OllamaStatus) *qroutonv1.OllamaStatus {
	return &qroutonv1.OllamaStatus{
		State:     s.State,
		Model:     s.Model,
		Detail:    s.Detail,
		Completed: int32(s.Completed),
		Total:     int32(s.Total),
		Reason:    s.Reason,
		Installed: s.Installed,
	}
}
