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

// Catches a replay that is not lossless for bytes JSON or UTF-8 would mangle:
// invalid sequences, NULs and a multibyte rune cut at a write boundary.
func TestReplaySinceAnOffsetReturnsArbitraryBytesExactly(t *testing.T) {
	random := make([]byte, 4096)
	source := rand.New(rand.NewPCG(1, 2))
	for i := range random {
		random[i] = byte(source.UintN(256))
	}
	for _, in := range [][]byte{nil, {0}, {0xff, 0xfe, 0x80}, []byte("é")[:1], []byte("\x1b[31m\r\n"), random} {
		ring := NewRing(WindowScrollback)
		ring.Write([]byte("before"))
		offset := ring.Write(in)
		out, start, reset := ring.Since(offset)
		if reset || start != offset || !bytes.Equal(out, in) {
			t.Fatalf("Since(%d) of %q gave %q from %d, reset %v", offset, in, out, start, reset)
		}
	}
}

// Catches a resume from bytes the ring has dropped, or from a future offset,
// being served as if it continued the page's stream.
func TestReplayFromAnOffsetOutsideTheRingResetsToTheWholeRing(t *testing.T) {
	ring := NewRing(8)
	ring.Write([]byte("0123456789abc"))
	for _, offset := range []uint64{0, 4, 14, 1 << 40} {
		out, start, reset := ring.Since(offset)
		if !reset || start != 5 || string(out) != "56789abc" {
			t.Fatalf("Since(%d) = %q from %d, reset %v; want the whole ring from 5 with reset", offset, out, start, reset)
		}
	}
	for offset, want := range map[uint64]string{5: "56789abc", 9: "9abc", 13: ""} {
		out, start, reset := ring.Since(offset)
		if reset || start != offset || string(out) != want {
			t.Fatalf("Since(%d) = %q from %d, reset %v; want %q", offset, out, start, reset, want)
		}
	}
}
