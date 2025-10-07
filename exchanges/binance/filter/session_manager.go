package filter

import (
	"context"
	"fmt"
	"time"

	"github.com/coachpo/meltica/exchanges/shared/routing"
	mdfilter "github.com/coachpo/meltica/marketdata/filter"
)

// SessionManager handles Binance private session lifecycle including listen-key management
type SessionManager struct {
	router      routing.RESTDispatcher
	config      mdfilter.SessionConfig
	authContext *mdfilter.AuthContext
	cancelFunc  context.CancelFunc
}

// NewSessionManager creates a new session manager for Binance private streams
func NewSessionManager(router routing.RESTDispatcher, config mdfilter.SessionConfig) *SessionManager {
	return &SessionManager{
		router: router,
		config: config,
	}
}

// InitPrivateSession initializes a private session with listen-key management
func (sm *SessionManager) InitPrivateSession(ctx context.Context, auth *mdfilter.AuthContext) error {
	if auth == nil {
		return fmt.Errorf("auth context is required")
	}

	sm.authContext = auth

	// Create listen key
	listenKey, err := sm.createListenKey(ctx)
	if err != nil {
		return fmt.Errorf("failed to create listen key: %w", err)
	}

	// Initialize auth context with listen key metadata
	auth.ListenKey = listenKey
	auth.ListenKeyMetadata = &mdfilter.ListenKeyMetadata{
		Key:           listenKey,
		CreatedAt:     time.Now(),
		LastKeepAlive: time.Now(),
		ExpiresAt:     time.Now().Add(60 * time.Minute), // Binance listen keys expire in 60 minutes
		RenewalCount:  0,
		FailureCount:  0,
		IsActive:      true,
	}
	auth.SessionState = mdfilter.SessionStateActive
	auth.RenewalInterval = mdfilter.Duration(30 * time.Minute) // Renew every 30 minutes
	auth.ExpiresAt = time.Now().Add(60 * time.Minute)

	// Start background keep-alive goroutine
	if sm.config.AutoRenew {
		sm.startKeepAlive(ctx)
	}

	return nil
}

// Close terminates the private session and releases resources
func (sm *SessionManager) Close() error {
	if sm.cancelFunc != nil {
		sm.cancelFunc()
		sm.cancelFunc = nil
	}

	// Close the listen key if we have an active session
	if sm.authContext != nil && sm.authContext.ListenKey != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := sm.closeListenKey(ctx, sm.authContext.ListenKey)
		if err != nil {
			return fmt.Errorf("failed to close listen key: %w", err)
		}

		sm.authContext.SessionState = mdfilter.SessionStateTerminated
		sm.authContext.ListenKeyMetadata.IsActive = false
	}

	return nil
}

// createListenKey creates a new Binance listen key
func (sm *SessionManager) createListenKey(ctx context.Context) (string, error) {
	var result struct {
		ListenKey string `json:"listenKey"`
	}

	msg := routing.RESTMessage{
		API:    "spot",
		Method: "POST",
		Path:   "/api/v3/userDataStream",
		Signed: false,
	}

	err := sm.router.Dispatch(ctx, msg, &result)
	if err != nil {
		return "", fmt.Errorf("failed to create listen key: %w", err)
	}

	if result.ListenKey == "" {
		return "", fmt.Errorf("empty listen key received")
	}

	return result.ListenKey, nil
}

// keepAlive sends a keep-alive request to extend the listen key
func (sm *SessionManager) keepAlive(ctx context.Context, listenKey string) error {
	msg := routing.RESTMessage{
		API:    "spot",
		Method: "PUT",
		Path:   fmt.Sprintf("/api/v3/userDataStream?listenKey=%s", listenKey),
		Signed: false,
	}

	err := sm.router.Dispatch(ctx, msg, nil)
	if err != nil {
		return fmt.Errorf("failed to keep alive listen key: %w", err)
	}

	return nil
}

// closeListenKey closes the listen key
func (sm *SessionManager) closeListenKey(ctx context.Context, listenKey string) error {
	msg := routing.RESTMessage{
		API:    "spot",
		Method: "DELETE",
		Path:   fmt.Sprintf("/api/v3/userDataStream?listenKey=%s", listenKey),
		Signed: false,
	}

	err := sm.router.Dispatch(ctx, msg, nil)
	if err != nil {
		return fmt.Errorf("failed to close listen key: %w", err)
	}

	return nil
}

// startKeepAlive starts the background keep-alive goroutine
func (sm *SessionManager) startKeepAlive(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	sm.cancelFunc = cancel

	go func() {
		keepAliveTicker := time.NewTicker(time.Duration(sm.config.KeepAliveInterval))
		defer keepAliveTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-keepAliveTicker.C:
				sm.performKeepAlive(ctx)
			}
		}
	}()
}

// performKeepAlive performs a single keep-alive operation
func (sm *SessionManager) performKeepAlive(ctx context.Context) {
	if sm.authContext == nil || sm.authContext.ListenKey == "" {
		return
	}

	err := sm.keepAlive(ctx, sm.authContext.ListenKey)
	if err != nil {
		sm.authContext.ListenKeyMetadata.FailureCount++
		sm.authContext.SessionState = mdfilter.SessionStateRenewing

		// Try to renew the listen key if keep-alive fails
		if sm.authContext.ListenKeyMetadata.FailureCount >= sm.config.MaxRenewalFailures {
			sm.renewListenKey(ctx)
		}
		return
	}

	// Update metadata on successful keep-alive
	sm.authContext.ListenKeyMetadata.LastKeepAlive = time.Now()
	sm.authContext.ListenKeyMetadata.RenewalCount++
	sm.authContext.ListenKeyMetadata.ExpiresAt = time.Now().Add(60 * time.Minute)
	sm.authContext.ExpiresAt = sm.authContext.ListenKeyMetadata.ExpiresAt
	sm.authContext.SessionState = mdfilter.SessionStateActive
	sm.authContext.ListenKeyMetadata.FailureCount = 0
}

// renewListenKey attempts to renew the listen key
func (sm *SessionManager) renewListenKey(ctx context.Context) {
	// Close old listen key
	if sm.authContext.ListenKey != "" {
		_ = sm.closeListenKey(ctx, sm.authContext.ListenKey)
	}

	// Create new listen key
	newListenKey, err := sm.createListenKey(ctx)
	if err != nil {
		sm.authContext.SessionState = mdfilter.SessionStateFailed
		sm.authContext.ListenKeyMetadata.IsActive = false
		return
	}

	// Update auth context with new listen key
	sm.authContext.ListenKey = newListenKey
	sm.authContext.ListenKeyMetadata = &mdfilter.ListenKeyMetadata{
		Key:           newListenKey,
		CreatedAt:     time.Now(),
		LastKeepAlive: time.Now(),
		ExpiresAt:     time.Now().Add(60 * time.Minute),
		RenewalCount:  0,
		FailureCount:  0,
		IsActive:      true,
	}
	sm.authContext.SessionState = mdfilter.SessionStateActive
	sm.authContext.LastRenewal = time.Now()
}

// GetSessionStatus returns the current session status
func (sm *SessionManager) GetSessionStatus() *SessionStatus {
	if sm.authContext == nil {
		return &SessionStatus{
			State: mdfilter.SessionStateInactive,
		}
	}

	return &SessionStatus{
			State:           sm.authContext.SessionState,
			ListenKey:       sm.authContext.ListenKey,
			CreatedAt:       sm.authContext.ListenKeyMetadata.CreatedAt,
			LastKeepAlive:   sm.authContext.ListenKeyMetadata.LastKeepAlive,
			ExpiresAt:       sm.authContext.ListenKeyMetadata.ExpiresAt,
			RenewalCount:    sm.authContext.ListenKeyMetadata.RenewalCount,
			FailureCount:    sm.authContext.ListenKeyMetadata.FailureCount,
			IsActive:        sm.authContext.ListenKeyMetadata.IsActive,
		}
}

// SessionStatus represents the current status of a private session
type SessionStatus struct {
	State         mdfilter.SessionState
	ListenKey     string
	CreatedAt     time.Time
	LastKeepAlive time.Time
	ExpiresAt     time.Time
	RenewalCount  int
	FailureCount  int
	IsActive      bool
}

// DefaultSessionConfig returns a sensible default configuration for Binance sessions
func DefaultSessionConfig() mdfilter.SessionConfig {
	return mdfilter.SessionConfig{
		KeepAliveInterval: mdfilter.Duration(30 * time.Minute),
		RenewalThreshold:  mdfilter.Duration(10 * time.Minute),
		MaxRenewalFailures: 3,
		AutoRenew:         true,
	}
}