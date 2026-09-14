package account

import (
	"context"
	"encoding/json"
)

type Account struct {
	ID      string `json:"id"`
	Balance int64  `json:"balance"`
	Tier    Tier   `json:"tier"`
}

type Service struct {
	client *Client
	ledger *Ledger
}

func NewService(config Config) *Service {
	return &Service{client: NewClient(config), ledger: NewLedger(config)}
}

func (s *Service) Lookup(ctx context.Context, id string) (Account, error) {
	raw, err := s.client.Fetch(ctx, TierStandard, "/accounts/"+id)
	if err != nil {
		return Account{}, err
	}
	var account Account
	if err := json.Unmarshal(raw, &account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Service) Reconcile(ctx context.Context, id string) (int64, error) {
	account, err := s.Lookup(ctx, id)
	if err != nil {
		return 0, err
	}
	entries, err := s.ledger.Entries(ctx, account.ID)
	if err != nil {
		return account.Balance, err
	}
	total := account.Balance
	for _, entry := range entries {
		total += entry.Amount
	}
	return total, nil
}
