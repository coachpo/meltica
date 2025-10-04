package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/coinbase/infra/rest"
	"github.com/coachpo/meltica/providers/coinbase/infra/wsinfra"
)

type RoutedMessage = coreprovider.RoutedMessage

type Subscription = coreprovider.Subscription

// WSDependencies defines hooks required by the routing layer.
type WSDependencies interface {
	EnsureInstruments(ctx context.Context) error
	Native(symbol string) string
	CanonicalFromNative(native string) string
	APIKey() string
	Secret() string
	Passphrase() string
}

// WSRouter routes raw Coinbase websocket frames into normalized events.
type WSRouter struct {
	infra      *wsinfra.Client
	deps       WSDependencies
	orderBooks *OrderBookManager
}

// NewWSRouter constructs a router backed by the websocket infrastructure client.
func NewWSRouter(infra *wsinfra.Client, deps WSDependencies) *WSRouter {
	return &WSRouter{
		infra:      infra,
		deps:       deps,
		orderBooks: NewOrderBookManager(),
	}
}

type wsSub struct {
	raw *wsinfra.Subscription
	c   chan RoutedMessage
	err chan error
}

func newWSSub(raw *wsinfra.Subscription) *wsSub {
	return &wsSub{
		raw: raw,
		c:   make(chan RoutedMessage, 1024),
		err: make(chan error, 1),
	}
}

func (s *wsSub) C() <-chan RoutedMessage { return s.c }
func (s *wsSub) Err() <-chan error       { return s.err }

func (s *wsSub) Close() error {
	if s.raw != nil {
		return s.raw.Close()
	}
	return nil
}

// SubscribePublic subscribes to Coinbase public streams and routes messages to Level 3.
func (w *WSRouter) SubscribePublic(ctx context.Context, topics ...string) (Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("coinbase/wsrouter: no topics provided")
	}
	if err := w.deps.EnsureInstruments(ctx); err != nil {
		return nil, err
	}
	payload, err := w.buildSubscriptionPayload(topics, false)
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

// SubscribePrivate subscribes to Coinbase authenticated channels.
func (w *WSRouter) SubscribePrivate(ctx context.Context) (Subscription, error) {
	if w.deps.APIKey() == "" || w.deps.Secret() == "" || w.deps.Passphrase() == "" {
		return nil, core.ErrNotSupported
	}
	if err := w.deps.EnsureInstruments(ctx); err != nil {
		return nil, err
	}
	payload, err := w.buildSubscriptionPayload(nil, true)
	if err != nil {
		return nil, err
	}
	rawSub, err := w.infra.SubscribePrivate(ctx, []string{payload})
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

// WSNativeSymbol converts canonical symbol to provider-native websocket symbol.
func (w *WSRouter) WSNativeSymbol(canonical string) string {
    return w.deps.Native(canonical)
}

// WSCanonicalSymbol converts provider-native websocket symbol to canonical.
func (w *WSRouter) WSCanonicalSymbol(native string) string {
    return w.deps.CanonicalFromNative(native)
}

func (w *WSRouter) buildSubscriptionPayload(topics []string, private bool) (string, error) {
	channelMap := map[string][]string{}
	for _, topic := range topics {
		channel, symbol := corews.ParseTopic(topic)
		if channel == "" {
			continue
		}
		providerChannel := mapper.ToProviderChannel(channel)
		if providerChannel == CNBTopicUser && !private {
			continue
		}
		if providerChannel == CNBTopicUser && private {
			channelMap[providerChannel] = append(channelMap[providerChannel], "*")
			continue
		}
		if symbol == "" {
			continue
		}
		native := w.deps.Native(symbol)
		if native == "" {
			native = symbol
		}
		channelMap[providerChannel] = append(channelMap[providerChannel], native)
	}

	payload := map[string]any{
		"type":        "subscribe",
		"product_ids": uniqueFlatten(channelMap),
		"channels":    buildChannels(channelMap),
	}
	if private {
		sig, err := rest.BuildWSSignature(w.deps.APIKey(), w.deps.Secret(), w.deps.Passphrase())
		if err != nil {
			return "", err
		}
		for k, v := range sig {
			payload[k] = v
		}
		if len(payload["channels"].([]any)) == 0 {
			payload["channels"] = []any{CNBTopicUser}
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func uniqueFlatten(ch map[string][]string) []string {
	set := map[string]struct{}{}
	for _, vals := range ch {
		for _, v := range vals {
			if strings.TrimSpace(v) == "" || v == "*" {
				continue
			}
			set[v] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	return out
}

func buildChannels(target map[string][]string) []any {
	if len(target) == 0 {
		return []any{CNBTopicTicker, CNBTopicMatches, CNBTopicLevel2}
	}
	out := make([]any, 0, len(target))
	for channel, products := range target {
		if len(products) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"name":        channel,
			"product_ids": products,
		})
	}
	return out
}
