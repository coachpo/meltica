package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	coreprovider "github.com/coachpo/meltica/core/provider"
	"github.com/coachpo/meltica/transport"
)

// Config configures the OKX REST infrastructure client.
type Config struct {
	APIKey     string
	Secret     string
	Passphrase string
	HTTPClient *http.Client
}

// Client manages normalized REST calls for OKX.
type Client struct {
	transport *transport.Client
}

// NewClient constructs a REST client with shared signing and error handling.
func NewClient(cfg Config) (*Client, error) {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	t := &transport.Client{
		HTTP:    httpClient,
		BaseURL: "https://www.okx.com",
		OnHTTPError: func(status int, body []byte) error {
			return mapCode(status, "", string(body))
		},
	}
	if cfg.APIKey != "" && cfg.Secret != "" && cfg.Passphrase != "" {
		t.Signer = newSigner(cfg.APIKey, cfg.Secret, cfg.Passphrase)
	}
	return &Client{transport: t}, nil
}

// Do executes a normalized REST request and unmarshals the OKX envelope.
func (c *Client) Do(ctx context.Context, req coreprovider.RESTRequest, out any) error {
	var env struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := c.transport.Do(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, &env); err != nil {
		return err
	}
	if env.Code != "0" && env.Code != "" {
		return mapCode(http.StatusOK, env.Code, env.Msg)
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// Transport exposes the underlying transport for advanced use cases.
func (c *Client) Transport() *transport.Client { return c.transport }
