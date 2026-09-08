package account

import (
	"context"
	"fmt"
	"net/http"

	shared "example.com/shared-lib"
)

type Client struct {
	config    Config
	transport http.RoundTripper
}

func NewClient(config Config) *Client {
	return &Client{
		config:    config,
		transport: shared.NewTransport(config.RetryDeadline),
	}
}

func (c *Client) Fetch(ctx context.Context, tier Tier, path string) ([]byte, error) {
	var body []byte
	err := retry(ctx, c.config, tier, func(ctx context.Context) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Endpoint+path, nil)
		if err != nil {
			return err
		}
		response, err := c.transport.RoundTrip(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode >= http.StatusInternalServerError {
			return shared.Transient(fmt.Errorf("upstream %d", response.StatusCode))
		}
		body, err = shared.ReadCapped(response.Body)
		return err
	})
	return body, err
}
