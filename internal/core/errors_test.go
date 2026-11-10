package core

import (
	"errors"
	"fmt"
	"testing"
)

// A client's refusal must read as itself to the page and as unavailable to the
// core; losing either breaks the one check a caller makes before acting.
func TestAnUnavailableSentinelKeepsItsTextAndMatchesErrUnavailable(t *testing.T) {
	const text = "workbench has no command to reveal a session's directory with"
	refusal := Unavailable(text)
	other := Unavailable("another refusal")
	wrapped := fmt.Errorf("reveal: %w", refusal)

	if refusal.Error() != text {
		t.Fatalf("Error() = %q, want %q", refusal.Error(), text)
	}
	if !errors.Is(wrapped, ErrUnavailable) || !errors.Is(wrapped, refusal) {
		t.Fatalf("%v does not match both ErrUnavailable and itself", wrapped)
	}
	if errors.Is(refusal, other) || errors.Is(errors.New(text), ErrUnavailable) {
		t.Fatal("unrelated errors matched as the same refusal")
	}
}
