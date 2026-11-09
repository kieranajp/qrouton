// Package core is the workbench without a display: what runs sessions, and the
// one interface through which it reaches the machine the human sits at.
package core

// Client is the core's only way to touch a window or run a local tool. A call
// the client cannot make returns an error matching ErrUnavailable.
type Client interface {
	Emit(event string, payload any)
	Focused() bool
	Raise()
	SessionShown(root string)
	Supports(Capability) bool
	Reveal(path string) error
	OpenText(path string) error
	CopyImage(path string) error
	CopyFile(path string) error
	ChooseDirectory() (string, error)
	// Chime takes the session root because the local client rings that
	// session's own notify script.
	Chime(sessionRoot string)
}

// Emitter broadcasts a payload to every page the client shows.
type Emitter func(event string, payload any)

// Capability names a client call that a client may not support.
type Capability string

const (
	CapabilityReveal          Capability = "reveal"
	CapabilityOpenText        Capability = "openText"
	CapabilityCopyImage       Capability = "copyImage"
	CapabilityCopyFile        Capability = "copyFile"
	CapabilityChooseDirectory Capability = "chooseDirectory"
)
