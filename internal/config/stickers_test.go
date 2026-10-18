package config

import (
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/theme"
)

// The page names a sticker's token from its colour, so a source scan sees only the prefix.
func TestEveryStickerColourHasAThemeToken(t *testing.T) {
	css := theme.CSS()
	for _, colour := range stickerColours {
		if !strings.Contains(css, "--sticker-"+colour+":") {
			t.Errorf("the palette declares no --sticker-%s", colour)
		}
	}
}
