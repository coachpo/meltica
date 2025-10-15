package unit

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/dispatcher"
	"github.com/coachpo/meltica/core/events"
)

func TestFanoutErrorMessageIncludesMetadata(t *testing.T) {
	err := &dispatcher.FanoutError{
		Operation:         "dispatcher fan-out",
		TraceID:           "trace-2",
		EventKind:         events.KindExecReport,
		RoutingVersion:    11,
		SubscriberCount:   5,
		FailedSubscribers: []string{"a", "b"},
		Errors:            []error{errors.New("first"), errors.New("second")},
	}
	msg := err.Error()
	require.Contains(t, msg, "dispatcher fan-out")
	require.Contains(t, msg, "trace_id=trace-2")
	require.Contains(t, msg, "event_kind=")
	require.Contains(t, msg, "routing_version=11")
	require.Contains(t, msg, "subscriber_count=5")
	require.Contains(t, msg, "failed_subscribers=[a b]")
}

func TestFanoutErrorUnwrapReturnsCopy(t *testing.T) {
	inner := errors.New("boom")
	err := &dispatcher.FanoutError{Errors: []error{inner}}
	unwrapped := err.Unwrap()
	require.Len(t, unwrapped, 1)
	require.True(t, errors.Is(err, inner))
	unwrapped[0] = nil
	require.NotNil(t, err.Errors[0])
}
