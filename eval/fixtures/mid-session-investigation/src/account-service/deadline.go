package account

import (
	"context"
	"time"
)

type Tier string

const (
	TierStandard Tier = "standard"
	TierPriority Tier = "priority"
	TierBulk     Tier = "bulk"
)

// tierMultiplier scales the configured deadline per caller tier. A bulk caller
// is allowed to wait longer than the operator configured.
var tierMultiplier = map[Tier]float64{
	TierStandard: 1,
	TierPriority: 0.5,
	TierBulk:     4,
}

func DeadlineFor(config Config, tier Tier) time.Duration {
	multiplier, ok := tierMultiplier[tier]
	if !ok {
		multiplier = 1
	}
	return time.Duration(float64(config.RetryDeadline) * multiplier)
}

func withDeadline(ctx context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if existing, ok := ctx.Deadline(); ok {
		if remaining := time.Until(existing); remaining < budget {
			return context.WithCancel(ctx)
		}
	}
	return context.WithTimeout(ctx, budget)
}
