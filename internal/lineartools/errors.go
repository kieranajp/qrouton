package lineartools

import "errors"

var (
	ErrNoCommand   = errors.New("workbench has no command for Linear custom scripts")
	ErrNotAnObject = errors.New("coding-tools.json must hold a JSON object")
)
