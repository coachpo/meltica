package routing

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/internal"
)

// PublicDispatcher handles public stream subscriptions.
type PublicDispatcher struct {
	infra    coretransport.StreamClient
	registry *StreamRegistry
	deps     WSDependencies
}

// NewPublicDispatcher creates a new public stream dispatcher.
func NewPublicDispatcher(infra coretransport.StreamClient, deps WSDependencies) *PublicDispatcher {
	return &PublicDispatcher{
		infra:    infra,
		registry: NewStreamRegistry(deps),
		deps:     deps,
	}
}

// Subscribe subscribes to public streams.
func (d *PublicDispatcher) Subscribe(ctx context.Context, topics ...string) (Subscription, error) {
	if len(topics) == 0 {
		return nil, internal.Invalid("public dispatcher: no topics provided")
	}

	streams, err := d.buildStreams(topics)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, internal.Invalid("public dispatcher: no streams derived from topics")
	}

	topicsForInfra := make([]coretransport.StreamTopic, len(streams))
	for i, stream := range streams {
		topicsForInfra[i] = coretransport.StreamTopic{Scope: coretransport.StreamScopePublic, Name: stream}
	}

	rawSub, err := d.infra.Subscribe(ctx, topicsForInfra...)
	if err != nil {
		return nil, err
	}

	sub := newWSSub(rawSub)
	go d.pumpPublic(sub)
	return sub, nil
}

// pumpPublic reads from the raw subscription and routes messages using the registry.
func (d *PublicDispatcher) pumpPublic(sub *wsSub) {
	defer close(sub.c)
	defer close(sub.err)

	rawCh := sub.raw.Messages()
	errCh := sub.raw.Errors()

	for {
		select {
		case raw, ok := <-rawCh:
			if !ok {
				return
			}
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: corestreams.RouteUnknown}
			if err := d.parsePublicMessage(&msg, raw.Data); err != nil {
				select {
				case sub.err <- err:
				default:
				}
				return
			}
			select {
			case sub.c <- msg:
			default:
				// Backpressure: drop oldest
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

// parsePublicMessage routes a message using the stream registry.
func (d *PublicDispatcher) parsePublicMessage(msg *RoutedMessage, raw []byte) error {
	payload, stream := unwrapCombinedPayload(raw)

	var meta struct {
		Event  string `json:"e"`
		Symbol string `json:"s"`
		Time   int64  `json:"E"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		// Not a recognized format, skip
		return nil
	}

	kind := ParseStreamKind(stream, meta.Event)
	if kind == "" {
		// Unknown stream kind, skip
		return nil
	}

	return d.registry.Dispatch(msg, payload, stream, kind)
}

// buildStreams converts canonical topics to exchange-native stream names.
func (d *PublicDispatcher) buildStreams(topics []string) ([]string, error) {
	return buildStreamsForTopics(topics, d.deps)
}

// buildStreamsForTopics converts canonical topics to Binance stream names.
func buildStreamsForTopics(topics []string, deps WSDependencies) ([]string, error) {
	streams := make([]string, 0, len(topics))
	for _, topic := range topics {
		kind, instrument, err := Parse(topic)
		if err != nil {
			return nil, err
		}
		nativeChannel, err := deps.NativeTopic(kind)
		if err != nil {
			return nil, err
		}
		if !core.TopicRequiresSymbol(kind) || instrument == "" {
			streams = append(streams, nativeChannel)
			continue
		}
		nativeSymbol, err := deps.NativeSymbol(instrument)
		if err != nil {
			return nil, err
		}
		streams = append(streams, strings.ToLower(nativeSymbol)+"@"+nativeChannel)
	}
	return streams, nil
}

// PrivateDispatcher handles private stream subscriptions with listen key management.
type PrivateDispatcher struct {
	infra    coretransport.StreamClient
	registry *StreamRegistry
	deps     WSDependencies
}

// NewPrivateDispatcher creates a new private stream dispatcher.
func NewPrivateDispatcher(infra coretransport.StreamClient, deps WSDependencies) *PrivateDispatcher {
	return &PrivateDispatcher{
		infra:    infra,
		registry: NewStreamRegistry(deps),
		deps:     deps,
	}
}

// Subscribe subscribes to the private user data stream.
func (d *PrivateDispatcher) Subscribe(ctx context.Context) (Subscription, error) {
	listenKey, err := d.deps.CreateListenKey(ctx)
	if err != nil {
		return nil, err
	}

	topic := coretransport.StreamTopic{Scope: coretransport.StreamScopePrivate, Name: listenKey}
	rawSub, err := d.infra.Subscribe(ctx, topic)
	if err != nil {
		return nil, err
	}

	sub := newWSSub(rawSub)
	go d.pumpPrivate(ctx, sub, listenKey)
	return &wsPrivateWrapper{sub: sub, deps: d.deps, listenKey: listenKey}, nil
}

// pumpPrivate reads from the raw subscription, routes messages, and manages listen key keepalive.
func (d *PrivateDispatcher) pumpPrivate(ctx context.Context, sub *wsSub, listenKey string) {
	defer close(sub.c)
	defer close(sub.err)

	rawCh := sub.raw.Messages()
	errCh := sub.raw.Errors()

	keepAliveTicker := time.NewTicker(30 * time.Minute)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepAliveTicker.C:
			_ = d.deps.KeepAliveListenKey(context.Background(), listenKey)
		case raw, ok := <-rawCh:
			if !ok {
				return
			}
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: corestreams.RouteUnknown}
			if err := d.parsePrivateMessage(&msg, raw.Data); err != nil {
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

// parsePrivateMessage routes a private message using the stream registry.
func (d *PrivateDispatcher) parsePrivateMessage(msg *RoutedMessage, payload []byte) error {
	var env struct {
		Event string `json:"e"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}

	kind := ParseStreamKind("", env.Event)
	if kind == "" {
		// Unknown event, skip
		return nil
	}

	return d.registry.Dispatch(msg, payload, "", kind)
}

// Helper to unwrap combined stream payload
func unwrapCombinedPayload(raw []byte) ([]byte, string) {
	var envelope struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Stream != "" && len(envelope.Data) > 0 {
		return envelope.Data, envelope.Stream
	}
	return raw, ""
}
