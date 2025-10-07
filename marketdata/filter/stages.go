package filter

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
)

func newSourceStage(
	adapter Adapter,
	req FilterRequest,
	withBooks bool,
	withTrades bool,
	withTickers bool,
) Stage {
	if adapter == nil {
		return NewStageFunc("source", func(ctx context.Context, input StageResult) StageResult {
			events := make(chan EventEnvelope)
			errors := make(chan error)
			close(events)
			close(errors)
			return StageResult{Events: events, Errors: errors}
		})
	}

	if !withBooks && !withTrades && !withTickers {
		return nil
	}

	return NewStageFunc("source", func(ctx context.Context, input StageResult) StageResult {
		events := make(chan EventEnvelope)
		errors := make(chan error, 16)

		var (
			bookSources   []BookSource
			tradeSources  []TradeSource
			tickerSources []TickerSource
			err           error
		)

		if withBooks {
			bookSources, err = adapter.BookSources(ctx, req.Symbols)
			if err != nil {
				go func() {
					defer close(events)
					defer close(errors)
					select {
					case errors <- err:
					case <-ctx.Done():
					}
				}()
				return StageResult{Events: events, Errors: errors}
			}
		}
		if withTrades {
			tradeSources, err = adapter.TradeSources(ctx, req.Symbols)
			if err != nil {
				go func() {
					defer close(events)
					defer close(errors)
					select {
					case errors <- err:
					case <-ctx.Done():
					}
				}()
				return StageResult{Events: events, Errors: errors}
			}
		}
		if withTickers {
			tickerSources, err = adapter.TickerSources(ctx, req.Symbols)
			if err != nil {
				go func() {
					defer close(events)
					defer close(errors)
					select {
					case errors <- err:
					case <-ctx.Done():
					}
				}()
				return StageResult{Events: events, Errors: errors}
			}
		}

		totalSources := len(bookSources) + len(tradeSources) + len(tickerSources)
		if totalSources == 0 {
			close(events)
			close(errors)
			return StageResult{Events: events, Errors: errors}
		}

		var wg sync.WaitGroup
		go func() {
			defer close(events)
			defer close(errors)

			for _, src := range bookSources {
				source := src
				wg.Add(1)
				go func() {
					defer wg.Done()
					eventCh := source.Events
					errCh := source.Errors
					for eventCh != nil || errCh != nil {
						select {
						case <-ctx.Done():
							return
						case evt, ok := <-eventCh:
							if !ok {
								eventCh = nil
								continue
							}
							book := evt
							envelope := EventEnvelope{
								Kind:      EventKindBook,
								Symbol:    source.Symbol,
								Timestamp: book.Time,
								Book:      &book,
							}
							select {
							case events <- envelope:
							case <-ctx.Done():
								return
							}
						case err, ok := <-errCh:
							if !ok {
								errCh = nil
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
			}

			for _, src := range tradeSources {
				source := src
				wg.Add(1)
				go func() {
					defer wg.Done()
					eventCh := source.Events
					errCh := source.Errors
					for eventCh != nil || errCh != nil {
						select {
						case <-ctx.Done():
							return
						case evt, ok := <-eventCh:
							if !ok {
								eventCh = nil
								continue
							}
							trade := evt
							envelope := EventEnvelope{
								Kind:      EventKindTrade,
								Symbol:    source.Symbol,
								Timestamp: trade.Time,
								Trade:     &trade,
							}
							select {
							case events <- envelope:
							case <-ctx.Done():
								return
							}
						case err, ok := <-errCh:
							if !ok {
								errCh = nil
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
			}

			for _, src := range tickerSources {
				source := src
				wg.Add(1)
				go func() {
					defer wg.Done()
					eventCh := source.Events
					errCh := source.Errors
					for eventCh != nil || errCh != nil {
						select {
						case <-ctx.Done():
							return
						case evt, ok := <-eventCh:
							if !ok {
								eventCh = nil
								continue
							}
							ticker := evt
							envelope := EventEnvelope{
								Kind:      EventKindTicker,
								Symbol:    source.Symbol,
								Timestamp: ticker.Time,
								Ticker:    &ticker,
							}
							select {
							case events <- envelope:
							case <-ctx.Done():
								return
							}
						case err, ok := <-errCh:
							if !ok {
								errCh = nil
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
			}

			wg.Wait()
		}()

		return StageResult{Events: events, Errors: errors}
	})
}

func newSamplingStage(interval time.Duration) Stage {
	return NewStageFunc("sampling", func(ctx context.Context, input StageResult) StageResult {
		if interval <= 0 || input.Events == nil {
			return input
		}

		events := make(chan EventEnvelope)
		errors := make(chan error, 1)
		eventSource := input.Events
		errorSource := input.Errors

		go func() {
			defer close(events)
			defer close(errors)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			var pending *EventEnvelope
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

		return StageResult{Events: events, Errors: errors}
	})
}

func newThrottleStage(interval time.Duration) Stage {
	if interval <= 0 {
		return nil
	}
	return NewStageFunc("throttle", func(ctx context.Context, input StageResult) StageResult {
		if input.Events == nil {
			return input
		}

		events := make(chan EventEnvelope)
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
					key := string(evt.Kind) + "|" + evt.Symbol
					t := evt.Timestamp
					if t.IsZero() {
						t = time.Now()
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

		return StageResult{Events: events, Errors: errors}
	})
}

func newNormalizeStage() Stage {
	return NewStageFunc("normalize", func(ctx context.Context, input StageResult) StageResult {
		if input.Events == nil {
			return input
		}
		events := make(chan EventEnvelope)
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
					if evt.Symbol == "" {
						continue
					}
					if evt.Timestamp.IsZero() {
						evt.Timestamp = time.Now()
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

		return StageResult{Events: events, Errors: errors}
	})
}

func newReliabilityStage() Stage {
	return NewStageFunc("reliability", func(ctx context.Context, input StageResult) StageResult {
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
		return StageResult{Events: events, Errors: errors}
	})
}

func newAggregationStage(cache *snapshotCache, depth int) Stage {
	if cache == nil && depth <= 0 {
		return nil
	}
	return NewStageFunc("aggregate", func(ctx context.Context, input StageResult) StageResult {
		if input.Events == nil && input.Errors == nil {
			return input
		}

		events := make(chan EventEnvelope)
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
					envelope := evt
					if depth > 0 && envelope.Kind == EventKindBook && envelope.Book != nil {
						trimmed := trimBookEvent(*envelope.Book, depth)
						envelope.Book = &trimmed
					}
					if cache != nil {
						cache.Update(envelope)
					}
					select {
					case events <- envelope:
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

		return StageResult{Events: events, Errors: errors}
	})
}

func newVWAPStage(enabled bool) Stage {
	if !enabled {
		return nil
	}
	return NewStageFunc("vwap", func(ctx context.Context, input StageResult) StageResult {
		if input.Events == nil && input.Errors == nil {
			return input
		}

		events := make(chan EventEnvelope)
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

					// Always forward the original event first.
					select {
					case events <- evt:
					case <-ctx.Done():
						return
					}

					if evt.Kind == EventKindTrade && evt.Trade != nil &&
						evt.Trade.Price != nil && evt.Trade.Quantity != nil && symbol != "" {
						acc := accumulators[symbol]
						if acc == nil {
							acc = &accum{
								amount: new(big.Rat),
								volume: new(big.Rat),
							}
							accumulators[symbol] = acc
						}

						amount := new(big.Rat).Mul(evt.Trade.Price, evt.Trade.Quantity)
						acc.amount.Add(acc.amount, amount)
						acc.volume.Add(acc.volume, evt.Trade.Quantity)
						acc.count++

						if acc.volume.Sign() != 0 {
							vwap := new(big.Rat).Quo(new(big.Rat).Set(acc.amount), acc.volume)
							analytics := &AnalyticsEvent{
								Symbol:     symbol,
								VWAP:       vwap,
								TradeCount: acc.count,
							}
							analyticsEnvelope := EventEnvelope{
								Kind:      EventKindVWAP,
								Symbol:    symbol,
								Timestamp: evt.Timestamp,
								Stats:     analytics,
							}
							select {
							case events <- analyticsEnvelope:
							case <-ctx.Done():
								return
							}
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

		return StageResult{Events: events, Errors: errors}
	})
}

func newObserverStage(observer Observer) Stage {
	if observer == nil {
		return nil
	}
	return NewStageFunc("observer", func(ctx context.Context, input StageResult) StageResult {
		events := make(chan EventEnvelope)
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
					if observer != nil {
						observer.OnEvent(evt)
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
					if err != nil && observer != nil {
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

		return StageResult{Events: events, Errors: errors}
	})
}

func dispatchStage() Stage {
	return NewStageFunc("dispatch", func(ctx context.Context, input StageResult) StageResult {
		result := StageResult{
			Events: input.Events,
			Errors: input.Errors,
		}
		if result.Events == nil {
			ch := make(chan EventEnvelope)
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
