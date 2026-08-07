package desktop

// renderer is the seam between the workbench's policy and the toolkit that
// draws it; only the implementation links WebKit.
type renderer interface {
	Open(spec windowSpec) error
	// Close takes a window off the screen; its OnClose still fires.
	Close(name string)
	// Emit delivers a payload to the pages of every open window.
	Emit(event string, payload any)
	// Run blocks on the event loop until the application quits.
	Run() error
	Quit()
}

// windowSpec describes a window to the renderer. URL names a directory: a page
// path 301-redirects to it, the webview does not follow that, and the window
// comes up blank with nothing reported anywhere.
type windowSpec struct {
	Name    string
	Title   string
	URL     string
	Width   int
	Height  int
	Focus   bool
	OnClose func()
}
