package pipeline

import (
	"context"
	"fmt"
	"sync"

	corestreams "github.com/coachpo/meltica/core/streams"
)

// multiSourceStage handles mixed channel sources including public feeds, private streams, and REST requests.
func multiSourceStage(
	adapter Adapter,
	req PipelineRequest,
	auth *AuthContext,
) PipelineStep {
	if adapter == nil {
		return NewPipelineStepFunc("source", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
			events := make(chan ClientEvent)
			errors := make(chan error)
			close(events)
			close(errors)
			return PipelineStepResult{Events: events, Errors: errors}
		})
	}

	return NewPipelineStepFunc("multi_source", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		events := make(chan ClientEvent, 128)
		errors := make(chan error, 16)

		var wg sync.WaitGroup

		// Start public feed sources
		if req.Feeds.Books || req.Feeds.Trades || req.Feeds.Tickers {
			startPublicSources(ctx, adapter, req, &wg, events, errors)
		}

		// Start private stream sources
		if req.EnablePrivate && auth != nil {
			startPrivateSources(ctx, adapter, auth, &wg, events, errors)
		}

		// Execute REST requests
		if len(req.RESTRequests) > 0 {
			startRESTRequests(ctx, adapter, req.RESTRequests, &wg, events, errors)
		}

		// For mixed channel sources, we don't close events/errors channels immediately
		// since private streams and REST requests may be long-running.
		// The channels will be closed when the context is cancelled.
		go func() {
			wg.Wait()
			// Only close channels if there are no long-running sources
			// For now, we rely on context cancellation to close channels
		}()

		return PipelineStepResult{Events: events, Errors: errors}
	})
}

func startPublicSources(
	ctx context.Context,
	adapter Adapter,
	req PipelineRequest,
	wg *sync.WaitGroup,
	events chan<- ClientEvent,
	errors chan<- error,
) {
	capabilities := adapter.Capabilities()

	if req.Feeds.Books && capabilities.Books {
		bookSources, err := adapter.BookSources(ctx, req.Symbols)
		if err != nil {
			select {
			case errors <- fmt.Errorf("book sources: %w", err):
			case <-ctx.Done():
			}
		} else {
			for _, src := range bookSources {
				startBookForwarder(ctx, src.Symbol, src.Events, src.Errors, wg, events, errors)
			}
		}
	}

	if req.Feeds.Trades && capabilities.Trades {
		tradeSources, err := adapter.TradeSources(ctx, req.Symbols)
		if err != nil {
			select {
			case errors <- fmt.Errorf("trade sources: %w", err):
			case <-ctx.Done():
			}
		} else {
			for _, src := range tradeSources {
				startTradeForwarder(ctx, src.Symbol, src.Events, src.Errors, wg, events, errors)
			}
		}
	}

	if req.Feeds.Tickers && capabilities.Tickers {
		tickerSources, err := adapter.TickerSources(ctx, req.Symbols)
		if err != nil {
			select {
			case errors <- fmt.Errorf("ticker sources: %w", err):
			case <-ctx.Done():
			}
		} else {
			for _, src := range tickerSources {
				startTickerForwarder(ctx, src.Symbol, src.Events, src.Errors, wg, events, errors)
			}
		}
	}
}

func startPrivateSources(
	ctx context.Context,
	adapter Adapter,
	auth *AuthContext,
	wg *sync.WaitGroup,
	events chan<- ClientEvent,
	errors chan<- error,
) {
	capabilities := adapter.Capabilities()
	if !capabilities.PrivateStreams {
		select {
		case errors <- fmt.Errorf("private streams not supported"):
		case <-ctx.Done():
		}
		return
	}

	// Initialize private session
	if err := adapter.InitPrivateSession(ctx, auth); err != nil {
		select {
		case errors <- fmt.Errorf("init private session: %w", err):
		case <-ctx.Done():
		}
		return
	}

	privateSources, err := adapter.PrivateSources(ctx, auth)
	if err != nil {
		select {
		case errors <- fmt.Errorf("private sources: %w", err):
		case <-ctx.Done():
		}
		return
	}

	for _, src := range privateSources {
		wg.Add(1)
		go func(source PrivateSource) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-source.Events:
					if !ok {
						return
					}
					select {
					case events <- clientEventFromPipeline(evt):
					case <-ctx.Done():
						return
					}
				case err, ok := <-source.Errors:
					if !ok {
						return
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
		}(src)
	}
}

func startRESTRequests(
	ctx context.Context,
	adapter Adapter,
	requests []InteractionRequest,
	wg *sync.WaitGroup,
	events chan<- ClientEvent,
	errors chan<- error,
) {
	capabilities := adapter.Capabilities()
	if !capabilities.RESTEndpoints {
		select {
		case errors <- fmt.Errorf("REST endpoints not supported"):
		case <-ctx.Done():
		}
		return
	}

	for _, restReq := range requests {
		wg.Add(1)
		go func(req InteractionRequest) {
			defer wg.Done()
			restEvents, restErrors, err := adapter.ExecuteREST(ctx, req)
			if err != nil {
				select {
				case errors <- fmt.Errorf("REST request %s %s: %w", req.Method, req.Path, err):
				case <-ctx.Done():
				}
				return
			}

			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-restEvents:
					if !ok {
						return
					}
					select {
					case events <- clientEventFromPipeline(evt):
					case <-ctx.Done():
						return
					}
				case err, ok := <-restErrors:
					if !ok {
						return
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
		}(restReq)
	}
}

func startBookForwarder(
	ctx context.Context,
	symbol string,
	eventCh <-chan corestreams.BookEvent,
	errorCh <-chan error,
	wg *sync.WaitGroup,
	events chan<- ClientEvent,
	errors chan<- error,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for eventCh != nil || errorCh != nil {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-eventCh:
				if !ok {
					eventCh = nil
					continue
				}

				payload := BookPayload{Book: &evt}
				event := ClientEvent{
					Channel: ChannelPublicWS,
					Symbol:  symbol,
					At:      evt.Time,
					Payload: payload,
				}

				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			case err, ok := <-errorCh:
				if !ok {
					errorCh = nil
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

func startTradeForwarder(
	ctx context.Context,
	symbol string,
	eventCh <-chan corestreams.TradeEvent,
	errorCh <-chan error,
	wg *sync.WaitGroup,
	events chan<- ClientEvent,
	errors chan<- error,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for eventCh != nil || errorCh != nil {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-eventCh:
				if !ok {
					eventCh = nil
					continue
				}

				payload := TradePayload{Trade: &evt}
				event := ClientEvent{
					Channel: ChannelPublicWS,
					Symbol:  symbol,
					At:      evt.Time,
					Payload: payload,
				}

				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			case err, ok := <-errorCh:
				if !ok {
					errorCh = nil
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

func startTickerForwarder(
	ctx context.Context,
	symbol string,
	eventCh <-chan corestreams.TickerEvent,
	errorCh <-chan error,
	wg *sync.WaitGroup,
	events chan<- ClientEvent,
	errors chan<- error,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for eventCh != nil || errorCh != nil {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-eventCh:
				if !ok {
					eventCh = nil
					continue
				}

				payload := TickerPayload{Ticker: &evt}
				event := ClientEvent{
					Channel: ChannelPublicWS,
					Symbol:  symbol,
					At:      evt.Time,
					Payload: payload,
				}

				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			case err, ok := <-errorCh:
				if !ok {
					errorCh = nil
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

func clientEventFromPipeline(evt Event) ClientEvent {
	channel := ChannelType(evt.Transport.String())
	// Preserve empty channel for unknown transports to allow later normalization.
	if evt.Transport == TransportUnknown {
		channel = ChannelHybrid
	}
	return ClientEvent{
		Channel:       channel,
		Symbol:        evt.Symbol,
		At:            evt.At,
		Payload:       evt.Payload,
		CorrelationID: evt.CorrelationID,
		Metadata:      evt.Metadata,
	}
}
