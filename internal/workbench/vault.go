package workbench

import (
	"context"
	"github.com/kieranajp/qrouton/internal/vault"
)

type VaultSearchRequest struct {
	Query   string `json:"query"`
	Profile string `json:"profile,omitempty"`
}
type VaultSearchResult struct {
	Status  vault.Status      `json:"status"`
	Outcome string            `json:"outcome"`
	Results []vault.Reference `json:"results"`
}
type VaultHost interface {
	VaultStatus(context.Context) (vault.Status, error)
	ReadVault(context.Context, vault.Reference) (vault.ReadResult, error)
	SearchVault(context.Context, VaultSearchRequest) (VaultSearchResult, error)
}

func (c *client) VaultStatus(ctx context.Context) (vault.Status, error) {
	res, err := c.call(ctx, Request{Op: OpVaultStatus})
	if err != nil {
		return vault.Status{}, err
	}
	if res.VaultStatus == nil {
		return vault.Status{}, ErrVaultResponse
	}
	return *res.VaultStatus, nil
}
func (c *client) ReadVault(ctx context.Context, ref vault.Reference) (vault.ReadResult, error) {
	res, err := c.call(ctx, Request{Op: OpReadVault, VaultRead: &ref})
	if err != nil {
		return vault.ReadResult{}, err
	}
	if res.VaultRead == nil {
		return vault.ReadResult{}, ErrVaultResponse
	}
	return *res.VaultRead, nil
}
func (c *client) SearchVault(ctx context.Context, input VaultSearchRequest) (VaultSearchResult, error) {
	res, err := c.call(ctx, Request{Op: OpSearchVault, VaultSearch: &input})
	if err != nil {
		return VaultSearchResult{}, err
	}
	if res.VaultSearch == nil {
		return VaultSearchResult{}, ErrVaultResponse
	}
	return *res.VaultSearch, nil
}
