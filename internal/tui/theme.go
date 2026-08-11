package tui

// The onboarding TUI draws in the same palette as the workbench window, so
// qrouton reads as one product from picker to conversation. internal/theme owns
// the shades and what each accent is for; only the lipgloss styles live here.
import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kieranajp/qrouton/internal/theme"
)

func shade(hex string) lipgloss.Color { return lipgloss.Color(hex) }

var (
	backdrop = shade(theme.Base)
	raised   = shade(theme.Surface0)
	rule     = shade(theme.Surface2)
	quiet    = shade(theme.Overlay1)
	label    = shade(theme.Subtext0)
	prose    = shade(theme.Text)

	action  = shade(theme.Roles[theme.RoleAccentAction])
	success = shade(theme.Roles[theme.RoleStateSuccess])
	failure = shade(theme.Roles[theme.RoleStateFailed])
)

var (
	accent = lipgloss.NewStyle().Foreground(action).Bold(true)
	body   = lipgloss.NewStyle().Foreground(prose)
	dim    = lipgloss.NewStyle().Foreground(quiet)
	good   = lipgloss.NewStyle().Foreground(success)
	bad    = lipgloss.NewStyle().Foreground(failure)

	// A crouton is a cube and the logo is a cube. Everything qrouton draws is
	// a box with square corners in that same design language.
	card = box(false)

	picked = box(true).Background(raised)
)

// box frames one unit of the UI. Focused boxes take the accent border so the
// cursor reads as a lit-up cube in the grid; the rest sit quietly in surface2.
func box(focused bool) lipgloss.Style {
	border := rule
	if focused {
		border = action
	}
	return lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(border)
}

// chip renders one selectable token: filled/accented when picked, quiet when not.
func chip(text string, selected bool) string {
	if selected {
		return lipgloss.NewStyle().Foreground(backdrop).Background(action).Bold(true).Padding(0, 1).Render(text)
	}
	return lipgloss.NewStyle().Foreground(label).Background(raised).Padding(0, 1).Render(text)
}
