// Package randomid makes random hex identifiers.
package randomid

import (
	"crypto/rand"
	"encoding/hex"
)

// Hex returns n random bytes as lowercase hex.
func Hex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
