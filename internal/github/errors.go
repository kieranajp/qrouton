package github

import (
	"errors"
	"fmt"
	"time"
)

var ErrNoToken = errors.New("no GitHub token: run `gh auth login` or set " + tokenEnvVar)

var ErrTokenRejected = errors.New("GitHub rejected the token")

var (
	ErrIssueInvalid  = errors.New("a bug report needs a nonblank title and body")
	ErrIssueRejected = errors.New("GitHub rejected the bug report")
	ErrIssueUnknown  = errors.New("an issue may have been created; inspect " + IssueListURL + " before retrying")
)

// ErrUnsupportedOwnerType means an owner is neither a user nor an organization,
// so qrouton does not know which endpoint lists its repositories.
var ErrUnsupportedOwnerType = errors.New("unsupported owner type")

func unsupportedOwnerType(ownerType, owner string) error {
	return fmt.Errorf("github: %w %q for %s", ErrUnsupportedOwnerType, ownerType, owner)
}

var ErrNotFound = errors.New("not found on GitHub")

var ErrRateLimited = errors.New("GitHub rate limit reached")

// RateLimitError carries when GitHub accepts requests again; errors.Is(err, ErrRateLimited) holds.
type RateLimitError struct{ Until time.Time }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf(rateLimitedFormat, ErrRateLimited, e.Until.Format(time.RFC3339))
}

func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

var ErrUnexpectedStatus = errors.New("GitHub answered with an unexpected status")

var ErrQuery = errors.New("GitHub reported a query error")
