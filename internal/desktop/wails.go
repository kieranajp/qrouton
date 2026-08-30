package desktop

import (
	"io/fs"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type wailsRenderer struct {
	app     *application.App
	running atomic.Bool
}

func newWailsRenderer(assets fs.FS, icon []byte) *wailsRenderer {
	r := &wailsRenderer{}
	r.app = application.New(application.Options{
		Name:        applicationName,
		Description: applicationDescription,
		Icon:        icon,
		Assets:      application.AssetOptions{Handler: assetHandler(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	r.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		r.running.Store(true)
	})
	return r
}

func (r *wailsRenderer) register(service application.Service) {
	r.app.RegisterService(service)
}

func (r *wailsRenderer) Open(spec windowSpec) error {
	r.onMain(func() {
		window := r.app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:   spec.Name,
			Title:  spec.Title,
			URL:    spec.URL,
			Width:  spec.Width,
			Height: spec.Height,
		})
		if spec.OnClose != nil {
			window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { spec.OnClose() })
		}
		window.Show()
		if !spec.Focus {
			if conversation, ok := r.app.Window.Get(mainWindowName); ok {
				conversation.Focus()
			}
		}
	})
	return nil
}

func (r *wailsRenderer) Retitle(name, title string) {
	r.onMain(func() {
		if window, ok := r.app.Window.Get(name); ok {
			window.SetTitle(title)
		}
	})
}

func (r *wailsRenderer) Focus(name string) {
	r.onMain(func() {
		if window, ok := r.app.Window.Get(name); ok {
			window.Focus()
		}
	})
}

func (r *wailsRenderer) Emit(event string, payload any) {
	r.app.Event.Emit(event, payload)
}

func (r *wailsRenderer) Run() error { return r.app.Run() }

func (r *wailsRenderer) Quit() { r.app.Quit() }

// chooseDirectory prompts for a directory, answering "" on a cancel.
// PromptForSingleSelection marshals to the main thread itself, so onMain must
// not be layered on top of it.
func (r *wailsRenderer) chooseDirectory() (string, error) {
	return r.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		PromptForSingleSelection()
}

// onMain runs work on the main thread, which is where window construction has
// to happen. InvokeSync has no loop to marshal onto until the application has
// started, so work queued before then runs inline.
func (r *wailsRenderer) onMain(work func()) {
	if r.running.Load() {
		application.InvokeSync(work)
		return
	}
	work()
}
