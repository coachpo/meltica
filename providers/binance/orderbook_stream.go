package binance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance/ws"
)

// OrderBookSnapshots returns a channel of synchronized Binance order book snapshots
// for the given symbol. The symbol must be provided in canonical form (e.g., BTC-USDT)
// or in Binance native form (e.g., BTCUSDT). The function handles snapshot initialization,
// incremental stream subscription, and automatic recovery via the provider's internal
// order book manager.
func (p *Provider) OrderBookSnapshots(ctx context.Context, symbol string) (<-chan corews.BookEvent, <-chan error, error) {
	canonicalSymbol, err := p.canonicalizeSymbol(symbol)
	if err != nil {
		return nil, nil, err
	}

	wsIface := p.WS()
	handler, ok := wsIface.(*ws.WS)
	if !ok {
		return nil, nil, errors.New("binance: unexpected websocket handler type")
	}

	sub, err := handler.SubscribePublic(ctx, corews.BookTopic(canonicalSymbol))
	if err != nil {
		return nil, nil, fmt.Errorf("binance: subscribe depth stream: %w", err)
	}

	if err := p.initializeWithRetries(ctx, handler, canonicalSymbol); err != nil {
		sub.Close()
		return nil, nil, err
	}

	events := make(chan corews.BookEvent, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)
		defer sub.Close()

		publishSnapshot := func() bool {
			snapshot, ok := handler.OrderBookSnapshot(canonicalSymbol)
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

		// Emit initial snapshot (if ready).
		if !publishSnapshot() {
			return
		}

		refreshTicker := time.NewTicker(500 * time.Millisecond)
		defer refreshTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-refreshTicker.C:
				if !publishSnapshot() {
					return
				}
			case _, ok := <-sub.C():
				if !ok {
					publishSnapshot()
					return
				}
				if !publishSnapshot() {
					return
				}
			case err, ok := <-sub.Err():
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

func (p *Provider) canonicalizeSymbol(symbol string) (string, error) {
	s := strings.TrimSpace(symbol)
	if s == "" {
		return "", errors.New("binance: empty symbol")
	}
	s = strings.ToUpper(s)
	if strings.Contains(s, "-") {
		return s, nil
	}

	var canonical string
	var panicErr any
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = r
			}
		}()
		canonical = p.CanonicalSymbol(s)
	}()
	if panicErr != nil {
		return "", fmt.Errorf("binance: unsupported symbol %s", symbol)
	}
	if canonical == "" {
		return "", fmt.Errorf("binance: unsupported symbol %s", symbol)
	}
	return canonical, nil
}

func (p *Provider) initializeWithRetries(ctx context.Context, handler *ws.WS, symbol string) error {
	const maxRetries = 5
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	if lastErr == nil {
		lastErr = errors.New("binance: failed to initialize order book")
	}
	return lastErr
}
