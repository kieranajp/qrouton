package vault

import (
	"slices"
	"testing"
)

func TestTokensMeetAcrossWordForms(t *testing.T) {
	for _, pair := range [][2]string{{"running", "run"}, {"planned", "plan"}, {"stopped", "stop"}, {"queries", "query"}} {
		a, b := tokens(pair[0]), tokens(pair[1])
		if !slices.Equal(a, b) {
			t.Errorf("tokens(%q) = %v, tokens(%q) = %v", pair[0], a, pair[1], b)
		}
	}
}
