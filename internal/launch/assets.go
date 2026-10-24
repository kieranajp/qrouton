package launch

import (
	"context"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
	"github.com/kieranajp/qrouton/prompts"
)

func sessionMode(dir string) string {
	m, err := session.Load(dir)
	if err != nil {
		return string(session.ModeRPI)
	}
	return string(m.EffectiveMode())
}

func sessionName(dir string) string {
	m, err := session.Load(dir)
	if err != nil {
		return ""
	}
	return m.DisplayName()
}

// sessionTicket is the provider a session's ticket belongs to, as a person
// writes it, and empty for a session assembled without one.
func sessionTicket(dir string) string {
	m, err := session.Load(dir)
	if err != nil {
		return ""
	}
	return ticket.ProviderLabel(m.TicketURL)
}

func primaryDiscovery(mode string) string {
	if mode == string(session.ModeAssistant) {
		return prompts.AssistantAsset
	}
	return prompts.OrchestratorAsset
}

func StampAssets(dir string) error {
	return StampAssetsWithLoader(context.Background(), dir, prompts.NewEmbeddedLoader())
}

func StampAssetsWithLoader(ctx context.Context, dir string, loader prompts.PromptLoader) error {
	return prompts.Stamp(ctx, dir, loader, primaryDiscovery(sessionMode(dir)))
}
