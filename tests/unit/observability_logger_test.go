package unit

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/dispatcher"
	"github.com/coachpo/meltica/core/events"
)

func TestFanoutErrorUnwrapsAndFormats(t *testing.T) {
	inner := errors.New("subscriber failure")
	panicErr := errors.New("subscriber panic")
	err := &dispatcher.FanoutError{
		Operation:         "dispatcher fan-out",
		TraceID:           "trace-1",
		EventKind:         events.KindMarketData,
		RoutingVersion:    7,
		SubscriberCount:   3,
		FailedSubscribers: []string{"fail"},
		Errors:            []error{inner, panicErr},
	}
	require.Contains(t, err.Error(), "trace_id=trace-1")
	require.True(t, errors.Is(err, inner))
	require.True(t, errors.Is(err, panicErr))
}
