package rest

import (
	"context"
	"net/http"
	"time"

	coreprovider "github.com/coachpo/meltica/core/provider"
	"github.com/coachpo/meltica/transport"
)

// Config configures the Coinbase REST infrastructure client.
type Config struct {
	APIKey     string
	Secret     string
	Passphrase string
	HTTPClient *http.Client
}

// Client manages normalized REST calls for Coinbase.
type Client struct {
	transport *transport.Client
}

// NewClient constructs a REST infrastructure client with signing and error handling.
func NewClient(cfg Config) (*Client, error) {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	t := &transport.Client{
		HTTP:    httpClient,
		BaseURL: "https://api.exchange.coinbase.com",
		Retry: transport.RetryPolicy{
			MaxRetries: 3,
			BaseDelay:  250 * time.Millisecond,
			MaxDelay:   1500 * time.Millisecond,
		},
		DefaultHeaders: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"User-Agent":   "meltica-coinbase/0.1",
		},
		OnHTTPError: mapHTTPError,
	}
	if cfg.APIKey != "" && cfg.Secret != "" && cfg.Passphrase != "" {
		signer, err := newSigner(cfg.APIKey, cfg.Secret, cfg.Passphrase)
		if err != nil {
			return nil, err
		}
		t.Signer = signer
	}
	return &Client{transport: t}, nil
}

// Do executes a normalized REST request.
func (c *Client) Do(ctx context.Context, req coreprovider.RESTRequest, out any) error {
	return c.transport.Do(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, out)
}

// DoWithHeaders executes a request and returns response headers.
func (c *Client) DoWithHeaders(ctx context.Context, req coreprovider.RESTRequest, out any) (http.Header, error) {
	return c.transport.DoWithHeaders(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, out)
}

// Transport exposes the underlying transport client for advanced workflows.
func (c *Client) Transport() *transport.Client { return c.transport }
