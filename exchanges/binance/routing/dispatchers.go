package routing

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	frameworkrouter "github.com/coachpo/meltica/market_data/framework/router"
)

var privateKeepAliveInterval = 30 * time.Minute

type PublicDispatcher struct {
	infra      coretransport.StreamClient
	deps       WSDependencies
	table      *frameworkrouter.RoutingTable
	dispatcher *frameworkrouter.RouterDispatcher
	hub        *processorHub
}

func NewPublicDispatcher(infra coretransport.StreamClient, deps WSDependencies, table *frameworkrouter.RoutingTable, dispatcher *frameworkrouter.RouterDispatcher, hub *processorHub) *PublicDispatcher {
	return &PublicDispatcher{infra: infra, deps: deps, table: table, dispatcher: dispatcher, hub: hub}
}

func (d *PublicDispatcher) Subscribe(ctx context.Context, topics ...string) (Subscription, error) {
	if len(topics) == 0 {
		return nil, internal.Invalid("public dispatcher: no topics provided")
	}
	streams, err := buildStreamsForTopics(topics, d.deps)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, internal.Invalid("public dispatcher: no streams derived from topics")
	}
	request := make([]coretransport.StreamTopic, len(streams))
	for i, stream := range streams {
		request[i] = coretransport.StreamTopic{Scope: coretransport.StreamScopePublic, Name: stream}
	}
	rawSub, err := d.infra.Subscribe(ctx, request...)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)
	go d.pumpPublic(sub)
	return sub, nil
}

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
			messageType, err := d.table.Detect(raw.Data)
			if err != nil {
				sendError(sub.err, err)
				continue
			}
			if strings.TrimSpace(messageType) == "" {
				continue
			}
			res := d.hub.Dispatch(messageType, raw.Data)
			if res.err != nil {
				sendError(sub.err, res.err)
				continue
			}
			msg := *res.msg
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
				sendError(sub.err, err)
			}
			return
		}
	}
}

type PrivateDispatcher struct {
	infra      coretransport.StreamClient
	deps       WSDependencies
	table      *frameworkrouter.RoutingTable
	dispatcher *frameworkrouter.RouterDispatcher
	hub        *processorHub
}

func NewPrivateDispatcher(infra coretransport.StreamClient, deps WSDependencies, table *frameworkrouter.RoutingTable, dispatcher *frameworkrouter.RouterDispatcher, hub *processorHub) *PrivateDispatcher {
	return &PrivateDispatcher{infra: infra, deps: deps, table: table, dispatcher: dispatcher, hub: hub}
}

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

func (d *PrivateDispatcher) pumpPrivate(ctx context.Context, sub *wsSub, listenKey string) {
	defer close(sub.c)
	defer close(sub.err)

	rawCh := sub.raw.Messages()
	errCh := sub.raw.Errors()

	keepAliveTicker := time.NewTicker(privateKeepAliveInterval)
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
			messageType, err := d.table.Detect(raw.Data)
			if err != nil {
				sendError(sub.err, err)
				continue
			}
			if strings.TrimSpace(messageType) == "" {
				continue
			}
			res := d.hub.Dispatch(messageType, raw.Data)
			if res.err != nil {
				sendError(sub.err, res.err)
				continue
			}
			sub.c <- *res.msg
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				sendError(sub.err, err)
			}
			return
		}
	}
}

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

func sendError(ch chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case ch <- err:
	default:
	}
}
