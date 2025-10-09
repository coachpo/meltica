package infra

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/capabilities"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/shared/infra/transport"
	"github.com/coachpo/meltica/exchanges/shared/routing"
)

// AdapterFactory exposes utilities that simplify building canonical exchange adapters.
type AdapterFactory struct {
	name core.ExchangeName
}

// NewAdapterFactory initializes a factory scoped to the provided exchange name.
func NewAdapterFactory(name core.ExchangeName) *AdapterFactory {
	trimmed := core.ExchangeName(strings.TrimSpace(string(name)))
	return &AdapterFactory{name: trimmed}
}

// RegisterSymbolTranslator wires the symbol translator into the core registry.
func (f *AdapterFactory) RegisterSymbolTranslator(translator core.SymbolTranslator) {
	if f == nil || translator == nil {
		return
	}
	core.RegisterSymbolTranslator(f.name, translator)
}

// RegisterTopicTranslator wires the topic translator into the core registry using the provided factory.
func (f *AdapterFactory) RegisterTopicTranslator(factory func() (core.TopicTranslator, error)) {
	if f == nil || factory == nil {
		return
	}
	translator, err := factory()
	if err != nil || translator == nil {
		return
	}
	core.RegisterTopicTranslator(f.name, translator)
}

// RESTClientConfig configures creation of a shared transport client.
type RESTClientConfig struct {
	BaseURL        string
	HTTPClient     *http.Client
	Retry          transport.RetryPolicy
	RateLimiter    transport.RateLimiter
	Signer         transport.Signer
	DefaultHeaders map[string]string
	CanonicalError errs.CanonicalCode
	DecorateError  func(*errs.E)
}

// NewRESTClient provisions a transport client with canonical error mapping.
func (f *AdapterFactory) NewRESTClient(cfg RESTClientConfig) (*transport.Client, error) {
	if f == nil {
		return nil, fmt.Errorf("infra: adapter factory nil")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("infra: base url required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	client := &transport.Client{
		HTTP:           httpClient,
		Retry:          cfg.Retry,
		RateLimiter:    cfg.RateLimiter,
		Signer:         cfg.Signer,
		BaseURL:        baseURL,
		DefaultHeaders: cloneStringMap(cfg.DefaultHeaders),
	}
	client.OnHTTPError = func(status int, body []byte) error {
		envelope := errs.New(string(f.name), errs.CodeExchange,
			errs.WithHTTP(status),
			errs.WithCanonicalCode(cfg.CanonicalError),
			errs.WithRawMessage(string(body)),
		)
		if cfg.DecorateError != nil {
			cfg.DecorateError(envelope)
		}
		return envelope
	}
	return client, nil
}

// ExchangeConfig describes the services exposed by the composite exchange.
type ExchangeConfig struct {
	Capabilities core.ExchangeCapabilities
	Version      string
	Spot         core.SpotAPI
	Linear       core.FuturesAPI
	Inverse      core.FuturesAPI
	WS           core.WS
	OnClose      func() error
}

// BuildExchange composes a canonical exchange implementation from supplied services.
func (f *AdapterFactory) BuildExchange(cfg ExchangeConfig) (core.Exchange, error) {
	if f == nil {
		return nil, fmt.Errorf("infra: adapter factory nil")
	}
	return &compositeExchange{
		name:    f.name,
		caps:    cfg.Capabilities,
		version: strings.TrimSpace(cfg.Version),
		spot:    cfg.Spot,
		linear:  cfg.Linear,
		inverse: cfg.Inverse,
		ws:      cfg.WS,
		onClose: cfg.OnClose,
	}, nil
}

// OrderAction aliases the shared routing order action type.
type OrderAction = routing.OrderAction

// OrderRouterConfig bundles dispatcher, translator, and capability metadata for router construction.
type OrderRouterConfig struct {
	Dispatcher   routing.RESTDispatcher
	Translator   routing.OrderTranslator
	Capabilities capabilities.Set
	Requirements map[OrderAction][]capabilities.Capability
}

// NewOrderRouter creates a capability-aware order router using shared routing helpers.
func (f *AdapterFactory) NewOrderRouter(cfg OrderRouterConfig) (*routing.OrderRouter, error) {
	if cfg.Dispatcher == nil {
		return nil, fmt.Errorf("infra: order router requires dispatcher")
	}
	if cfg.Translator == nil {
		return nil, fmt.Errorf("infra: order router requires translator")
	}
	options := []routing.OrderRouterOption{}
	if cfg.Capabilities != 0 {
		options = append(options, routing.WithCapabilities(cfg.Capabilities))
	}
	for action, deps := range cfg.Requirements {
		if len(deps) == 0 {
			continue
		}
		options = append(options, routing.WithCapabilityRequirement(routing.OrderAction(action), deps...))
	}
	return routing.NewOrderRouter(cfg.Dispatcher, cfg.Translator, options...)
}

type compositeExchange struct {
	name    core.ExchangeName
	version string
	caps    core.ExchangeCapabilities
	spot    core.SpotAPI
	linear  core.FuturesAPI
	inverse core.FuturesAPI
	ws      core.WS
	onClose func() error
}

func (c *compositeExchange) Name() string { return string(c.name) }

func (c *compositeExchange) Capabilities() core.ExchangeCapabilities { return c.caps }

func (c *compositeExchange) SupportedProtocolVersion() string {
	if v := strings.TrimSpace(c.version); v != "" {
		return v
	}
	return core.ProtocolVersion
}

func (c *compositeExchange) Close() error {
	if c.onClose != nil {
		return c.onClose()
	}
	return nil
}

func (c *compositeExchange) Spot(context.Context) core.SpotAPI {
	return c.spot
}

func (c *compositeExchange) LinearFutures(context.Context) core.FuturesAPI {
	return c.linear
}

func (c *compositeExchange) InverseFutures(context.Context) core.FuturesAPI {
	return c.inverse
}

func (c *compositeExchange) WS() core.WS { return c.ws }

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Ensure compositeExchange implements optional participant interfaces when services are supplied.
var (
	_ core.Exchange                  = (*compositeExchange)(nil)
	_ core.SpotParticipant           = (*compositeExchange)(nil)
	_ core.LinearFuturesParticipant  = (*compositeExchange)(nil)
	_ core.InverseFuturesParticipant = (*compositeExchange)(nil)
	_ core.WebsocketParticipant      = (*compositeExchange)(nil)
)
