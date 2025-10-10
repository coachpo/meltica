package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/coachpo/meltica/core"
	coreregistry "github.com/coachpo/meltica/core/registry"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance"
	binancebridge "github.com/coachpo/meltica/exchanges/binance/bridge"
	binancel4 "github.com/coachpo/meltica/exchanges/binance/filter"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	sharedplugin "github.com/coachpo/meltica/exchanges/shared/plugin"
	sharedrouting "github.com/coachpo/meltica/exchanges/shared/routing"
	frameworkrouter "github.com/coachpo/meltica/market_data/framework/router"
	mdfilter "github.com/coachpo/meltica/pipeline"
)

// Name exposes the registry identifier for Binance.
const Name core.ExchangeName = "binance"

var registerOnce sync.Once

// Register binds the Binance exchange factory and symbol translator into the core registries.
// Consumers should invoke Register() during application bootstrap (or import the package for side effects).
func Register() {
	registerOnce.Do(func() {
		sharedplugin.Register(sharedplugin.Registration{
			Name: Name,
			Build: func(cfg coreregistry.Config) (core.Exchange, error) {
				return binance.New(cfg.APIKey, cfg.APISecret)
			},
			SymbolTranslator: func(exchange core.Exchange) (core.SymbolTranslator, error) {
				x, ok := exchange.(*binance.Exchange)
				if !ok {
					return nil, fmt.Errorf("binance: unexpected exchange type %T", exchange)
				}
				return binance.NewSymbolTranslator(x), nil
			},
			TopicTranslator: func(core.Exchange) (core.TopicTranslator, error) {
				return binance.NewTopicTranslator(), nil
			},
			Capabilities:    binance.Capabilities(),
			ProtocolVersion: core.ProtocolVersion,
			Metadata: map[string]string{
				"provider": "Binance",
			},
		})
	})
}

func init() {
	Register()
}

// MessageTypeDescriptors exposes the Binance message descriptors consumed by the routing framework.
func MessageTypeDescriptors() []*frameworkrouter.MessageTypeDescriptor {
	return bnrouting.BinanceMessageTypeDescriptors()
}

// RegisterRouting registers Binance processors with the provided routing table.
func RegisterRouting(table *frameworkrouter.RoutingTable, deps bnrouting.WSDependencies) error {
	return bnrouting.RegisterProcessors(table, deps)
}

// NewFilterAdapter constructs a Level-4 adapter backed by the supplied exchange implementation.
func NewFilterAdapter(exchange core.Exchange) (mdfilter.Adapter, *mdfilter.AuthContext, error) {
	if exchange == nil {
		return nil, nil, fmt.Errorf("binance filter: exchange is nil")
	}
	if core.ExchangeName(exchange.Name()) != Name {
		return nil, nil, fmt.Errorf("binance filter: unsupported exchange %s", exchange.Name())
	}

	var books interface {
		BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
	}
	if provider, ok := exchange.(interface {
		BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
	}); ok {
		books = provider
	}

	var ws core.WS
	if participant, ok := exchange.(core.WebsocketParticipant); ok {
		ws = participant.WS()
	}

	var privateWS core.WS
	if privateParticipant, ok := exchange.(interface {
		PrivateWS() core.WS
	}); ok {
		privateWS = privateParticipant.PrivateWS()
	}

	var restBridge binancebridge.Dispatcher
	if restParticipant, ok := exchange.(interface {
		RESTRouter() interface{}
	}); ok {
		if router := restParticipant.RESTRouter(); router != nil {
			if dispatcher, ok := router.(sharedrouting.RESTDispatcher); ok {
				restBridge = binancebridge.NewRouterBridge(dispatcher)
			}
		}
	}

	var auth *mdfilter.AuthContext
	if creds, ok := exchange.(interface {
		Credentials() (string, string)
	}); ok {
		apiKey, secret := creds.Credentials()
		if apiKey != "" && secret != "" {
			auth = &mdfilter.AuthContext{APIKey: apiKey, Secret: secret}
		}
	}

	if books == nil && ws == nil && privateWS == nil && restBridge == nil {
		return nil, nil, fmt.Errorf("binance filter: exchange %s lacks required feeds", exchange.Name())
	}

	var (
		adapter *binancel4.Adapter
		err     error
	)

	if restBridge != nil {
		adapter, err = binancel4.NewAdapterWithREST(books, ws, privateWS, restBridge)
	} else {
		adapter, err = binancel4.NewAdapter(books, ws)
	}
	if err != nil {
		return nil, nil, err
	}
	return adapter, auth, nil
}
