package pipeline

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/layers"
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

func newSymbolGuardStage(exchange core.ExchangeName) PipelineStep {
	if exchange == "" {
		return nil
	}
	guard := core.NewSymbolGuard(exchange, func(alert core.SymbolDriftAlert) {
		log.Printf("symbol guard drift exchange=%s feed=%s canonical=%s expected=%s observed=%s remediation=%q",
			alert.Exchange, alert.Feed, alert.Canonical, alert.ExpectedSymbol, alert.ObservedSymbol, alert.Remediation)
	})
	reported := make(map[string]struct{})
	return NewPipelineStepFunc("symbol_guard", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
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
				if eventSource == nil {
					if errorSource == nil {
						break
					}
					select {
					case <-ctx.Done():
						return
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
					default:
						errorSource = nil
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-eventSource:
					if !ok {
						eventSource = nil
						continue
					}
					feedID, snapshot := guardSnapshotFromEvent(evt)
					if feedID != "" && snapshot != nil {
						if err := guard.Check(feedID, snapshot); err != nil {
							if _, seen := reported[feedID]; !seen {
								reported[feedID] = struct{}{}
								select {
								case errors <- err:
								case <-ctx.Done():
									return
								}
							}
							continue
						}
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

func newNormalizeStage() PipelineStep {
	return NewPipelineStepFunc("normalize", func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		if input.Events == nil {
			return input
		}
		events := make(chan ClientEvent, 128)
		errors := make(chan error, 16)
		eventSource := input.Events
		errorSource := input.Errors
		lastSequence := make(map[string]uint64)

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
					if feed, sym, seq, seqOK := extractSourceSequence(evt.Metadata); seqOK {
						key := feed + "|" + sym
						if last, seen := lastSequence[key]; seen && seq <= last {
							continue
						}
						lastSequence[key] = seq
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

func newLayerFilterStage(filter layers.Filter) PipelineStep {
	name := "layers_filter"
	if filter != nil && filter.Name() != "" {
		name = filter.Name()
	}
	return NewPipelineStepFunc(name, func(ctx context.Context, input PipelineStepResult) PipelineStepResult {
		layerInputs := clientEventsToLayerEvents(ctx, input.Events)
		filtered, err := filter.Apply(ctx, layerInputs)
		events := make(chan ClientEvent)
		errors := make(chan error, 1)

		if err != nil {
			go func() {
				defer close(events)
				defer close(errors)
				select {
				case errors <- err:
				case <-ctx.Done():
				}
				forwardErrors(ctx, input.Errors, errors)
			}()
			return PipelineStepResult{Events: events, Errors: errors}
		}

		go func() {
			defer close(events)
			defer close(errors)

			layerOutputs := layerEventsToClientEvents(ctx, filtered)
			for layerOutputs != nil || input.Errors != nil {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-layerOutputs:
					if !ok {
						layerOutputs = nil
						continue
					}
					select {
					case events <- evt:
					case <-ctx.Done():
						return
					}
				case err, ok := <-input.Errors:
					if !ok {
						input.Errors = nil
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

func clientEventsToLayerEvents(ctx context.Context, events <-chan ClientEvent) <-chan layers.Event {
	if events == nil {
		return nil
	}
	out := make(chan layers.Event)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				select {
				case out <- clientEventToLayerEvent(evt):
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func layerEventsToClientEvents(ctx context.Context, events <-chan layers.Event) <-chan ClientEvent {
	if events == nil {
		return nil
	}
	out := make(chan ClientEvent)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				select {
				case out <- layerEventToClientEvent(evt):
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func forwardErrors(ctx context.Context, src <-chan error, dst chan<- error) {
	for src != nil {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-src:
			if !ok {
				src = nil
				continue
			}
			if err != nil {
				select {
				case dst <- err:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func clientEventToLayerEvent(evt ClientEvent) layers.Event {
	metadata := make(map[string]string, len(evt.Metadata)+4)
	for k, v := range evt.Metadata {
		metadata[k] = fmt.Sprint(v)
	}
	if evt.Symbol != "" {
		metadata["symbol"] = evt.Symbol
	}
	if evt.CorrelationID != "" {
		metadata["correlation_id"] = evt.CorrelationID
	}
	if evt.Channel != "" {
		metadata["channel"] = string(evt.Channel)
	}
	return layers.Event{
		Type:      string(evt.Channel),
		Payload:   evt.Payload,
		Metadata:  metadata,
		Timestamp: evt.At,
	}
}

func layerEventToClientEvent(evt layers.Event) ClientEvent {
	metadata := make(map[string]any, len(evt.Metadata))
	for k, v := range evt.Metadata {
		metadata[k] = v
	}
	symbol := lookupAndDelete(metadata, "symbol")
	correlation := lookupAndDelete(metadata, "correlation_id")
	channelStr := lookupAndDelete(metadata, "channel")
	channel := ChannelType(evt.Type)
	if channelStr != "" {
		channel = ChannelType(channelStr)
	}
	var payload TransportPayload
	switch p := evt.Payload.(type) {
	case nil:
		payload = nil
	case TransportPayload:
		payload = p
	default:
		payload = UnknownPayload{Detail: p}
	}
	return ClientEvent{
		Channel:       channel,
		Symbol:        symbol,
		At:            evt.Timestamp,
		Payload:       payload,
		CorrelationID: correlation,
		Metadata:      metadata,
	}
}

func lookupAndDelete(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if v, ok := metadata[key]; ok {
		delete(metadata, key)
		return fmt.Sprint(v)
	}
	return ""
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

func extractSourceSequence(meta map[string]any) (feed string, symbol string, sequence uint64, ok bool) {
	if meta == nil {
		return "", "", 0, false
	}
	feedVal, feedOK := meta[metadataKeySourceFeed].(string)
	symbolVal, symbolOK := meta[metadataKeySourceSymbol].(string)
	if !feedOK || !symbolOK || feedVal == "" || symbolVal == "" {
		return "", "", 0, false
	}
	rawSeq, present := meta[metadataKeySourceSequence]
	if !present {
		return "", "", 0, false
	}
	var seq uint64
	switch v := rawSeq.(type) {
	case uint64:
		seq = v
	case uint32:
		seq = uint64(v)
	case uint16:
		seq = uint64(v)
	case int:
		if v < 0 {
			return "", "", 0, false
		}
		seq = uint64(v)
	case int64:
		if v < 0 {
			return "", "", 0, false
		}
		seq = uint64(v)
	case float64:
		if v < 0 {
			return "", "", 0, false
		}
		seq = uint64(v)
	default:
		return "", "", 0, false
	}
	if seq == 0 {
		return "", "", 0, false
	}
	return feedVal, symbolVal, seq, true
}

type symbolGuardSnapshot struct {
	canonical string
	native    string
}

func (s symbolGuardSnapshot) Canonical() string { return s.canonical }

func (s symbolGuardSnapshot) Native() string { return s.native }

func guardSnapshotFromEvent(evt ClientEvent) (string, core.SymbolSnapshot) {
	meta := evt.Metadata
	feedType := metadataString(meta, metadataKeySourceFeed)
	canonical := metadataString(meta, metadataKeySourceSymbol)
	native := metadataString(meta, metadataKeySourceNative)
	var symbol string

	switch payload := evt.Payload.(type) {
	case BookPayload:
		if payload.Book != nil {
			if canonical == "" {
				canonical = payload.Book.Symbol
			}
			if payload.Book.VenueSymbol != "" {
				native = payload.Book.VenueSymbol
			}
		}
		if feedType == "" {
			feedType = "book"
		}
	case TradePayload:
		if payload.Trade != nil {
			if canonical == "" {
				canonical = payload.Trade.Symbol
			}
			if payload.Trade.VenueSymbol != "" {
				native = payload.Trade.VenueSymbol
			}
		}
		if feedType == "" {
			feedType = "trade"
		}
	case TickerPayload:
		if payload.Ticker != nil {
			if canonical == "" {
				canonical = payload.Ticker.Symbol
			}
			if payload.Ticker.VenueSymbol != "" {
				native = payload.Ticker.VenueSymbol
			}
		}
		if feedType == "" {
			feedType = "ticker"
		}
	default:
		if feedType == "" {
			return "", nil
		}
	}

	if canonical == "" {
		return "", nil
	}
	if native == "" {
		native = canonical
	}
	symbol = canonical
	if symbol == "" {
		symbol = metadataString(meta, metadataKeySourceSymbol)
	}
	feedID := feedType
	if symbol != "" {
		feedID = fmt.Sprintf("%s:%s", feedType, symbol)
	}
	return feedID, symbolGuardSnapshot{canonical: canonical, native: native}
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	if value, ok := meta[key]; ok {
		if s, ok := value.(string); ok {
			return s
		}
	}
	return ""
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
