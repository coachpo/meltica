package strategies

import (
	"context"
	"log"

	"github.com/coachpo/meltica/internal/schema"
)

// Logging logs all events - useful for debugging.
type Logging struct {
	Logger *log.Logger
}

// OnTrade logs trade events.
func (s *Logging) OnTrade(_ context.Context, _ *schema.Event, _ schema.TradePayload, price float64) {
	s.Logger.Printf("[STRATEGY] Trade received: price=%.2f", price)
}

// OnTicker logs ticker events.
func (s *Logging) OnTicker(_ context.Context, _ *schema.Event, payload schema.TickerPayload) {
	s.Logger.Printf("[STRATEGY] Ticker: last=%s bid=%s ask=%s",
		payload.LastPrice, payload.BidPrice, payload.AskPrice)
}

// OnBookSnapshot logs book snapshot events.
func (s *Logging) OnBookSnapshot(_ context.Context, _ *schema.Event, payload schema.BookSnapshotPayload) {
	s.Logger.Printf("[STRATEGY] Book snapshot: %d bids, %d asks", len(payload.Bids), len(payload.Asks))
}

// OnBookUpdate logs book update events.
func (s *Logging) OnBookUpdate(_ context.Context, _ *schema.Event, payload schema.BookUpdatePayload) {
	s.Logger.Printf("[STRATEGY] Book update: %d bids, %d asks", len(payload.Bids), len(payload.Asks))
}

// OnOrderFilled logs filled order events.
func (s *Logging) OnOrderFilled(_ context.Context, _ *schema.Event, payload schema.ExecReportPayload) {
	s.Logger.Printf("[STRATEGY] Order filled: id=%s qty=%s price=%s",
		payload.ClientOrderID, payload.FilledQuantity, payload.AvgFillPrice)
}

// OnOrderRejected logs rejected order events.
func (s *Logging) OnOrderRejected(_ context.Context, _ *schema.Event, payload schema.ExecReportPayload, reason string) {
	s.Logger.Printf("[STRATEGY] Order rejected: id=%s reason=%s", payload.ClientOrderID, reason)
}

// OnOrderPartialFill logs partial fill events.
func (s *Logging) OnOrderPartialFill(_ context.Context, _ *schema.Event, payload schema.ExecReportPayload) {
	s.Logger.Printf("[STRATEGY] Order partial fill: id=%s filled=%s remaining=%s",
		payload.ClientOrderID, payload.FilledQuantity, payload.RemainingQty)
}

// OnOrderCancelled logs cancelled order events.
func (s *Logging) OnOrderCancelled(_ context.Context, _ *schema.Event, payload schema.ExecReportPayload) {
	s.Logger.Printf("[STRATEGY] Order cancelled: id=%s", payload.ClientOrderID)
}
