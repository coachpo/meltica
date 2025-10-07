package pipeline

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
)

func newSamplingStage(interval time.Duration) PipelineStep {
	return NewPipelineStepFunc("sampling", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		if interval <= 0 || input.Events == nil {
			return input
		}

		events := make(chan ClientEvent)
		errors := make(chan error, 1)
		eventSource := input.Events
		errorSource := input.Errors

		go func() {
			defer close(events)
			defer close(errors)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			var pending *ClientEvent
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-eventSource:
					if !ok {
						if pending != nil {
							select {
							case events <- *pending:
							case <-ctx.Done():
							}
						}
						return
					}
					copy := evt
					pending = &copy
				case <-ticker.C:
					if pending != nil {
						select {
						case events <- *pending:
						case <-ctx.Done():
							return
						}
						pending = nil
					}
				case err, ok := <-errorSource:
					if !ok {
						continue
					}
					if err != nil {
						select {
						case errors <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()

		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func newThrottleStage(interval time.Duration) PipelineStep {
	if interval <= 0 {
		return nil
	}
	return NewPipelineStepFunc("throttle", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		if input.Events == nil {
			return input
		}

		events := make(chan ClientEvent)
		errors := make(chan error, 1)
		eventSource := input.Events
		errorSource := input.Errors
		lastEmit := make(map[string]time.Time)

		go func() {
			defer close(events)
			defer close(errors)
			for eventSource != nil || errorSource != nil {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-eventSource:
					if !ok {
						eventSource = nil
						continue
					}
					key := payloadIdentity(evt.Payload) + "|" + evt.Symbol
					t := evt.At
					if t.IsZero() {
						t = time.Now()
						evt.At = t
					}
					if prev, ok := lastEmit[key]; ok {
						if t.Sub(prev) < interval {
							continue
						}
					}
					lastEmit[key] = t
					select {
					case events <- evt:
					case <-ctx.Done():
						return
					}
				case err, ok := <-errorSource:
					if !ok {
						errorSource = nil
						continue
					}
					if err != nil {
						select {
						case errors <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()

		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func newNormalizeStage() PipelineStep {
	return NewPipelineStepFunc("normalize", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		if input.Events == nil {
			return input
		}
		events := make(chan ClientEvent, 128)
		errors := make(chan error, 16)
		eventSource := input.Events
		errorSource := input.Errors

		go func() {
			defer close(events)
			defer close(errors)
			for eventSource != nil || errorSource != nil {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-eventSource:
					if !ok {
						eventSource = nil
						continue
					}
					if evt.Symbol == "" {
						if _, ok := evt.Payload.(RestResponsePayload); !ok {
							continue
						}
					}
					if evt.At.IsZero() {
						evt.At = time.Now()
					}
					select {
					case events <- evt:
					case <-ctx.Done():
						return
					}
				case err, ok := <-errorSource:
					if !ok {
						errorSource = nil
						continue
					}
					if err != nil {
						select {
						case errors <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()

		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func newReliabilityStage() PipelineStep {
	return NewPipelineStepFunc("reliability", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		if input.Errors == nil {
			return input
		}
		errors := make(chan error, 4)
		events := input.Events
		go func() {
			defer close(errors)
			for {
				select {
				case <-ctx.Done():
					return
				case err, ok := <-input.Errors:
					if !ok {
						return
					}
					if err != nil {
						select {
						case errors <- fmt.Errorf("filter pipeline: %w", err):
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()
		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func newAggregationStage(cache *snapshotCache, depth int) PipelineStep {
	if cache == nil && depth <= 0 {
		return nil
	}
	return NewPipelineStepFunc("aggregate", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		if input.Events == nil && input.Errors == nil {
			return input
		}

		events := make(chan ClientEvent)
		errors := make(chan error, 1)
		eventSource := input.Events
		errorSource := input.Errors

		go func() {
			defer close(events)
			defer close(errors)
			for eventSource != nil || errorSource != nil {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-eventSource:
					if !ok {
						eventSource = nil
						continue
					}
					if depth > 0 {
						if bookPayload, ok := evt.Payload.(BookPayload); ok && bookPayload.Book != nil {
							trimmed := trimBookEvent(*bookPayload.Book, depth)
							bookPayload.Book = &trimmed
							evt.Payload = bookPayload
						}
					}
					if cache != nil {
						cache.Update(evt)
					}
					select {
					case events <- evt:
					case <-ctx.Done():
						return
					}
				case err, ok := <-errorSource:
					if !ok {
						errorSource = nil
						continue
					}
					if err != nil {
						select {
						case errors <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()

		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func newVWAPStage(enabled bool) PipelineStep {
	if !enabled {
		return nil
	}
	return NewPipelineStepFunc("vwap", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		if input.Events == nil && input.Errors == nil {
			return input
		}

		events := make(chan ClientEvent)
		errors := make(chan error, 1)
		eventSource := input.Events
		errorSource := input.Errors

		type accum struct {
			amount *big.Rat
			volume *big.Rat
			count  int64
		}
		accumulators := make(map[string]*accum)

		go func() {
			defer close(events)
			defer close(errors)

			for eventSource != nil || errorSource != nil {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-eventSource:
					if !ok {
						eventSource = nil
						continue
					}

					symbol := normalizeSymbol(evt.Symbol)

					select {
					case events <- evt:
					case <-ctx.Done():
						return
					}

					tradePayload, ok := evt.Payload.(TradePayload)
					if !ok || tradePayload.Trade == nil {
						continue
					}
					if tradePayload.Trade.Price == nil || tradePayload.Trade.Quantity == nil || symbol == "" {
						continue
					}

					acc := accumulators[symbol]
					if acc == nil {
						acc = &accum{
							amount: new(big.Rat),
							volume: new(big.Rat),
						}
						accumulators[symbol] = acc
					}

					amount := new(big.Rat).Mul(tradePayload.Trade.Price, tradePayload.Trade.Quantity)
					acc.amount.Add(acc.amount, amount)
					acc.volume.Add(acc.volume, tradePayload.Trade.Quantity)
					acc.count++

					if acc.volume.Sign() != 0 {
						vwap := new(big.Rat).Quo(new(big.Rat).Set(acc.amount), acc.volume)
						analytics := &AnalyticsEvent{
							Symbol:     symbol,
							VWAP:       vwap,
							TradeCount: acc.count,
						}
						analyticsEvent := ClientEvent{
							Channel: evt.Channel,
							Symbol:  symbol,
							At:      evt.At,
							Payload: AnalyticsPayload{Analytics: analytics},
						}
						select {
						case events <- analyticsEvent:
						case <-ctx.Done():
							return
						}
					}
				case err, ok := <-errorSource:
					if !ok {
						errorSource = nil
						continue
					}
					if err != nil {
						select {
						case errors <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()

		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func newObserverStage(observer Observer) PipelineStep {
	if observer == nil {
		return nil
	}
	return NewPipelineStepFunc("observer", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		events := make(chan ClientEvent)
		errors := make(chan error, 1)
		eventSource := input.Events
		errorSource := input.Errors

		go func() {
			defer close(events)
			defer close(errors)
			for eventSource != nil || errorSource != nil {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-eventSource:
					if !ok {
						eventSource = nil
						continue
					}
					observer.OnEvent(evt)
					select {
					case events <- evt:
					case <-ctx.Done():
						return
					}
				case err, ok := <-errorSource:
					if !ok {
						errorSource = nil
						continue
					}
					if err != nil {
						observer.OnError(err)
					}
					select {
					case errors <- err:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func dispatchStage() PipelineStep {
	return NewPipelineStepFunc("dispatch", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		result := PipelineStepResult{
			Events: input.Events,
			Errors: input.Errors,
		}
		if result.Events == nil {
			ch := make(chan ClientEvent)
			close(ch)
			result.Events = ch
		}
		if result.Errors == nil {
			ch := make(chan error)
			close(ch)
			result.Errors = ch
		}
		return result
	})
}

func trimBookEvent(evt corestreams.BookEvent, depth int) corestreams.BookEvent {
	result := corestreams.BookEvent{
		Symbol: evt.Symbol,
		Time:   evt.Time,
	}
	result.Bids = cloneLevels(evt.Bids, depth)
	result.Asks = cloneLevels(evt.Asks, depth)
	return result
}

func cloneLevels(levels []core.BookDepthLevel, depth int) []core.BookDepthLevel {
	if len(levels) == 0 {
		return nil
	}
	if depth <= 0 || depth > len(levels) {
		copied := make([]core.BookDepthLevel, len(levels))
		copy(copied, levels)
		return copied
	}
	copied := make([]core.BookDepthLevel, depth)
	copy(copied, levels[:depth])
	return copied
}

func payloadIdentity(payload TransportPayload) string {
	switch payload.(type) {
	case BookPayload:
		return "book"
	case TradePayload:
		return "trade"
	case TickerPayload:
		return "ticker"
	case AnalyticsPayload:
		return "analytics"
	case AccountPayload:
		return "account"
	case OrderPayload:
		return "order"
	case RestResponsePayload:
		return "rest_response"
	case UnknownPayload:
		return "unknown"
	default:
		return "payload"
	}
}
