package diagram

import (
	"github.com/d2lang/d2/d2target"

	"github.com/kieranajp/qrouton/internal/theme"
)

// paletteRevision is part of every cache key: editing the overrides invalidates
// rendered SVG rather than leaving the old colours on screen.
const paletteRevision = "1"

// neutralDefaultTheme is d2 theme 0, the only stock theme an override can
// repaint completely: the others force caps-lock labels and container dots.
const neutralDefaultTheme = int64(0)

// overrides repaints d2 in Catppuccin Macchiato. The neutral ramp runs ink to
// ground, so N1 is the lightest of the seven here and N7 the darkest.
func overrides() *d2target.ThemeOverrides {
	return &d2target.ThemeOverrides{
		N1: ptr(theme.Text),
		N2: ptr(theme.Subtext1),
		N3: ptr(theme.Subtext0),
		N4: ptr(theme.Surface2),
		N5: ptr(theme.Surface1),
		N6: ptr(theme.Surface0),
		N7: ptr(theme.Base),

		B1: ptr(theme.Roles[theme.RoleAccentAction]),
		B2: ptr(theme.Overlay1),
		B3: ptr(theme.Overlay0),
		B4: ptr(theme.Surface2),
		B5: ptr(theme.Surface1),
		B6: ptr(theme.Surface0),

		AA2: ptr(theme.Roles[theme.RoleAccentLiteral]),
		AA4: ptr(theme.Surface2),
		AA5: ptr(theme.Surface1),

		AB4: ptr(theme.Surface1),
		AB5: ptr(theme.Surface0),
	}
}

func ptr[T any](value T) *T { return &value }
