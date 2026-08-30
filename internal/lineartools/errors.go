package lineartools

import "errors"

var (
	ErrNoCommand   = errors.New("workbench has no command for Linear custom scripts")
	ErrNotAnObject = errors.New("must be a JSON object")
)
