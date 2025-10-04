package exchange

import (
	"context"
	"strings"
	"time"

	coreexchange "github.com/coachpo/meltica/core/exchange"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/exchanges/binance/routing"
)

func (x *Exchange) OrderBookSnapshots(ctx context.Context, symbol string) (<-chan coreexchange.BookEvent, <-chan error, error) {
	canonicalSymbol, err := x.canonicalizeSymbol(symbol)
	if err != nil {
		return nil, nil, err
	}

	sub, err := x.wsRouter.SubscribePublic(ctx, corews.BookTopic(canonicalSymbol))
	if err != nil {
		return nil, nil, internal.WrapExchange(err, "subscribe depth stream")
	}

	if err := x.initializeWithRetries(ctx, x.wsRouter, canonicalSymbol); err != nil {
		_ = sub.Close()
		return nil, nil, err
	}

	events := make(chan coreexchange.BookEvent, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)
		defer sub.Close()

		publishSnapshot := func() bool {
			snapshot, ok := x.wsRouter.OrderBookSnapshot(canonicalSymbol)
			if !ok {
				return true
			}
			select {
			case events <- snapshot:
				return true
			case <-ctx.Done():
				return false
			}
		}

		if !publishSnapshot() {
			return
		}

		refreshTicker := time.NewTicker(500 * time.Millisecond)
		defer refreshTicker.Stop()

		routed := sub.C()
		routeErrs := sub.Err()

		for {
			select {
			case <-ctx.Done():
				return
			case <-refreshTicker.C:
				if !publishSnapshot() {
					return
				}
			case _, ok := <-routed:
				if !ok {
					publishSnapshot()
					return
				}
				if !publishSnapshot() {
					return
				}
			case err, ok := <-routeErrs:
				if !ok {
					return
				}
				if err != nil {
					select {
					case errs <- err:
					default:
					}
				}
				return
			}
		}
	}()

	return events, errs, nil
}

func (x *Exchange) canonicalizeSymbol(symbol string) (string, error) {
	s := strings.TrimSpace(symbol)
	if s == "" {
		return "", internal.Invalid("empty symbol")
	}
	s = strings.ToUpper(s)
	if strings.Contains(s, "-") {
		return s, nil
	}
	canonical, err := x.CanonicalSymbol(s)
	if err != nil {
		return "", err
	}
	if canonical == "" {
		return "", internal.Invalid("unsupported symbol %s", symbol)
	}
	return canonical, nil
}

func (x *Exchange) initializeWithRetries(ctx context.Context, handler *routing.WSRouter, symbol string) error {
	const maxRetries = 5
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return internal.WrapNetwork(ctx.Err(), "order book initialization cancelled")
		default:
		}
		initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := handler.InitializeOrderBook(initCtx, symbol)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		backoff := time.Duration(attempt+1) * time.Second
		select {
		case <-ctx.Done():
			return internal.WrapNetwork(ctx.Err(), "order book initialization cancelled")
		case <-time.After(backoff):
		}
	}
	if lastErr == nil {
		lastErr = internal.Exchange("failed to initialize order book")
	}
	return lastErr
}
