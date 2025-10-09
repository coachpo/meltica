package unit

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/parser"
)

func TestValidationStageMarksValid(t *testing.T) {
	stage := parser.NewValidationStage()
	env := &framework.MessageEnvelope{}
	result := stage.Validate(env)
	require.True(t, result.Valid)
	require.True(t, env.Validated())
	require.Nil(t, result.Err)
	require.EqualValues(t, 0, result.InvalidCount)
}

func TestValidationStageTracksInvalidMessages(t *testing.T) {
	clock := &stubClock{now: time.Unix(0, 0)}
	validatorErr := errors.New("missing required field")
	stage := parser.NewValidationStage(
		parser.WithValidators(parser.ValidatorFunc(func(env *framework.MessageEnvelope) error {
			return validatorErr
		})),
		parser.WithValidationWindow(2*time.Second),
		parser.WithInvalidThreshold(2),
		parser.WithClock(clock.Now),
	)

	firstEnv := &framework.MessageEnvelope{}
	first := stage.Validate(firstEnv)
	require.False(t, first.Valid)
	require.Error(t, first.Err)
	require.IsType(t, &errs.E{}, first.Err)
	require.EqualValues(t, 1, first.InvalidCount)
	require.False(t, first.ThresholdExceeded)
	require.False(t, firstEnv.Validated())
	require.Len(t, firstEnv.Errors(), 1)

	clock.Advance(time.Second)
	secondEnv := &framework.MessageEnvelope{}
	second := stage.Validate(secondEnv)
	require.False(t, second.Valid)
	require.EqualValues(t, 2, second.InvalidCount)
	require.True(t, second.ThresholdExceeded)
	require.Len(t, secondEnv.Errors(), 1)

	clock.Advance(3 * time.Second)
	thirdEnv := &framework.MessageEnvelope{}
	third := stage.Validate(thirdEnv)
	require.False(t, third.Valid)
	require.EqualValues(t, 1, third.InvalidCount)
	require.False(t, third.ThresholdExceeded)
	require.Len(t, thirdEnv.Errors(), 1)
}

type stubClock struct {
	mu  sync.Mutex
	now time.Time
}

func (s *stubClock) Now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now
}

func (s *stubClock) Advance(delta time.Duration) {
	s.mu.Lock()
	s.now = s.now.Add(delta)
	s.mu.Unlock()
}
