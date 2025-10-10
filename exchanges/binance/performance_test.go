package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/core/streams/mocks"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/routing"
)

type perfDeps struct {
	listenKey      string
	keepAliveCount int32
	closeCount     int32
	keepAliveSignal chan struct{}
	closeSignal     chan struct{}
}

func (d *perfDeps) CanonicalSymbol(binanceSymbol string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if strings.Contains(upper, "-") || len(upper) < 6 {
		return upper, nil
	}
	return upper[:len(upper)/2] + "-" + upper[len(upper)/2:], nil
}

func (d *perfDeps) NativeSymbol(canonical string) (string, error) {
	return strings.ReplaceAll(strings.ToUpper(canonical), "-", ""), nil
}

func (d *perfDeps) NativeTopic(topic core.Topic) (string, error) {
	switch topic {
	case core.TopicTrade:
		return "trade", nil
	case core.TopicBookDelta:
		return "depth@100ms", nil
	case core.TopicUserOrder:
		return "order", nil
	case core.TopicUserBalance:
		return "balance", nil
	default:
		return "ticker", nil
	}
}

func (d *perfDeps) CreateListenKey(context.Context) (string, error) {
	if d.listenKey == "" {
		d.listenKey = "listen-key"
	}
	return d.listenKey, nil
}

func (d *perfDeps) KeepAliveListenKey(context.Context, string) error {
	atomic.AddInt32(&d.keepAliveCount, 1)
	if d.keepAliveSignal != nil {
		select {
		case d.keepAliveSignal <- struct{}{}:
		default:
		}
	}
	return nil
}

func (d *perfDeps) CloseListenKey(context.Context, string) error {
	atomic.AddInt32(&d.closeCount, 1)
	if d.closeSignal != nil {
		select {
		case d.closeSignal <- struct{}{}:
		default:
		}
	}
	return nil
}

func (d *perfDeps) BookDepthSnapshot(context.Context, string, int) (corestreams.BookEvent, int64, error) {
	return corestreams.BookEvent{}, 0, nil
}

type perfSubscription struct {
	messages chan coretransport.RawMessage
	errors   chan error
}

func newPerfSubscription(buffer int) (*perfSubscription, *mocks.StreamSubscription) {
	ps := &perfSubscription{
		messages: make(chan coretransport.RawMessage, buffer),
		errors:   make(chan error, 1),
	}
	return ps, &mocks.StreamSubscription{
		MessagesFunc: func() <-chan coretransport.RawMessage { return ps.messages },
		ErrorsFunc:   func() <-chan error { return ps.errors },
		CloseFunc: func() error {
			close(ps.messages)
			close(ps.errors)
			return nil
		},
	}
}

func TestBinanceRoutingThroughput(t *testing.T) {
	const (
		streamCount       = 5
		messagesPerStream = 200
		maxDuration       = 2 * time.Second
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps := &perfDeps{}
	var (
		mu       sync.Mutex
		subIndex int
		subs     []*perfSubscription
	)

	client := &mocks.StreamClient{}
	client.SubscribeFunc = func(_ context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		mu.Lock()
		defer mu.Unlock()
		ps, sub := newPerfSubscription(messagesPerStream + 16)
		subs = append(subs, ps)
		subIndex++
		return sub, nil
	}

	router := routing.NewWSRouter(client, deps)
	t.Cleanup(func() { require.NoError(t, router.Close()) })

	streams := make([]corestreams.Subscription, 0, streamCount)
	symbols := []string{"BTC-USDT", "ETH-USDT", "BNB-USDT", "XRP-USDT", "ADA-USDT"}
	for i := 0; i < streamCount; i++ {
		stream, err := router.SubscribePublic(ctx, routing.Trade(symbols[i]))
		require.NoError(t, err)
		require.NotNil(t, stream)
		streams = append(streams, stream)
	}

	var routed int32
	done := make(chan struct{}, streamCount)
	errCh := make(chan error, streamCount)
	for i, stream := range streams {
		go func(idx int, sub corestreams.Subscription) {
			defer func() {
				_ = sub.Close()
				done <- struct{}{}
			}()
			received := 0
			deadline := time.After(maxDuration)
			for received < messagesPerStream {
				select {
				case msg := <-sub.C():
					if msg.Route != corestreams.RouteTradeUpdate {
						errCh <- fmt.Errorf("unexpected route %v on stream %d", msg.Route, idx)
						return
					}
					if msg.Parsed == nil {
						errCh <- fmt.Errorf("nil parsed payload on stream %d", idx)
						return
					}
					received++
					atomic.AddInt32(&routed, 1)
				case err := <-sub.Err():
					if err != nil {
						errCh <- fmt.Errorf("stream %d reported error: %v", idx, err)
						return
					}
				case <-deadline:
					errCh <- fmt.Errorf("stream %d timed out waiting for messages", idx)
					return
				}
			}
		}(i, stream)
	}

	start := time.Now()
	for i, sub := range subs {
		dat := tradePayload(symbols[i%len(symbols)])
		for j := 0; j < messagesPerStream; j++ {
			sub.messages <- coretransport.RawMessage{Data: dat}
		}
	}

	for i := 0; i < streamCount; i++ {
		select {
		case <-done:
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(maxDuration):
			t.Fatalf("stream reader %d did not complete within %s", i, maxDuration)
		}
	}

	duration := time.Since(start)
	if duration > maxDuration {
		t.Fatalf("routing %d messages exceeded max duration %s (took %s)", streamCount*messagesPerStream, maxDuration, duration)
	}
	require.Equal(t, int32(streamCount*messagesPerStream), routed)
}

func BenchmarkBinanceFrameworkRouting(b *testing.B) {
	ctx := context.Background()
	deps := &perfDeps{}
	ps, sub := newPerfSubscription(1024)
	client := &mocks.StreamClient{
		SubscribeFunc: func(context.Context, ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
			return sub, nil
		},
	}

	router := routing.NewWSRouter(client, deps)
	b.Cleanup(func() { _ = router.Close() })

	stream, err := router.SubscribePublic(ctx, routing.Trade("BTC-USDT"))
	if err != nil {
		b.Fatalf("SubscribePublic: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		payload := tradePayload("BTC-USDT")
		for pb.Next() {
			ps.messages <- coretransport.RawMessage{Data: payload}
			select {
			case <-stream.C():
			default:
			}
		}
	})
}

func tradePayload(symbol string) []byte {
	base := strings.ToLower(strings.ReplaceAll(symbol, "-", ""))
	payload := map[string]any{
		"stream": base + "@trade",
		"data": map[string]any{
			"e": "trade",
			"s": strings.ReplaceAll(strings.ToUpper(symbol), "-", ""),
			"p": "100.01",
			"q": "0.50",
			"T": time.Now().UnixMilli(),
		},
	}
	data, _ := json.Marshal(payload)
	return data
}
