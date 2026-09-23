package workbench

import (
	"context"

	"github.com/kieranajp/qrouton/internal/vault"
)

// VaultHost reaches the index the desktop process holds in memory.
type VaultHost interface {
	SearchVault(ctx context.Context, q vault.Query) (vault.Result, error)
	ReadVault(ctx context.Context, req vault.ReadRequest) (vault.Excerpt, error)
}

func (c *client) SearchVault(ctx context.Context, q vault.Query) (vault.Result, error) {
	res, err := c.call(ctx, Request{Op: OpVaultSearch, VaultSearch: &q})
	if err != nil {
		return vault.Result{}, err
	}
	if res.VaultResult == nil {
		return vault.Result{}, ErrVaultAnswerMissing
	}
	return *res.VaultResult, nil
}

func (c *client) ReadVault(ctx context.Context, req vault.ReadRequest) (vault.Excerpt, error) {
	res, err := c.call(ctx, Request{Op: OpVaultRead, VaultRead: &req})
	if err != nil {
		return vault.Excerpt{}, err
	}
	if res.VaultExcerpt == nil {
		return vault.Excerpt{}, ErrVaultAnswerMissing
	}
	return *res.VaultExcerpt, nil
}
