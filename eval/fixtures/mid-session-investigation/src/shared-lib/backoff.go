package shared

import (
	"math"
	"time"
)

const (
	minimumDelay = 10 * time.Millisecond
	// jitterShare is the fraction of each delay left to the caller's own jitter.
	jitterShare = 0.2
)

type Backoff struct {
	budget   time.Duration
	attempts int
	spent    time.Duration
}

// NewBackoff spreads the whole budget across the attempts, so a caller that
// raises its deadline gets longer waits rather than more of them.
func NewBackoff(budget time.Duration, attempts int) *Backoff {
	if attempts < 1 {
		attempts = 1
	}
	return &Backoff{budget: budget, attempts: attempts}
}

func (b *Backoff) Next(index int) (time.Duration, bool) {
	if index+1 >= b.attempts {
		return 0, false
	}
	share := float64(b.budget) / math.Pow(2, float64(b.attempts-index))
	delay := time.Duration(share * (1 - jitterShare))
	if delay < minimumDelay {
		delay = minimumDelay
	}
	if b.spent+delay > b.budget {
		return 0, false
	}
	b.spent += delay
	return delay, true
}

func (b *Backoff) Remaining() time.Duration {
	return b.budget - b.spent
}
