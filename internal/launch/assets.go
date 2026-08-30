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

func primaryDiscovery(mode string) string {
	if mode == modeAssistant {
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
