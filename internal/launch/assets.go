package launch

import (
	"context"
	"encoding/json"
	"os"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/prompts"
)

// Runner modes mirror session.SessionMode; launch stays a leaf and reads the
// value straight from the manifest rather than importing the session package.
const (
	modeRPI       = "rpi"
	modeAssistant = "assistant"
)

// sessionMode reads the session's runner mode from its manifest, defaulting to
// RPI when the manifest is absent or unreadable.
func sessionMode(dir string) string {
	content, err := os.ReadFile(sessionpaths.Manifest(dir))
	if err != nil {
		return modeRPI
	}
	var manifest struct {
		Mode string `json:"mode"`
	}
	if json.Unmarshal(content, &manifest) == nil && manifest.Mode == modeAssistant {
		return modeAssistant
	}
	return modeRPI
}

// primaryDiscovery is the rendered prompt that CLAUDE.md/AGENTS.md link to for
// the given mode; the other prompt is still stamped, for escalation.
func primaryDiscovery(mode string) string {
	if mode == modeAssistant {
		return prompts.AssistantAsset
	}
	return prompts.OrchestratorAsset
}

// StampAssets refreshes qrouton's canonical, session-local assets and their
// runner adapters from the prompts embedded in the binary.
func StampAssets(dir string) error {
	return StampAssetsWithLoader(context.Background(), dir, prompts.NewEmbeddedLoader())
}

// StampAssetsWithLoader allows callers and tests to supply a prompt source.
func StampAssetsWithLoader(ctx context.Context, dir string, loader prompts.PromptLoader) error {
	return prompts.Stamp(ctx, dir, loader, primaryDiscovery(sessionMode(dir)))
}
