package strategies

import (
	"context"

	"github.com/coachpo/meltica/internal/schema"
)

// NoOp is a strategy that does nothing - useful for monitoring-only lambdas.
type NoOp struct{}

// OnTrade does nothing.
func (s *NoOp) OnTrade(_ context.Context, _ *schema.Event, _ schema.TradePayload, _ float64) {}

// OnTicker does nothing.
func (s *NoOp) OnTicker(_ context.Context, _ *schema.Event, _ schema.TickerPayload) {}

// OnBookSnapshot does nothing.
func (s *NoOp) OnBookSnapshot(_ context.Context, _ *schema.Event, _ schema.BookSnapshotPayload) {}

// OnBookUpdate does nothing.
func (s *NoOp) OnBookUpdate(_ context.Context, _ *schema.Event, _ schema.BookUpdatePayload) {}

// OnOrderFilled does nothing.
func (s *NoOp) OnOrderFilled(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {}

// OnOrderRejected does nothing.
func (s *NoOp) OnOrderRejected(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload, _ string) {}

// OnOrderPartialFill does nothing.
func (s *NoOp) OnOrderPartialFill(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {}

// OnOrderCancelled does nothing.
func (s *NoOp) OnOrderCancelled(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {}
