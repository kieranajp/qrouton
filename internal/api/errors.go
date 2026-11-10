package api

import (
	"errors"
	"fmt"

	"github.com/kieranajp/qrouton/internal/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrNoClient = errors.New("this call carries no client id from Hello")

func protocolMismatch(client, core uint32) error {
	return fmt.Errorf("the client speaks protocol %d and the core speaks %d", client, core)
}

// wireErrors is the one table between core sentinels and status codes, read in
// both directions so a refusal still matches its sentinel after the trip.
var wireErrors = []struct {
	sentinel error
	code     codes.Code
	rebuild  func(text string) error
}{
	{core.ErrUnavailable, codes.Unimplemented, core.Unavailable},
	{core.ErrNotFound, codes.NotFound, core.NotFound},
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	for _, row := range wireErrors {
		if errors.Is(err, row.sentinel) {
			return status.Error(row.code, err.Error())
		}
	}
	return status.Error(codes.Unknown, err.Error())
}

// FromStatus turns a call's error back into one with the text the core gave it.
func FromStatus(err error) error {
	if err == nil {
		return nil
	}
	s, ok := status.FromError(err)
	if !ok {
		return err
	}
	for _, row := range wireErrors {
		if s.Code() == row.code {
			return row.rebuild(s.Message())
		}
	}
	return errors.New(s.Message())
}
