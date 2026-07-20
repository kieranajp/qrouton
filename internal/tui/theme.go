package tui

// The onboarding TUI runs outside Zellij, so its palette is defined here to
// match the Zellij theme the session launches into (catppuccin-macchiato, set
// in internal/launch/assets/zellij-config.kdl). Keeping one source of truth
// means qrouton reads as one product from picker to pane. If the Zellij theme
// changes, change these hexes to match.
//
// Catppuccin Macchiato: https://github.com/catppuccin/catppuccin
import "github.com/charmbracelet/lipgloss"

const (
	ctpBase     = lipgloss.Color("#24273a")
	ctpMantle   = lipgloss.Color("#1e2030")
	ctpSurface0 = lipgloss.Color("#363a4f")
	ctpSurface1 = lipgloss.Color("#494d64")
	ctpSurface2 = lipgloss.Color("#5b6078")
	ctpOverlay0 = lipgloss.Color("#6e738d")
	ctpOverlay1 = lipgloss.Color("#8087a2")
	ctpSubtext0 = lipgloss.Color("#a5adcb")
	ctpText     = lipgloss.Color("#cad3f5")
	ctpBlue     = lipgloss.Color("#8aadf4")
	ctpSapphire = lipgloss.Color("#7dc4e4")
	ctpGreen    = lipgloss.Color("#a6da95")
	ctpYellow   = lipgloss.Color("#eed49f")
	ctpPeach    = lipgloss.Color("#f5a97f")
	ctpRed      = lipgloss.Color("#ed8796")
	ctpMauve    = lipgloss.Color("#c6a0f6")
)

var (
	accent = lipgloss.NewStyle().Foreground(ctpBlue).Bold(true)
	body   = lipgloss.NewStyle().Foreground(ctpText)
	dim    = lipgloss.NewStyle().Foreground(ctpOverlay1)
	good   = lipgloss.NewStyle().Foreground(ctpGreen)
	bad    = lipgloss.NewStyle().Foreground(ctpRed)

	// A crouton is a cube; Zellij is squares; the logo is a cube. Everything
	// qrouton draws is a box with square corners in that same design language.
	card = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(ctpSurface2)

	picked = card.Copy().BorderForeground(ctpBlue).Background(ctpSurface0)
)

// box frames one unit of the UI. Focused boxes take the accent border so the
// cursor reads as a lit-up cube in the grid; the rest sit quietly in surface2.
func box(focused bool) lipgloss.Style {
	b := lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(ctpSurface2)
	if focused {
		b = b.BorderForeground(ctpBlue)
	}
	return b
}

// chip renders one selectable token: filled/accented when picked, quiet when not.
func chip(label string, selected bool) string {
	if selected {
		return lipgloss.NewStyle().Foreground(ctpBase).Background(ctpBlue).Bold(true).Padding(0, 1).Render(label)
	}
	return lipgloss.NewStyle().Foreground(ctpSubtext0).Background(ctpSurface0).Padding(0, 1).Render(label)
}
