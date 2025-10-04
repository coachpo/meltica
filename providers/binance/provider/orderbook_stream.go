package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance/routing"
)

func (p *Provider) OrderBookSnapshots(ctx context.Context, symbol string) (<-chan coreprovider.BookEvent, <-chan error, error) {
	canonicalSymbol, err := p.canonicalizeSymbol(symbol)
	if err != nil {
		return nil, nil, err
	}

	sub, err := p.wsRouter.SubscribePublic(ctx, corews.BookTopic(canonicalSymbol))
	if err != nil {
		return nil, nil, fmt.Errorf("binance: subscribe depth stream: %w", err)
	}

	if err := p.initializeWithRetries(ctx, p.wsRouter, canonicalSymbol); err != nil {
		_ = sub.Close()
		return nil, nil, err
	}

	events := make(chan coreprovider.BookEvent, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)
		defer sub.Close()

		publishSnapshot := func() bool {
			snapshot, ok := p.wsRouter.OrderBookSnapshot(canonicalSymbol)
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

func (p *Provider) initializeWithRetries(ctx context.Context, handler *routing.WSRouter, symbol string) error {
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
