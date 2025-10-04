package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	coreprovider "github.com/coachpo/meltica/core/provider"
	"github.com/coachpo/meltica/providers/kraken/infra/wsinfra"
)

type RoutedMessage = coreprovider.RoutedMessage

type Subscription = coreprovider.Subscription

// WSDependencies defines the hooks required by the Kraken websocket router.
type WSDependencies interface {
	EnsureInstruments(ctx context.Context) error
	NativeSymbolForWS(canon string) string
	CanonicalSymbol(exch string, requested []string) string
	MapNativeToCanon(native string) string
	GetWSToken(ctx context.Context) (string, error)
}

// WSRouter routes raw Kraken websocket frames into normalized events.
type WSRouter struct {
	infra *wsinfra.Client
	deps  WSDependencies
}

// NewWSRouter creates a router bound to the websocket infrastructure client.
func NewWSRouter(infra *wsinfra.Client, deps WSDependencies) *WSRouter {
	return &WSRouter{infra: infra, deps: deps}
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

// SubscribePublic subscribes to Kraken public channels and routes messages to Level 3.
func (w *WSRouter) SubscribePublic(ctx context.Context, topics ...string) (Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("kraken/wsrouter: no topics provided")
	}
	if err := w.deps.EnsureInstruments(ctx); err != nil {
		return nil, err
	}

	payloads := make([]string, 0, len(topics))
	for _, topic := range topics {
		channel, canon := parseTopic(topic)
		if channel == "" || canon == "" {
			continue
		}
		native := w.deps.NativeSymbolForWS(canon)
		if native == "" {
			return nil, fmt.Errorf("kraken/wsrouter: unsupported symbol %s", canon)
		}
		payload := map[string]any{
			"method": "subscribe",
			"params": map[string]any{
				"channel": channel,
				"symbol":  []string{strings.ReplaceAll(native, "-", "/")},
			},
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, string(data))
	}
	if len(payloads) == 0 {
		return nil, fmt.Errorf("kraken/wsrouter: no subscription payloads built")
	}

	rawSub, err := w.infra.SubscribePublic(ctx, payloads)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)
	go w.readPublic(sub, topics)
	return sub, nil
}

// SubscribePrivate subscribes to Kraken authenticated channels using a WS token.
func (w *WSRouter) SubscribePrivate(ctx context.Context) (Subscription, error) {
	token, err := w.deps.GetWSToken(ctx)
	if err != nil {
		return nil, err
	}
	rawSub, err := w.infra.SubscribePrivate(ctx, token)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)
	go w.readPrivate(sub)
	return sub, nil
}

func (w *WSRouter) readPublic(sub *wsSub, requested []string) {
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
			if err := w.parsePublicMessage(&msg, raw.Data, requested); err != nil {
				select {
				case sub.err <- err:
				default:
				}
				return
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

func (w *WSRouter) readPrivate(sub *wsSub) {
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
			if err := w.parsePrivateMessage(&msg, raw.Data); err != nil {
				select {
				case sub.err <- err:
				default:
				}
				return
			}
			sub.c <- msg
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

// WSNativeSymbol converts a canonical symbol to Kraken websocket native format.
func (w *WSRouter) WSNativeSymbol(canonical string) string {
	return w.deps.NativeSymbolForWS(canonical)
}

// WSCanonicalSymbol converts a Kraken-native websocket symbol to canonical format.
func (w *WSRouter) WSCanonicalSymbol(native string, requested []string) string {
	return w.deps.CanonicalSymbol(native, requested)
}

func parseTopic(topic string) (string, string) {
	if topic == "" {
		return "", ""
	}
	parts := strings.SplitN(topic, ":", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}
