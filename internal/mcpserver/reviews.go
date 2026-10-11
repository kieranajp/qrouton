package mcpserver

import (
	"context"
	"fmt"
	"os"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type startReviewInput struct {
	Plan         string            `json:"plan" jsonschema:"Binding plan Markdown path within this session"`
	Spec         string            `json:"spec,omitempty" jsonschema:"Optional binding spec Markdown path within this session"`
	Bases        map[string]string `json:"bases,omitempty" jsonschema:"Optional explicit base refs keyed by editing repository name"`
	Brief        string            `json:"brief" jsonschema:"Review purpose and authorized completed scope, at most 16 KiB"`
	Verification string            `json:"verification,omitempty" jsonschema:"Recorded verification evidence for the reviewed commits, at most 16 KiB"`
}

type startReviewOutput struct {
	Requested  bool             `json:"requested"`
	RunID      string           `json:"run_id,omitempty"`
	WindowName string           `json:"window_name,omitempty"`
	ReportPath string           `json:"report_path,omitempty"`
	Settings   *config.Reviewer `json:"settings,omitempty"`
	Error      string           `json:"error,omitempty"`
	Message    string           `json:"message"`
}

func addReviewTools(server *mcp.Server, windows *windowManager, handle workbench.Handle) {
	mcp.AddTool(server, &mcp.Tool{Name: toolReviewerSettings, Description: descReviewerSettings},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, launch.ReviewerState, error) {
			cfg, err := config.ReadFile()
			if err != nil {
				return nil, launch.ReviewerState{}, err
			}
			state := launch.ReviewerSettings(cfg)
			message := reviewerDisabledMessage
			if state.Enabled {
				message = reviewerAvailableMessage
				if !state.Available {
					message = state.Error
				}
			}
			return textResult(message), state, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: toolStartReview, Description: descStartReview},
		func(ctx context.Context, _ *mcp.CallToolRequest, input startReviewInput) (*mcp.CallToolResult, startReviewOutput, error) {
			out, err := windows.startReview(ctx, handle, input)
			if err != nil {
				return nil, out, err
			}
			return textResult(out.Message), out, nil
		})
}

func (m *windowManager) startReview(ctx context.Context, handle workbench.Handle, input startReviewInput) (startReviewOutput, error) {
	cfg, err := config.ReadFile()
	if err != nil {
		return startReviewOutput{}, err
	}
	state := launch.ReviewerSettings(cfg)
	out := startReviewOutput{Settings: state.Settings}
	if !state.Enabled {
		out.Message = reviewerDisabledMessage
		return out, nil
	}
	if !state.Available {
		out.Error, out.Message = state.Error, state.Error
		return out, nil
	}
	bin, err := os.Executable()
	if err != nil {
		return out, err
	}
	prepared, err := launch.PrepareReview(ctx, cfg, m.root, bin, handle, m.editor, launch.ReviewInput{
		Plan: input.Plan, Spec: input.Spec, Bases: input.Bases, Brief: input.Brief, Verification: input.Verification,
	})
	if err != nil {
		return out, err
	}
	out.RunID, out.WindowName, out.ReportPath = prepared.RunID, prepared.WindowName, prepared.ReportPath
	out.Settings = &prepared.Settings
	id, err := m.open(ctx, prepared.WindowName, prepared.Window)
	if err != nil {
		out.Error = err.Error()
		out.Message = fmt.Sprintf(reviewUncertainFormat, prepared.WindowName, prepared.ReportPath)
		return out, nil
	}
	if err := ctx.Err(); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), reviewCloseTimeout)
		defer cancel()
		closeErr := m.host.Close(closeCtx, id)
		m.mu.Lock()
		if entry := m.windows[prepared.WindowName]; entry.id == id && closeErr == nil {
			delete(m.windows, prepared.WindowName)
		}
		m.mu.Unlock()
		out.Error = err.Error()
		if closeErr != nil {
			out.Error += "; " + closeErr.Error()
		}
		out.Message = fmt.Sprintf(reviewInterruptedFormat, prepared.WindowName, prepared.ReportPath)
		return out, nil
	}
	out.Requested = true
	out.Message = fmt.Sprintf(reviewRequestedFormat, prepared.WindowName, prepared.ReportPath)
	return out, nil
}
