package core

import (
	"bytes"
	"sync"
)

// outputStream is a terminal's output: what it retains and what it emits, kept
// in one order so a replay ends exactly where the next live output begins.
type outputStream struct {
	mu   sync.Mutex
	ring *Ring
}

func newOutputStream(limit int) *outputStream {
	return &outputStream{ring: NewRing(limit)}
}

// publish copies b, because the pump reads its next chunk into the same buffer
// while a client may still hold this one.
func (s *outputStream) publish(b []byte, emit func(Output)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	emit(Output{Data: bytes.Clone(b), Offset: s.ring.Write(b)})
}

// attach hands deliver the replay from offset from, or the whole ring when from
// is nil, while no live output can pass it. An error from deliver means it
// delivered nothing.
func (s *outputStream) attach(from *uint64, deliver func(Output) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Output{Replay: true}
	if from == nil {
		// Since(0) is the whole ring whether or not it has wrapped.
		out.Data, out.Offset, _ = s.ring.Since(0)
	} else {
		out.Data, out.Offset, out.Reset = s.ring.Since(*from)
	}
	return deliver(out)
}

func (s *outputStream) ordered(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f()
}
