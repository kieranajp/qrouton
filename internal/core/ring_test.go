package core

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"
)

func TestReadWithoutFullReturnsTheLastScreenful(t *testing.T) {
	buffer := NewRing(WindowScrollback)
	for i := 0; i < screenLines*3; i++ {
		buffer.Write([]byte("line\n"))
	}
	if lines := strings.Count(buffer.Text(false), "\n") + 1; lines > screenLines {
		t.Fatalf("a screenful is %d lines", lines)
	}
	if lines := strings.Count(buffer.Text(true), "\n") + 1; lines <= screenLines {
		t.Fatalf("full read returned only %d lines", lines)
	}
}

func TestRingKeepsTheTailWithinItsLimit(t *testing.T) {
	buffer := NewRing(8)
	buffer.Write([]byte("0123456789abc"))
	if got := buffer.Text(true); got != "56789abc" {
		t.Fatalf("ring text = %q, want the last bytes", got)
	}
}

// Catches a tab's scrollback shrinking, growing without bound, or keeping the
// head rather than the tail once it overflows across many writes.
func TestATabRingKeepsTheLast256KiB(t *testing.T) {
	buffer := NewRing(WindowScrollback)
	var written []byte
	chunk := make([]byte, 4096)
	for i := 0; len(written) < WindowScrollback+3*len(chunk); i++ {
		for j := range chunk {
			chunk[j] = byte(i + j)
		}
		buffer.Write(chunk)
		written = append(written, chunk...)
	}

	kept := buffer.Bytes()
	if len(kept) != 256*1024 {
		t.Fatalf("the tab ring kept %d bytes, want 256 KiB", len(kept))
	}
	if !bytes.Equal(kept, written[len(written)-len(kept):]) {
		t.Fatal("the tab ring kept something other than the most recent bytes")
	}
}

// Catches a transport that is not lossless for bytes JSON or UTF-8 would mangle:
// invalid sequences, NULs and a multibyte rune cut at a chunk boundary.
func TestEncodeChunkAndDecodeInputRoundTripArbitraryBytes(t *testing.T) {
	random := make([]byte, 4096)
	source := rand.New(rand.NewPCG(1, 2))
	for i := range random {
		random[i] = byte(source.UintN(256))
	}
	for _, in := range [][]byte{nil, {0}, {0xff, 0xfe, 0x80}, []byte("é")[:1], []byte("\x1b[31m\r\n"), random} {
		out, err := DecodeInput(EncodeChunk(in).Encoded)
		if err != nil {
			t.Fatalf("decoding %q: %v", in, err)
		}
		if !bytes.Equal(out, in) {
			t.Fatalf("round trip of %q gave %q", in, out)
		}
	}
}
