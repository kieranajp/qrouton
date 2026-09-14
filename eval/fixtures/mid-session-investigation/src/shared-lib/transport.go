package shared

import (
	"io"
	"net"
	"net/http"
	"time"
)

const bodyCap = 1 << 20

// NewTransport applies the clamped budget as the per-attempt dial and response
// timeout, so one slow attempt cannot consume the caller's whole retry window.
func NewTransport(budget time.Duration) http.RoundTripper {
	clamped := Clamp(budget)
	return &http.Transport{
		DialContext:           (&net.Dialer{Timeout: clamped}).DialContext,
		ResponseHeaderTimeout: clamped,
		IdleConnTimeout:       MaxDeadline,
	}
}

func ReadCapped(reader io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, bodyCap))
}
