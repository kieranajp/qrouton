package shared

import (
	"errors"
	"net"
)

type transient struct{ error }

func (t transient) Transient() bool { return true }

func Transient(err error) error {
	if err == nil {
		return nil
	}
	return transient{err}
}

func Retryable(err error) bool {
	var marked interface{ Transient() bool }
	if errors.As(err, &marked) {
		return marked.Transient()
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
