package randomid

import (
	"regexp"
	"testing"
)

var lowerHex = regexp.MustCompile(`^[0-9a-f]*$`)

func TestHexIsLowercaseHexOfTwiceTheByteCount(t *testing.T) {
	for _, n := range []int{0, 1, 6, 16} {
		got := Hex(n)
		if len(got) != 2*n {
			t.Errorf("Hex(%d) length = %d, want %d", n, len(got), 2*n)
		}
		if !lowerHex.MatchString(got) {
			t.Errorf("Hex(%d) = %q, want lowercase hex", n, got)
		}
	}
}

func TestHexDiffersBetweenCalls(t *testing.T) {
	if Hex(16) == Hex(16) {
		t.Fatal("two 16-byte ids matched")
	}
}
