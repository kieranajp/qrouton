// Package theme is qrouton's palette: the Catppuccin Macchiato ramp, the one
// job each accent carries, and the custom properties the workbench serves.
// https://github.com/catppuccin/catppuccin
package theme

import "strings"

// The neutral ramp darkest first, then the hues. Catppuccin Macchiato verbatim.
const (
	Crust     = "#181926"
	Mantle    = "#1e2030"
	Base      = "#24273a"
	Surface0  = "#363a4f"
	Surface1  = "#494d64"
	Surface2  = "#5b6078"
	Overlay0  = "#6e738d"
	Overlay1  = "#8087a2"
	Subtext0  = "#a5adcb"
	Subtext1  = "#b8c0e0"
	Text      = "#cad3f5"
	Rosewater = "#f4dbd6"
	Flamingo  = "#f0c6c6"
	Pink      = "#f5bde6"

	Blue     = "#8aadf4"
	Lavender = "#b7bdf8"
	Sapphire = "#7dc4e4"
	Sky      = "#91d7e3"
	Teal     = "#8bd5ca"
	Green    = "#a6da95"
	Yellow   = "#eed49f"
	Peach    = "#f5a97f"
	Red      = "#ed8796"
	Maroon   = "#ee99a0"
	Mauve    = "#c6a0f6"
)

// Roles give each accent its one job: blue acts, butter names, cyan quotes the
// machine, the rest report. Reusing one for a second job is how a field label
// and a warning end up the same colour and neither can be trusted.
var Roles = map[string]string{
	RoleAccentAction:      Blue,
	RoleAccentLabel:       Yellow,
	RoleAccentLiteral:     Sapphire,
	RoleActionDestructive: Red,
	RoleStateSelected:     Blue,
	RoleStateSuccess:      Green,
	RoleStateRunning:      Teal,
	RoleStateFailed:       Red,
	RoleStateWaiting:      Peach,
	RoleStateGuided:       Mauve,
	RoleRepoEditing:       Green,
	RoleRepoReference:     Blue,
	RoleRepoOff:           Overlay1,
	RoleArtifactPlan:      Lavender,
	RoleArtifactSpec:      Pink,
	RoleArtifactResearch:  Sky,
	RoleArtifactNote:      Flamingo,
	RoleArtifactExplainer: Maroon,
}

// roleOrder fixes the render order; ranging Roles would reshuffle it each time.
var roleOrder = []string{
	RoleAccentAction, RoleAccentLabel, RoleAccentLiteral, RoleActionDestructive,
	RoleStateSelected, RoleStateSuccess, RoleStateRunning,
	RoleStateFailed, RoleStateWaiting, RoleStateGuided,
	RoleRepoEditing, RoleRepoReference, RoleRepoOff,
	RoleArtifactPlan, RoleArtifactSpec, RoleArtifactResearch, RoleArtifactNote, RoleArtifactExplainer,
}

type token struct{ name, value string }

// ramp is the only place a hex appears in the rendered file.
var ramp = []token{
	{"ctp-crust", Crust},
	{"ctp-mantle", Mantle},
	{"ctp-base", Base},
	{"ctp-surface-0", Surface0},
	{"ctp-surface-1", Surface1},
	{"ctp-surface-2", Surface2},
	{"ctp-overlay-0", Overlay0},
	{"ctp-overlay-1", Overlay1},
	{"ctp-subtext-0", Subtext0},
	{"ctp-subtext-1", Subtext1},
	{"ctp-text", Text},
	{"ctp-rosewater", Rosewater},
	{"ctp-flamingo", Flamingo},
	{"ctp-pink", Pink},
	{"ctp-blue", Blue},
	{"ctp-lavender", Lavender},
	{"ctp-sapphire", Sapphire},
	{"ctp-sky", Sky},
	{"ctp-teal", Teal},
	{"ctp-green", Green},
	{"ctp-yellow", Yellow},
	{"ctp-peach", Peach},
	{"ctp-red", Red},
	{"ctp-maroon", Maroon},
	{"ctp-mauve", Mauve},
}

// aliases say what a shade is for.
var aliases = []token{
	{"surface-app", ref("ctp-base")},
	{"surface-chrome", ref("ctp-mantle")},
	{"surface-terminal", ref("ctp-crust")},
	{"surface-raised", ref("ctp-surface-0")},

	{"text-primary", ref("ctp-text")},
	{"text-secondary", ref("ctp-subtext-0")},
	{"text-muted", ref("ctp-overlay-1")},
	{"text-faint", ref("ctp-overlay-0")},
	{"text-on-accent", ref("ctp-crust")},
	{"caret", ref("ctp-rosewater")},

	{"border-subtle", ref("ctp-surface-0")},
	{"border-default", ref("ctp-surface-2")},
	{"border-accent", ref(RoleAccentAction)},

	{"diff-add-bg", "#1c2b24"},
	{"diff-add-fg", ref("ctp-green")},
	{"diff-del-bg", "#2c2028"},
	{"diff-del-fg", ref("ctp-red")},
	{"diff-hunk-bg", ref("ctp-mantle")},
	{"diff-file-fg", ref("ctp-blue")},

	{"mac-close", ref("ctp-red")},
	{"mac-min", ref("ctp-yellow")},
	{"mac-zoom", ref("ctp-green")},
}

// CSS renders the palette as custom properties on :root. The workbench serves
// this, so no colours file exists for the frontend to drift from.
func CSS() string {
	var b strings.Builder
	b.WriteString(cssHeader)
	b.WriteString(cssRootOpen)
	write(&b, ramp)
	b.WriteString(cssRoleComment)
	for _, name := range roleOrder {
		declare(&b, name, Roles[name])
	}
	b.WriteString(cssAliasComment)
	write(&b, aliases)
	b.WriteString(cssRootClose)
	return b.String()
}

func write(b *strings.Builder, tokens []token) {
	for _, t := range tokens {
		declare(b, t.name, t.value)
	}
}

func declare(b *strings.Builder, name, value string) {
	b.WriteString(cssIndent)
	b.WriteString(cssVarPrefix)
	b.WriteString(name)
	b.WriteString(cssNameValueSep)
	b.WriteString(value)
	b.WriteString(cssTerminator)
}

func ref(name string) string { return cssRefOpen + name + cssRefClose }
