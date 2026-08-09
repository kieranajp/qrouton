package theme

// Role names, which are the custom property names without their dashes.
const (
	RoleAccentAction  = "accent-action"
	RoleAccentLabel   = "accent-label"
	RoleStateSelected = "state-selected"
	RoleStateSuccess  = "state-success"
	RoleStateRunning  = "state-running"
	RoleStateFailed   = "state-failed"
	RoleStateWaiting  = "state-waiting"
	RoleStateGuided   = "state-guided"
	RoleRepoEditing   = "role-editing"
	RoleRepoReference = "role-reference"
	RoleRepoOff       = "role-off"
)

// The stylesheet CSS renders, and the path the workbench serves it at.
const (
	Path      = "/tokens/colors.css"
	MediaType = "text/css; charset=utf-8"

	cssHeader = "/* Catppuccin Macchiato, rendered from internal/theme. The neutral ramp is\n" +
		"   the palette verbatim; each accent carries one job and must not be reused.\n" +
		"   Nothing generates a file: the workbench serves this. */\n"
	cssRootOpen  = ":root {\n"
	cssRootClose = "}\n"

	cssRoleComment  = "\n  /* one job each; do not borrow */\n"
	cssAliasComment = "\n  /* what a shade is for */\n"

	cssIndent       = "  "
	cssVarPrefix    = "--"
	cssNameValueSep = ": "
	cssTerminator   = ";\n"
	cssRefOpen      = "var(--"
	cssRefClose     = ")"
)
