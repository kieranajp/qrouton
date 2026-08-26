package desktop

// renderer is the seam between the workbench's policy and the toolkit that
// draws it; only the implementation links WebKit.
type renderer interface {
	Open(spec windowSpec) error
	// Retitle renames an open window, which is how the conversation learns the
	// name of a session onboarding chose after it opened.
	Retitle(name, title string)
	Focus(name string)
	// Emit delivers a payload to the pages of every open window.
	Emit(event string, payload any)
	// Run blocks on the event loop until the application quits.
	Run() error
	Quit()
}

// windowSpec describes the main conversation window to the renderer.
type windowSpec struct {
	Name    string
	Title   string
	URL     string
	Width   int
	Height  int
	Focus   bool
	OnClose func()
}
