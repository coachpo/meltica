package routing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/okx/infra/rest"
	"github.com/coachpo/meltica/providers/okx/infra/wsinfra"
)

type RoutedMessage = coreprovider.RoutedMessage

type Subscription = coreprovider.Subscription

// WSDependencies exposes provider hooks used by the router.
type WSDependencies interface {
	APIKey() string
	Secret() string
	Passphrase() string
}

// WSRouter normalizes OKX websocket frames into RoutedMessage events.
type WSRouter struct {
	infra      *wsinfra.Client
	deps       WSDependencies
	orderBooks *OrderBookManager
}

// NewWSRouter constructs a websocket routing layer.
func NewWSRouter(infra *wsinfra.Client, deps WSDependencies) *WSRouter {
	return &WSRouter{infra: infra, deps: deps, orderBooks: NewOrderBookManager()}
}

type wsSub struct {
	raw *wsinfra.Subscription
	c   chan RoutedMessage
	err chan error
}

func newWSSub(raw *wsinfra.Subscription) *wsSub {
	return &wsSub{raw: raw, c: make(chan RoutedMessage, 1024), err: make(chan error, 1)}
}

func (s *wsSub) C() <-chan RoutedMessage { return s.c }
func (s *wsSub) Err() <-chan error       { return s.err }
func (s *wsSub) Close() error {
	if s.raw != nil {
		return s.raw.Close()
	}
	return nil
}

// SubscribePublic subscribes to public OKX channels.
func (w *WSRouter) SubscribePublic(ctx context.Context, topics ...string) (Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("okx/wsrouter: no topics provided")
	}
	payload, err := w.buildSubscribePayload(topics)
	if err != nil {
		return nil, err
	}
	rawSub, err := w.infra.SubscribePublic(ctx, []string{payload})
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)
	go w.readLoop(sub, false)
	return sub, nil
}

// SubscribePrivate subscribes to authenticated OKX streams.
func (w *WSRouter) SubscribePrivate(ctx context.Context, topics ...string) (Subscription, error) {
	if w.deps.APIKey() == "" || w.deps.Secret() == "" || w.deps.Passphrase() == "" {
		return nil, core.ErrNotSupported
	}
	loginPayload, err := w.buildLoginPayload()
	if err != nil {
		return nil, err
	}
	payloads := []string{loginPayload}
	if len(topics) > 0 {
		subPayload, err := w.buildSubscribePayload(topics)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, subPayload)
	}
	rawSub, err := w.infra.SubscribePrivate(ctx, payloads)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)
	go w.readLoop(sub, true)
	return sub, nil
}

func (w *WSRouter) readLoop(sub *wsSub, private bool) {
	defer close(sub.c)
	defer close(sub.err)

	rawCh := sub.raw.Raw()
	errCh := sub.raw.Err()

	for {
		select {
		case raw, ok := <-rawCh:
			if !ok {
				return
			}
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: coreprovider.RouteUnknown}
			var err error
			if private {
				err = w.parsePrivateMessage(&msg, raw.Data)
			} else {
				err = w.parsePublicMessage(&msg, raw.Data)
			}
			if err != nil {
				select {
				case sub.err <- err:
				default:
				}
				return
			}
			if msg.Topic == "" {
				continue
			}
			select {
			case sub.c <- msg:
			default:
				select {
				case <-sub.c:
				default:
				}
				sub.c <- msg
			}
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				select {
				case sub.err <- err:
				default:
				}
			}
			return
		}
	}
}

// Close releases router resources.
func (w *WSRouter) Close() error { return nil }

// WSNativeSymbol converts canonical symbol to OKX websocket symbol.
func (w *WSRouter) WSNativeSymbol(canonical string) string { return canonical }

// WSCanonicalSymbol converts OKX websocket symbol to canonical format.
func (w *WSRouter) WSCanonicalSymbol(native string) string { return native }

func (w *WSRouter) buildSubscribePayload(topics []string) (string, error) {
	args := w.buildSubscriptionArgs(topics)
	if len(args) == 0 {
		return "", fmt.Errorf("okx/wsrouter: no subscription args")
	}
	payload := map[string]any{
		"op":   "subscribe",
		"args": args,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (w *WSRouter) buildSubscriptionArgs(topics []string) []map[string]string {
	args := make([]map[string]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument := corews.ParseTopic(topic)
		if channel == "" {
			continue
		}
		providerChannel := mapper.ToProviderChannel(channel)
		if providerChannel == "" {
			providerChannel = channel
		}
		arg := map[string]string{"channel": providerChannel}
		if instrument != "" {
			arg["instId"] = instrument
		}
		args = append(args, arg)
	}
	return args
}

func (w *WSRouter) buildLoginPayload() (string, error) {
	sig := rest.BuildWSSignature(w.deps.APIKey(), w.deps.Secret(), w.deps.Passphrase())
	payload := map[string]any{
		"op": "login",
		"args": []map[string]string{
			sig,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
