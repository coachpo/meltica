package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	coreexchange "github.com/coachpo/meltica/core/exchange"
	"github.com/coachpo/meltica/errs"
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

// Connect satisfies the exchange.Connection interface; REST connections are established lazily per request.
func (c *Client) Connect(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

// Close satisfies the exchange.Connection interface. REST clients keep no persistent resources.
func (c *Client) Close() error { return nil }

// DoRequest issues a REST call and returns the raw response payload for upper layers.
func (c *Client) DoRequest(ctx context.Context, req coreexchange.RESTRequest) (*coreexchange.RESTResponse, error) {
	client, err := c.clientForAPI(API(req.API))
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	hdr, status, err := client.DoWithHeaders(ctx, req.Method, req.Path, req.Query, req.Body, req.Signed, req.Header, &raw)
	if err != nil {
		return nil, err
	}
	var headerCopy http.Header
	if hdr != nil {
		headerCopy = hdr.Clone()
	}
	body := append([]byte(nil), raw...)
	return &coreexchange.RESTResponse{
		Status:     status,
		Header:     headerCopy,
		Body:       body,
		ReceivedAt: time.Now(),
	}, nil
}

// HandleResponse decodes the raw RESTResponse into the provided output structure when required.
func (c *Client) HandleResponse(ctx context.Context, req coreexchange.RESTRequest, resp *coreexchange.RESTResponse, out any) error {
	if out == nil || resp == nil {
		return nil
	}
	if len(resp.Body) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return internal.WrapExchange(err, "rest: decode %s %s", req.Method, req.Path)
	}
	return nil
}

// HandleError normalises transport failures into canonical error types.
func (c *Client) HandleError(ctx context.Context, req coreexchange.RESTRequest, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var e *errs.E
	if errors.As(err, &e) {
		return err
	}
	return internal.WrapNetwork(err, "rest: %s %s", req.Method, req.Path)
}

// Do executes a REST request against the appropriate Binance surface and unmarshals the response into out.
func (c *Client) Do(ctx context.Context, req coreexchange.RESTRequest, out any) error {
	resp, err := c.DoRequest(ctx, req)
	if err != nil {
		return c.HandleError(ctx, req, err)
	}
	return c.HandleResponse(ctx, req, resp, out)
}

func (c *Client) clientForAPI(api API) (*transport.Client, error) {
	switch api {
	case SpotAPI, "":
		return c.sapi, nil
	case LinearAPI:
		return c.fapi, nil
	case InverseAPI:
		return c.dapi, nil
	default:
		return nil, internal.Invalid("rest: unsupported api %q", api)
	}
}

// Spot exposes the underlying spot transport client for advanced workflows.
func (c *Client) Spot() *transport.Client { return c.sapi }

// Linear exposes the underlying linear futures transport client for advanced workflows.
func (c *Client) Linear() *transport.Client { return c.fapi }

// Inverse exposes the underlying coin-margined futures transport client for advanced workflows.
func (c *Client) Inverse() *transport.Client { return c.dapi }
