package rest

import (
	"context"
	"net/http"
	"strings"
	"time"

	coreexchange "github.com/coachpo/meltica/core/exchange"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/transport"
)

// API indicates which Binance REST surface should process the request.
type API string

const (
	SpotAPI    API = "spot"
	LinearAPI  API = "linear"
	InverseAPI API = "inverse"
)

// Config configures the Binance REST infrastructure client.
type Config struct {
	APIKey         string
	Secret         string
	HTTPClient     *http.Client
	SpotBaseURL    string
	LinearBaseURL  string
	InverseBaseURL string
	Timeout        time.Duration
}

// Client manages low-level REST connections to Binance endpoints (Level 1 infrastructure).
type Client struct {
	key    string
	secret string

	sapi *transport.Client
	fapi *transport.Client
	dapi *transport.Client
}

const (
	defaultSpotBase    = "https://api.binance.com"
	defaultLinearBase  = "https://fapi.binance.com"
	defaultInverseBase = "https://dapi.binance.com"
)

// NewClient creates a REST infrastructure client with shared signing and error handling.
func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	} else if httpClient.Timeout == 0 && cfg.Timeout > 0 {
		httpClient.Timeout = cfg.Timeout
	}

	spotBase := cfg.SpotBaseURL
	if strings.TrimSpace(spotBase) == "" {
		spotBase = defaultSpotBase
	}
	linearBase := cfg.LinearBaseURL
	if strings.TrimSpace(linearBase) == "" {
		linearBase = defaultLinearBase
	}
	inverseBase := cfg.InverseBaseURL
	if strings.TrimSpace(inverseBase) == "" {
		inverseBase = defaultInverseBase
	}

	c := &Client{
		key:    cfg.APIKey,
		secret: cfg.Secret,
		sapi:   &transport.Client{HTTP: httpClient, BaseURL: spotBase},
		fapi:   &transport.Client{HTTP: httpClient, BaseURL: linearBase},
		dapi:   &transport.Client{HTTP: httpClient, BaseURL: inverseBase},
	}

	signer := func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
		hdr, err := signHMAC(c.secret, method, path, q, body, ts)
		if hdr == nil {
			hdr = http.Header{}
		}
		if c.key != "" {
			hdr.Set("X-MBX-APIKEY", c.key)
		}
		return hdr, err
	}

	c.sapi.Signer = signer
	c.fapi.Signer = signer
	c.dapi.Signer = signer

	c.sapi.OnHTTPError = mapHTTPError
	c.fapi.OnHTTPError = mapHTTPError
	c.dapi.OnHTTPError = mapHTTPError

	return c
}

// Request describes a low-level REST call routed through the Binance infrastructure client.
// Do executes a REST request against the appropriate Binance surface and unmarshals the response into out.

func (c *Client) Do(ctx context.Context, req coreexchange.RESTRequest, out any) error {
	switch API(req.API) {
	case SpotAPI:
		return c.sapi.Do(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, out)
	case LinearAPI:
		return c.fapi.Do(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, out)
	case InverseAPI:
		return c.dapi.Do(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, out)
	default:
		return internal.Invalid("rest: unsupported api %q", req.API)
	}
}

// Spot exposes the underlying spot transport client for advanced workflows.
func (c *Client) Spot() *transport.Client { return c.sapi }

// Linear exposes the underlying linear futures transport client for advanced workflows.
func (c *Client) Linear() *transport.Client { return c.fapi }

// Inverse exposes the underlying coin-margined futures transport client for advanced workflows.
func (c *Client) Inverse() *transport.Client { return c.dapi }
