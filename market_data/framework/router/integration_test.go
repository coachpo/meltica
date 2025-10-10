package router

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutingPipelineIntegration(t *testing.T) {
	rt := NewRoutingTable()
	defaultProc := newMockProcessor("default")
	require.NoError(t, rt.SetDefault(defaultProc))

	tradeProc := newMockProcessor("trade")
	require.NoError(t, rt.Register(&MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "trade",
		}},
		ProcessorRef: "trade",
	}, tradeProc))

	accountProc := newMockProcessor("account")
	require.NoError(t, rt.Register(&MessageTypeDescriptor{
		ID: "account",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "account",
		}},
		ProcessorRef: "account",
	}, accountProc))

	failingProc := newMockProcessor("funding")
	failingProc.initErr = errors.New("init failure")
	require.NoError(t, rt.Register(&MessageTypeDescriptor{
		ID: "funding",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "funding",
		}},
		ProcessorRef: "funding",
	}, failingProc))

	dispatcher := NewRouterDispatcher(context.Background(), rt.metrics)
	defer dispatcher.Shutdown()

	tradeInbox := dispatcher.Bind("trade", rt.Lookup("trade"))
	accountInbox := dispatcher.Bind("account", rt.Lookup("account"))
	defaultInbox := dispatcher.BindDefault(rt.defaultProc)

	tradeTotal := 5
	accountTotal := 5

	tradeDone := make(chan error, 1)
	go func() {
		for i := 0; i < tradeTotal; i++ {
			raw := <-tradeInbox
			if _, err := tradeProc.Process(context.Background(), raw); err != nil {
				tradeDone <- fmt.Errorf("trade processing: %w", err)
				return
			}
		}
		tradeDone <- nil
	}()

	accountDone := make(chan error, 1)
	go func() {
		for i := 0; i < accountTotal; i++ {
			raw := <-accountInbox
			if _, err := accountProc.Process(context.Background(), raw); err != nil {
				accountDone <- fmt.Errorf("account processing: %w", err)
				return
			}
		}
		accountDone <- nil
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, tradeTotal+accountTotal)
	for i := 0; i < tradeTotal; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw := []byte(fmt.Sprintf(`{"type":"trade","seq":%d}`, i))
			msgType, err := rt.Detect(raw)
			if err != nil {
				errCh <- fmt.Errorf("detect trade: %w", err)
				return
			}
			if msgType != "trade" {
				errCh <- fmt.Errorf("expected trade message type, got %s", msgType)
				return
			}
			if err := dispatcher.Dispatch(msgType, raw); err != nil {
				errCh <- fmt.Errorf("dispatch trade: %w", err)
			}
		}()
	}

	for i := 0; i < accountTotal; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw := []byte(fmt.Sprintf(`{"type":"account","seq":%d}`, i))
			msgType, err := rt.Detect(raw)
			if err != nil {
				errCh <- fmt.Errorf("detect account: %w", err)
				return
			}
			if msgType != "account" {
				errCh <- fmt.Errorf("expected account message type, got %s", msgType)
				return
			}
			if err := dispatcher.Dispatch(msgType, raw); err != nil {
				errCh <- fmt.Errorf("dispatch account: %w", err)
			}
		}()
	}

	wg.Wait()
	require.NoError(t, <-tradeDone)
	require.NoError(t, <-accountDone)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	default:
	}
	require.Equal(t, tradeTotal, tradeProc.Calls())
	require.Equal(t, accountTotal, accountProc.Calls())

	unknown := []byte(`{"type":"unknown","payload":1}`)
	_, detectErr := rt.Detect(unknown)
	require.Error(t, detectErr)
	defaultReceived := make(chan []byte, 1)
	go func() {
		raw := <-defaultInbox
		_, _ = defaultProc.Process(context.Background(), raw)
		defaultReceived <- raw
	}()
	require.NoError(t, dispatcher.Dispatch("", unknown))
	require.Equal(t, unknown, <-defaultReceived)

	funding := []byte(`{"type":"funding","rate":"0.01"}`)
	msgType, err := rt.Detect(funding)
	require.NoError(t, err)
	require.Equal(t, "funding", msgType)
	fundingReg := rt.Lookup(msgType)
	require.Equal(t, ProcessorStatusUnavailable, fundingReg.Status)

	fallback := make(chan []byte, 1)
	go func() {
		raw := <-defaultInbox
		_, _ = defaultProc.Process(context.Background(), raw)
		fallback <- raw
	}()
	require.NoError(t, dispatcher.Dispatch("", funding))
	require.Equal(t, funding, <-fallback)

	metrics := rt.GetMetrics()
	require.Equal(t, uint64(1), metrics.ProcessorInitFailures)
	require.GreaterOrEqual(t, metrics.RoutingErrors, uint64(1))
	require.Equal(t, uint64(tradeTotal), metrics.MessagesRouted["trade"])
	require.Equal(t, uint64(accountTotal), metrics.MessagesRouted["account"])
	require.Equal(t, uint64(1), metrics.MessagesRouted["funding"])
	require.Equal(t, 2, defaultProc.Calls())
}
