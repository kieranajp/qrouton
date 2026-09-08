package account

import (
	"context"
	"encoding/json"
	"time"
)

type Entry struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount"`
}

// Ledger keeps its own timeout: reconciliation is a batch read and does not
// share the request-path retry budget.
type Ledger struct {
	client  *Client
	timeout time.Duration
}

func NewLedger(config Config) *Ledger {
	return &Ledger{client: NewClient(config), timeout: config.LedgerTimeout}
}

func (l *Ledger) Entries(ctx context.Context, accountID string) ([]Entry, error) {
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	raw, err := l.client.Fetch(ctx, TierBulk, "/ledger/"+accountID)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
