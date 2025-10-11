package filter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/exchanges/binance/bridge"
	mdfilter "github.com/coachpo/meltica/pipeline"
)

func TestSessionManagerInitPrivateSession(t *testing.T) {
	dispatcher := &stubDispatcher{
		createKeys: []string{"listen-key-1"},
	}
	config := mdfilter.SessionConfig{
		KeepAliveInterval:  mdfilter.Duration(10 * time.Second),
		MaxRenewalFailures: 1,
		AutoRenew:          false,
	}
	sm := NewSessionManager(dispatcher, config)
	auth := &mdfilter.AuthContext{}

	require.NoError(t, sm.InitPrivateSession(context.Background(), auth))
	require.Equal(t, 1, dispatcher.CreateCalls())

	status := sm.GetSessionStatus()
	require.Equal(t, mdfilter.SessionStateActive, status.State)
	require.Equal(t, "listen-key-1", status.ListenKey)
	require.True(t, status.IsActive)
	require.NotZero(t, status.CreatedAt)
	require.NotZero(t, status.ExpiresAt)
	require.Equal(t, 0, status.FailureCount)
	require.Equal(t, 0, status.RenewalCount)
}

func TestSessionManagerPerformKeepAliveFailureTriggersRenewal(t *testing.T) {
	dispatcher := &stubDispatcher{
		createKeys:    []string{"initial-key", "renewed-key"},
		keepAliveErrs: []error{errors.New("network down")},
		closeErrs:     []error{nil},
	}
	config := mdfilter.SessionConfig{
		KeepAliveInterval:  mdfilter.Duration(time.Second),
		MaxRenewalFailures: 1,
		AutoRenew:          false,
	}
	sm := NewSessionManager(dispatcher, config)
	auth := &mdfilter.AuthContext{}
	require.NoError(t, sm.InitPrivateSession(context.Background(), auth))

	sm.performKeepAlive(context.Background())

	require.Equal(t, 2, dispatcher.CreateCalls())
	require.Equal(t, 1, dispatcher.KeepAliveCalls())
	require.Equal(t, 1, dispatcher.CloseCalls())

	status := sm.GetSessionStatus()
	require.Equal(t, "renewed-key", status.ListenKey)
	require.Equal(t, mdfilter.SessionStateActive, status.State)
	require.Equal(t, 0, status.FailureCount)
	require.True(t, status.IsActive)
}

func TestSessionManagerCloseTerminatesSession(t *testing.T) {
	dispatcher := &stubDispatcher{
		createKeys: []string{"listen-key-1"},
	}
	config := mdfilter.SessionConfig{AutoRenew: false}
	sm := NewSessionManager(dispatcher, config)
	auth := &mdfilter.AuthContext{}
	require.NoError(t, sm.InitPrivateSession(context.Background(), auth))

	require.NoError(t, sm.Close())
	require.Equal(t, 1, dispatcher.CloseCalls())
	require.Equal(t, mdfilter.SessionStateTerminated, auth.SessionState)
	require.False(t, auth.ListenKeyMetadata.IsActive)
}

func TestSessionManagerPerformKeepAliveSuccess(t *testing.T) {
	dispatcher := &stubDispatcher{createKeys: []string{"key"}}
	config := mdfilter.SessionConfig{KeepAliveInterval: mdfilter.Duration(time.Second), MaxRenewalFailures: 2}
	sm := NewSessionManager(dispatcher, config)
	auth := &mdfilter.AuthContext{}
	require.NoError(t, sm.InitPrivateSession(context.Background(), auth))

	prevRenewals := auth.ListenKeyMetadata.RenewalCount
	auth.ListenKeyMetadata.FailureCount = 3
	auth.SessionState = mdfilter.SessionStateRenewing

	sm.performKeepAlive(context.Background())

	require.Equal(t, 1, dispatcher.KeepAliveCalls())
	status := sm.GetSessionStatus()
	require.Equal(t, mdfilter.SessionStateActive, status.State)
	require.Equal(t, 0, status.FailureCount)
	require.Equal(t, prevRenewals+1, status.RenewalCount)
	require.False(t, status.LastKeepAlive.IsZero())
}

func TestSessionManagerPerformKeepAliveWithoutAuth(t *testing.T) {
	dispatcher := &stubDispatcher{}
	sm := NewSessionManager(dispatcher, mdfilter.SessionConfig{})
	sm.performKeepAlive(context.Background())
	require.Equal(t, 0, dispatcher.KeepAliveCalls())
	status := sm.GetSessionStatus()
	require.Equal(t, mdfilter.SessionStateInactive, status.State)
}

func TestSessionManagerRenewListenKeyFailureMarksFailed(t *testing.T) {
	dispatcher := &stubDispatcher{
		createKeys:    []string{"initial-key", "unused"},
		createErrs:    []error{nil, errors.New("create failed")},
		keepAliveErrs: []error{errors.New("timeout")},
	}
	config := mdfilter.SessionConfig{KeepAliveInterval: mdfilter.Duration(time.Second), MaxRenewalFailures: 1}
	sm := NewSessionManager(dispatcher, config)
	auth := &mdfilter.AuthContext{}
	require.NoError(t, sm.InitPrivateSession(context.Background(), auth))

	sm.performKeepAlive(context.Background())

	require.Equal(t, 2, dispatcher.CreateCalls())
	require.Equal(t, 1, dispatcher.KeepAliveCalls())
	require.Equal(t, 1, dispatcher.CloseCalls())
	status := sm.GetSessionStatus()
	require.Equal(t, mdfilter.SessionStateFailed, status.State)
	require.False(t, status.IsActive)
	require.Equal(t, "initial-key", status.ListenKey)
}

func TestSessionManagerAutoRenewStopsOnCancel(t *testing.T) {
	dispatcher := &stubDispatcher{createKeys: []string{"key"}}
	config := mdfilter.SessionConfig{
		KeepAliveInterval:  mdfilter.Duration(5 * time.Millisecond),
		MaxRenewalFailures: 3,
		AutoRenew:          true,
	}
	sm := NewSessionManager(dispatcher, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	auth := &mdfilter.AuthContext{}
	require.NoError(t, sm.InitPrivateSession(ctx, auth))

	require.Eventually(t, func() bool {
		return dispatcher.KeepAliveCalls() >= 1
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.NotNil(t, sm.cancelFunc)
	sm.cancelFunc()
	sm.cancelFunc = nil
	calls := dispatcher.KeepAliveCalls()
	time.Sleep(30 * time.Millisecond)
	require.Equal(t, calls, dispatcher.KeepAliveCalls())
}

func TestSessionManagerGetStatusInactiveWithoutAuth(t *testing.T) {
	sm := NewSessionManager(nil, mdfilter.SessionConfig{})
	status := sm.GetSessionStatus()
	require.Equal(t, mdfilter.SessionStateInactive, status.State)
}

func TestSessionManagerInitRequiresAuth(t *testing.T) {
	sm := NewSessionManager(&stubDispatcher{createKeys: []string{"key"}}, mdfilter.SessionConfig{})
	require.Error(t, sm.InitPrivateSession(context.Background(), nil))
}

func TestDefaultSessionConfigValues(t *testing.T) {
	cfg := DefaultSessionConfig()
	require.Equal(t, mdfilter.Duration(30*time.Minute), cfg.KeepAliveInterval)
	require.Equal(t, mdfilter.Duration(10*time.Minute), cfg.RenewalThreshold)
	require.Equal(t, 3, cfg.MaxRenewalFailures)
	require.True(t, cfg.AutoRenew)
}

type stubDispatcher struct {
	mu            sync.Mutex
	createKeys    []string
	createErrs    []error
	keepAliveErrs []error
	closeErrs     []error

	createCalls    int
	keepAliveCalls int
	closeCalls     int
}

var _ bridge.Dispatcher = (*stubDispatcher)(nil)

func (s *stubDispatcher) Dispatch(ctx context.Context, req mdfilter.InteractionRequest, result any) error {
	switch req.Method {
	case "POST":
		return s.handleCreate(result)
	case "PUT":
		var err error
		s.mu.Lock()
		s.keepAliveCalls++
		idx := s.keepAliveCalls - 1
		if idx < len(s.keepAliveErrs) && s.keepAliveErrs[idx] != nil {
			err = s.keepAliveErrs[idx]
		}
		s.mu.Unlock()
		return err
	case "DELETE":
		var err error
		s.mu.Lock()
		s.closeCalls++
		idx := s.closeCalls - 1
		if idx < len(s.closeErrs) && s.closeErrs[idx] != nil {
			err = s.closeErrs[idx]
		}
		s.mu.Unlock()
		return err
	default:
		return errors.New("unexpected method")
	}
}

func (s *stubDispatcher) handleCreate(result any) error {
	var (
		err       error
		key       string
		setResult bool
	)
	s.mu.Lock()
	s.createCalls++
	idx := s.createCalls - 1
	if idx < len(s.createErrs) && s.createErrs[idx] != nil {
		err = s.createErrs[idx]
	} else if idx >= len(s.createKeys) {
		err = errors.New("unexpected create call")
	} else {
		key = s.createKeys[idx]
		setResult = true
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if result != nil && setResult {
		resp, ok := result.(*struct {
			ListenKey string `json:"listenKey"`
		})
		if !ok {
			return errors.New("unexpected create result type")
		}
		resp.ListenKey = key
	}
	return nil
}

func (s *stubDispatcher) CreateCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createCalls
}

func (s *stubDispatcher) KeepAliveCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keepAliveCalls
}

func (s *stubDispatcher) CloseCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeCalls
}
