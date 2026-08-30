package github

import (
	"errors"
	"fmt"
)

var ErrNoToken = errors.New("no GitHub token: run `gh auth login` or set " + tokenEnvVar)

// ErrUnsupportedOwnerType means an owner is neither a user nor an organization,
// so qrouton does not know which endpoint lists its repositories.
var ErrUnsupportedOwnerType = errors.New("unsupported owner type")

func unsupportedOwnerType(ownerType, owner string) error {
	return fmt.Errorf("github: %w %q for %s", ErrUnsupportedOwnerType, ownerType, owner)
}
