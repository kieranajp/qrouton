package tui

// The onboarding TUI's palette is Catppuccin Macchiato, the same one the
// workbench window draws in, so qrouton reads as one product from picker to
// conversation.
//
// Only the shades the views actually draw with are declared; the rest of
// Catppuccin Macchiato is a lookup away if a new one is needed.
// https://github.com/catppuccin/catppuccin
import "github.com/charmbracelet/lipgloss"

const (
	ctpBase     = lipgloss.Color("#24273a")
	ctpSurface0 = lipgloss.Color("#363a4f")
	ctpSurface2 = lipgloss.Color("#5b6078")
	ctpOverlay1 = lipgloss.Color("#8087a2")
	ctpSubtext0 = lipgloss.Color("#a5adcb")
	ctpText     = lipgloss.Color("#cad3f5")
	ctpBlue     = lipgloss.Color("#8aadf4")
	ctpGreen    = lipgloss.Color("#a6da95")
	ctpRed      = lipgloss.Color("#ed8796")
)

var (
	accent = lipgloss.NewStyle().Foreground(ctpBlue).Bold(true)
	body   = lipgloss.NewStyle().Foreground(ctpText)
	dim    = lipgloss.NewStyle().Foreground(ctpOverlay1)
	good   = lipgloss.NewStyle().Foreground(ctpGreen)
	bad    = lipgloss.NewStyle().Foreground(ctpRed)

	// A crouton is a cube and the logo is a cube. Everything qrouton draws is
	// a box with square corners in that same design language.
	card = box(false)

	picked = box(true).Background(ctpSurface0)
)

// box frames one unit of the UI. Focused boxes take the accent border so the
// cursor reads as a lit-up cube in the grid; the rest sit quietly in surface2.
func box(focused bool) lipgloss.Style {
	border := ctpSurface2
	if focused {
		border = ctpBlue
	}
	return lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(border)
}

// chip renders one selectable token: filled/accented when picked, quiet when not.
func chip(label string, selected bool) string {
	if selected {
		return lipgloss.NewStyle().Foreground(ctpBase).Background(ctpBlue).Bold(true).Padding(0, 1).Render(label)
	}
	return lipgloss.NewStyle().Foreground(ctpSubtext0).Background(ctpSurface0).Padding(0, 1).Render(label)
}
