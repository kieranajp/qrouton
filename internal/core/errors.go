package core

import "errors"

var ErrUnavailable = errors.New("this client cannot do that")

// Unavailable is a sentinel with its own text that also matches ErrUnavailable,
// so a caller can test for the one refusal or for any.
func Unavailable(text string) error {
	return &unavailable{text: text}
}

type unavailable struct{ text string }

func (u *unavailable) Error() string { return u.text }

func (u *unavailable) Is(target error) bool { return target == ErrUnavailable }

var ErrTerminalNotStarted = errors.New("terminal is not started")
