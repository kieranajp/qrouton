package account

import (
	"context"
	"errors"
	"time"

	shared "example.com/shared-lib"
)

var ErrAttemptsExhausted = errors.New("account: attempts exhausted")

type attempt func(context.Context) error

func retry(ctx context.Context, config Config, tier Tier, run attempt) error {
	budget := DeadlineFor(config, tier)
	ctx, cancel := withDeadline(ctx, budget)
	defer cancel()

	backoff := shared.NewBackoff(budget, config.Attempts)
	var last error
	for index := 0; index < config.Attempts; index++ {
		last = run(ctx)
		if last == nil {
			return nil
		}
		if !shared.Retryable(last) {
			return last
		}
		delay, ok := backoff.Next(index)
		if !ok {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	if last != nil {
		return last
	}
	return ErrAttemptsExhausted
}
