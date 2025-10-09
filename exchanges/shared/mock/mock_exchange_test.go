package mock

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/pipeline"
)

func TestSandboxExchangeDefaults(t *testing.T) {
	exchange, adapter := NewExchange("sandbox")
	if !exchange.Capabilities().Has(core.CapabilityMarketTrades) {
		t.Fatalf("expected default exchange to expose market trades capability")
	}
	caps := adapter.Capabilities()
	if !caps.Trades || !caps.Books || !caps.Tickers {
		t.Fatalf("expected default pipeline capabilities to be enabled: %+v", caps)
	}
}

func TestPublishTradeDeliversEvent(t *testing.T) {
	_, adapter := NewExchange("sandbox")
	facade := pipeline.NewInteractionFacade(adapter, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := facade.SubscribePublic(ctx, []string{"BTC-USDT"}, pipeline.WithTrades())
	if err != nil {
		t.Fatalf("SubscribePublic returned error: %v", err)
	}
	defer stream.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		adapter.PublishTrade("BTC-USDT", corestreams.TradeEvent{
			Symbol:   "BTC-USDT",
			Price:    big.NewRat(50000, 1),
			Quantity: big.NewRat(1, 2),
			Time:     time.Now(),
		})
	}()

	select {
	case evt := <-stream.Events:
		payload, ok := evt.Payload.(pipeline.TradePayload)
		if !ok || payload.Trade == nil {
			t.Fatalf("expected trade payload, got %#v", evt.Payload)
		}
		if evt.Channel != pipeline.ChannelPublicWS {
			t.Fatalf("expected public channel, got %s", evt.Channel)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for trade event")
	}
}

func TestPublishPrivateDeliversEvent(t *testing.T) {
	_, adapter := NewExchange("sandbox")
	facade := pipeline.NewInteractionFacade(adapter, &pipeline.AuthContext{APIKey: "demo"})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := facade.SubscribePrivate(ctx)
	if err != nil {
		t.Fatalf("SubscribePrivate returned error: %v", err)
	}
	defer stream.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		adapter.PublishPrivate("orders", pipeline.Event{
			Symbol: "BTC-USDT",
			Payload: pipeline.OrderPayload{Order: &pipeline.OrderEvent{
				Symbol:   "BTC-USDT",
				OrderID:  "abc",
				Quantity: big.NewRat(1, 1),
			}},
		})
	}()

	select {
	case evt := <-stream.Events:
		payload, ok := evt.Payload.(pipeline.OrderPayload)
		if !ok || payload.Order == nil {
			t.Fatalf("expected order payload, got %#v", evt.Payload)
		}
		if evt.Channel != pipeline.ChannelPrivateWS {
			t.Fatalf("expected private channel, got %s", evt.Channel)
		}
		if evt.Metadata["source.feed"] != "orders" {
			t.Fatalf("expected metadata source.feed orders, got %v", evt.Metadata)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for private event")
	}
}

func TestSetRESTResponseOverridesDefault(t *testing.T) {
	_, adapter := NewExchange("sandbox")
	facade := pipeline.NewInteractionFacade(adapter, nil)

	adapter.SetRESTResponse("req-1", pipeline.Event{
		Payload: pipeline.RestResponsePayload{
			Response: &pipeline.RestResponse{RequestID: "req-1", StatusCode: 299},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := pipeline.InteractionRequest{Method: "GET", Path: "/api/v3/account", CorrelationID: "req-1"}
	stream, err := facade.FetchREST(ctx, []pipeline.InteractionRequest{req})
	if err != nil {
		t.Fatalf("FetchREST returned error: %v", err)
	}
	defer stream.Close()

	select {
	case evt := <-stream.Events:
		payload, ok := evt.Payload.(pipeline.RestResponsePayload)
		if !ok || payload.Response == nil {
			t.Fatalf("expected rest response payload, got %#v", evt.Payload)
		}
		if payload.Response.StatusCode != 299 {
			t.Fatalf("expected status code 299, got %d", payload.Response.StatusCode)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for REST response")
	}
}
