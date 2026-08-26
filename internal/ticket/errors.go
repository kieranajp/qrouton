package ticket

import (
	"errors"
	"fmt"
)

// URL-shape errors. Each names the provider, because the fix is to paste a
// different link.
var (
	ErrUnsupportedProvider    = errors.New("ticket must be a Linear or Asana URL")
	ErrNotLinearIssue         = errors.New("ticket must be a Linear issue URL")
	ErrNotAsanaTask           = errors.New("ticket must be an Asana task URL")
	ErrInvalidLinearReference = errors.New("Linear issue must be an identifier like LIF-2841 or a linear.app issue URL")
)

// Credential errors, worded as the action the user has to take.
var (
	ErrNoLinearToken = errors.New("set " + linearTokenEnvVar + " to load ticket details")
	ErrNoAsanaToken  = errors.New("set " + asanaTokenEnvVar + " to load ticket details")
)

// ErrTicketNotFound means the provider answered, but with no such ticket.
var ErrTicketNotFound = errors.New("ticket not found")

func providerError(provider string, err error) error {
	return fmt.Errorf("%s: loading ticket: %w", provider, err)
}

func notFound(provider string) error {
	return fmt.Errorf("%s: %w", provider, ErrTicketNotFound)
}
