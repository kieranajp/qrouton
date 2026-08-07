// Package desktop is qrouton's workbench: the Wails application, the PTY the
// conversation runs in, the OS windows the session shows the user, and the
// control socket the agent's tools reach them through.
package desktop

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Options is the session the workbench opens on.
type Options struct {
	SessionRoot string
	Socket      string
	Argv        []string
	Env         []string
}

// Run opens the workbench and blocks until the conversation window closes.
// Closing that window ends the session — there is no detached server and
// nothing survives in the background.
func Run(opts Options) error {
	if len(opts.Argv) == 0 {
		return ErrNoAgentCommand
	}
	if opts.Socket == "" {
		return ErrNoControlSocket
	}
	assets, err := frontend()
	if err != nil {
		return err
	}
	r := newWailsRenderer(assets)
	term := newTerm(opts, r.Emit)
	windows := newWindows(r, r.Emit)
	r.register(application.NewService(term))
	r.register(application.NewService(windows))
	return run(r, term, windows, opts)
}

// run is Run with the renderer already built, so the window lifecycle and the
// control socket are exercised against a double instead of a display.
func run(r renderer, term *Term, windows *Windows, opts Options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var root atomic.Pointer[string]
	root.Store(&opts.SessionRoot)
	go watchChrome(ctx, func() string { return *root.Load() }, r.Emit)

	server, err := serveControl(opts.Socket, windows, func(adopted string) { root.Store(&adopted) })
	if err != nil {
		return err
	}
	defer server.Close()

	err = r.Open(windowSpec{
		Name:   mainWindowName,
		Title:  mainWindowTitle + titleSeparator + filepath.Base(opts.SessionRoot),
		URL:    frontendRoot,
		Width:  mainWindowWidth,
		Height: mainWindowHeight,
		Focus:  true,
		OnClose: func() {
			windows.stopAll()
			term.Stop()
			r.Quit()
		},
	})
	if err != nil {
		return err
	}
	return r.Run()
}

// frontend is the embedded page tree the webview serves.
func frontend() (fs.FS, error) {
	return fs.Sub(assetFS, assetRoot)
}
