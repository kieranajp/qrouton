package desktop

import "errors"

var (
	ErrNoControlSocket = errors.New("workbench has no control socket address")
	ErrStaleFrontend   = errors.New(staleFrontendError)
)
