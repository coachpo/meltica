package rest

import (
	"context"
	"net/http"
	"time"

	coreprovider "github.com/coachpo/meltica/core/provider"
	"github.com/coachpo/meltica/transport"
)

// API enumerates Kraken REST surfaces.
type API string

const (
	SpotAPI    API = "spot"
	FuturesAPI API = "futures"
)

// Config configures the Kraken REST infrastructure client.
type Config struct {
	APIKey     string
	Secret     string
	HTTPClient *http.Client
}

// Client wraps low-level REST transports for Kraken surfaces.
type Client struct {
	spot    *transport.Client
	futures *transport.Client
}

const (
	spotBase    = "https://api.kraken.com"
	futuresBase = "https://futures.kraken.com/derivatives"
)

// NewClient builds a REST infrastructure client with shared signing and error handling.
func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	baseClient := func(base string) *transport.Client {
		c := &transport.Client{
			HTTP:    httpClient,
			BaseURL: base,
			Retry: transport.RetryPolicy{
				MaxRetries: 3,
				BaseDelay:  250 * time.Millisecond,
				MaxDelay:   1500 * time.Millisecond,
			},
			DefaultHeaders: map[string]string{
				"Accept":     "application/json",
				"User-Agent": "meltica-kraken/0.1",
			},
		}
		c.OnHTTPError = mapHTTPError
		if cfg.APIKey != "" && cfg.Secret != "" {
			c.Signer = newSigner(cfg.APIKey, cfg.Secret)
		}
		return c
	}

	return &Client{
		spot:    baseClient(spotBase),
		futures: baseClient(futuresBase),
	}
}

// Do executes a normalized REST request against the appropriate Kraken surface.
func (c *Client) Do(ctx context.Context, req coreprovider.RESTRequest, out any) error {
	client := c.spot
	if API(req.API) == FuturesAPI && c.futures != nil {
		client = c.futures
	}
	if client == nil {
		client = c.spot
	}
	return client.Do(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, out)
}

// Spot exposes the underlying spot transport for advanced scenarios.
func (c *Client) Spot() *transport.Client { return c.spot }

// Futures exposes the futures transport for advanced scenarios.
func (c *Client) Futures() *transport.Client { return c.futures }
