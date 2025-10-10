package router

import (
	"context"

	"github.com/coachpo/meltica/errs"
)

// FlowController coordinates backpressure metrics for channel dispatch operations.
type FlowController struct {
	metrics *RoutingMetrics
}

// NewFlowController constructs a flow controller using the provided metrics sink.
func NewFlowController(metrics *RoutingMetrics) *FlowController {
	return &FlowController{metrics: metrics}
}

// Send delivers the payload to the channel, tracking backpressure when the operation blocks.
func (f *FlowController) Send(ctx context.Context, messageTypeID string, inbox chan []byte, payload []byte) error {
	if inbox == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor channel not configured"))
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case inbox <- payload:
		return nil
	default:
	}

	if f.metrics != nil {
		f.metrics.RecordBackpressureStart(messageTypeID)
		defer f.metrics.RecordBackpressureEnd(messageTypeID)
	}

	select {
	case inbox <- payload:
		return nil
	case <-ctx.Done():
		err := ctx.Err()
		if err == nil {
			err = context.Canceled
		}
		return errs.New("", errs.CodeInvalid, errs.WithMessage("dispatch cancelled"), errs.WithCause(err))
	}
}

// Metrics returns the underlying metrics sink (used in tests).
func (f *FlowController) Metrics() *RoutingMetrics {
	return f.metrics
}
