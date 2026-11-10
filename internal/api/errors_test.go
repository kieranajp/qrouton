package api

import (
	"errors"
	"testing"

	"github.com/kieranajp/qrouton/internal/core"
)

// Catches a refusal that loses its text or its sentinel crossing the wire, which
// would change what the page shows or what a caller checks for.
func TestACoreErrorKeepsItsTextAndSentinelAcrossTheWire(t *testing.T) {
	for _, tc := range []struct {
		err      error
		sentinel error
	}{
		{core.ErrNoRevealCommand, core.ErrUnavailable},
		{core.UnknownSession("ghost"), core.ErrNotFound},
		{errors.New("plain failure"), nil},
	} {
		back := FromStatus(toStatus(tc.err))
		if back.Error() != tc.err.Error() {
			t.Fatalf("%q came back as %q", tc.err, back)
		}
		if tc.sentinel != nil && !errors.Is(back, tc.sentinel) {
			t.Fatalf("%q no longer matches %v", back, tc.sentinel)
		}
	}
	if FromStatus(toStatus(nil)) != nil {
		t.Fatal("no error came back as one")
	}
}
