// Package userscope runs a command in a transient systemd user scope of its
// own, so systemd-oomd and the kernel can kill it without taking its siblings.
// Only Linux has one; elsewhere every command starts as given.
package userscope

const (
	systemdRun          = "systemd-run"
	userFlag            = "--user"
	scopeFlag           = "--scope"
	quietFlag           = "--quiet"
	noExpandFlag        = "--expand-environment=no"
	collectFlag         = "--collect"
	unitFlagPrefix      = "--unit="
	propertyFlag        = "-p"
	endOfFlags          = "--"
	probeCommand        = "true"
	unitSuffix          = ".scope"
	sessionUnitPrefix   = "qrouton-session-"
	tabUnitPrefix       = "qrouton-tab-"
	workbenchUnitPrefix = "app-qrouton-"
)
