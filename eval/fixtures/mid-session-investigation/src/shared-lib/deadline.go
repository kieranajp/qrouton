package shared

import (
	"context"
	"time"
)

const (
	// MaxDeadline is the estate-wide ceiling. A service asking for longer is
	// clamped to it, whatever its own configuration says.
	MaxDeadline = 30 * time.Second
	MinDeadline = 50 * time.Millisecond
)

func Clamp(budget time.Duration) time.Duration {
	switch {
	case budget > MaxDeadline:
		return MaxDeadline
	case budget < MinDeadline:
		return MinDeadline
	default:
		return budget
	}
}

func PerAttempt(budget time.Duration, attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	return Clamp(budget / time.Duration(attempts))
}

func Bounded(ctx context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, Clamp(budget))
}
