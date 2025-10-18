package binance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
	json "github.com/goccy/go-json"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/coachpo/meltica/internal/telemetry"
)

const (
	// Binance WebSocket endpoints
	binanceWSURL     = "wss://stream.binance.com:9443"
	binanceTestnetWS = "wss://testnet.binance.vision/ws"

	// Rate limits per Binance documentation
	maxMessagesPerSecond = 5
	maxStreamsPerConn    = 1024
	maxConnAttemptsPerIP = 300

	// Timeouts and intervals
	pingInterval    = 20 * time.Second
	pongTimeout     = 60 * time.Second
	reconnectDelay  = 5 * time.Second
	maxReconnectDel = 60 * time.Second
	connectionTTL   = 23 * time.Hour // Binance connections valid for 24h, reconnect before
)

// Errors returned by WebSocket provider.
var (
	// ErrTooManyStreams is returned when attempting to subscribe to more than 1024 streams.
	ErrTooManyStreams = errors.New("too many streams subscribed")
	// ErrRateLimitExceeded is returned when message rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("message rate limit exceeded")
	// ErrConnectionClosed is returned when WebSocket connection is closed.
	ErrConnectionClosed = errors.New("websocket connection closed")
	// ErrReconnectFailed is returned after max reconnection attempts.
	ErrReconnectFailed = errors.New("failed to reconnect after max attempts")
)

// WSProviderConfig configures the WebSocket provider.
type WSProviderConfig struct {
	BaseURL       string
	UseTestnet    bool
	APIKey        string // Optional: for authenticated streams
	RateLimiter   *RateLimiter
	Logger        *log.Logger
	MaxReconnects int
}

// WSProvider implements real WebSocket connection to Binance.
type WSProvider struct {
	cfg         WSProviderConfig
	conn        *websocket.Conn
	connMu      sync.RWMutex
	streams     map[string]bool
	streamsMu   sync.RWMutex
	rateLimiter *RateLimiter
	logger      *log.Logger
	pongMu      sync.Mutex
	pongTimer   *time.Timer

	ctx        context.Context
	cancel     context.CancelFunc
	reconnectC chan struct{}

	started        bool
	startedMu      sync.Mutex
	reconnectCount int
	lastConnect    time.Time

	// Telemetry
	reconnectCounter metric.Int64Counter
}

// NewBinanceWSProvider creates a production-ready WebSocket provider.
func NewBinanceWSProvider(cfg WSProviderConfig) *WSProvider {
	if cfg.BaseURL == "" {
		if cfg.UseTestnet {
			cfg.BaseURL = binanceTestnetWS
		} else {
			cfg.BaseURL = binanceWSURL
		}
	}
	if cfg.RateLimiter == nil {
		cfg.RateLimiter = NewRateLimiter(maxMessagesPerSecond)
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.MaxReconnects == 0 {
		cfg.MaxReconnects = 10
	}

	meter := otel.Meter("provider")
	reconnectCounter, _ := meter.Int64Counter("provider.reconnections",
		metric.WithDescription("Number of provider reconnection attempts"),
		metric.WithUnit("{reconnection}"))

	//nolint:exhaustruct // other fields initialized on Subscribe()
	return &WSProvider{
		cfg:              cfg,
		streams:          make(map[string]bool),
		rateLimiter:      cfg.RateLimiter,
		logger:           cfg.Logger,
		reconnectC:       make(chan struct{}, 1),
		reconnectCounter: reconnectCounter,
	}
}

// Subscribe establishes WebSocket connection and subscribes to topics.
// Implements auto-reconnection with exponential backoff.
func (p *WSProvider) Subscribe(ctx context.Context, topics []string) (<-chan []byte, <-chan error, error) {
	p.startedMu.Lock()
	if p.started {
		p.startedMu.Unlock()
		return nil, nil, errors.New("provider already started")
	}
	p.started = true
	p.startedMu.Unlock()

	if len(topics) > maxStreamsPerConn {
		return nil, nil, ErrTooManyStreams
	}

	p.ctx, p.cancel = context.WithCancel(ctx)

	// Large buffer for high-frequency Binance streams (9 streams × ~100 msgs/sec)
	frames := make(chan []byte, 2048)
	errs := make(chan error, 16)

	// Initial connection
	if err := p.connect(); err != nil {
		return nil, nil, fmt.Errorf("initial connection failed: %w", err)
	}

	// Subscribe to topics
	if err := p.subscribeTo(topics); err != nil {
		p.disconnect()
		return nil, nil, fmt.Errorf("initial subscription failed: %w", err)
	}

	// Start goroutines
	go p.pingLoop()
	go p.readLoop(frames, errs)
	go p.reconnectLoop(topics)
	go p.connectionTTLMonitor()

	return frames, errs, nil
}

// connect establishes WebSocket connection to Binance.
func (p *WSProvider) connect() error {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if p.conn != nil {
		return nil // Already connected
	}

	// Build combined stream URL
	url := fmt.Sprintf("%s/stream", p.cfg.BaseURL)

	// Create context with timeout for dial
	dialCtx, dialCancel := context.WithTimeout(p.ctx, 10*time.Second)
	defer dialCancel()

	// Dial with coder/websocket (first-class context support)
	// Note: Binance doesn't use WebSocket compression, data comes uncompressed
	conn, _, err := websocket.Dial(dialCtx, url, &websocket.DialOptions{
		OnPongReceived: func(ctx context.Context, payload []byte) {
			p.resetPongTimer()
		},
	})
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	// Set read limit to 10MB to handle large orderbook snapshots
	// Default is 32KB which is too small for depth@100ms streams
	conn.SetReadLimit(10 * 1024 * 1024)

	p.conn = conn
	p.lastConnect = time.Now()
	p.reconnectCount = 0
	p.resetPongTimer()
	p.logger.Printf("binance ws: connected to %s (10MB read limit)", url)

	return nil
}

// disconnect closes the WebSocket connection.
func (p *WSProvider) disconnect() {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if p.conn != nil {
		// Close with normal closure status
		_ = p.conn.Close(websocket.StatusNormalClosure, "")
		p.conn = nil
		p.logger.Println("binance ws: disconnected")
	}
	p.stopPongTimer()
}

// subscribeTo sends subscribe messages for the given topics.
func (p *WSProvider) subscribeTo(topics []string) error {
	if len(topics) == 0 {
		p.logger.Println("binance ws: subscribeTo called with 0 topics")
		return nil
	}

	p.logger.Printf("binance ws: subscribing to %d topics: %v", len(topics), topics)

	p.streamsMu.Lock()
	for _, topic := range topics {
		p.streams[topic] = true
	}
	p.streamsMu.Unlock()

	// Build subscribe message
	msg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": topics,
		"id":     time.Now().UnixNano(),
	}

	err := p.sendJSON(msg)
	if err != nil {
		p.logger.Printf("binance ws: subscribe failed: %v", err)
	} else {
		p.logger.Printf("binance ws: subscribe message sent successfully")
	}
	return err
}

// sendJSON sends a JSON message with rate limiting.
func (p *WSProvider) sendJSON(v interface{}) error {
	// Rate limiting
	if !p.rateLimiter.Allow() {
		return ErrRateLimitExceeded
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	p.connMu.RLock()
	conn := p.conn
	p.connMu.RUnlock()

	if conn == nil {
		return ErrConnectionClosed
	}

	// Write with timeout context
	writeCtx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	if err := conn.Write(writeCtx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

// pingLoop sends periodic ping frames as required by Binance.
func (p *WSProvider) pingLoop() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.connMu.RLock()
			conn := p.conn
			p.connMu.RUnlock()

			if conn != nil {
				pingCtx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
				if err := conn.Ping(pingCtx); err != nil {
					cancel()
					p.logger.Printf("binance ws: ping failed: %v", err)
					p.triggerReconnect()
				} else {
					cancel()
					p.resetPongTimer()
				}
			}
		}
	}
}

// readLoop reads messages from WebSocket.
func (p *WSProvider) readLoop(frames chan<- []byte, errs chan<- error) {
	defer close(frames)
	defer close(errs)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		p.connMu.RLock()
		conn := p.conn
		p.connMu.RUnlock()

		if conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Read with context for automatic timeout
		readCtx, cancel := context.WithTimeout(p.ctx, pongTimeout)
		msgType, data, err := conn.Read(readCtx)
		cancel()

		if err != nil {
			// Check for normal closure
			closeStatus := websocket.CloseStatus(err)
			if closeStatus == websocket.StatusNormalClosure || closeStatus == websocket.StatusGoingAway {
				p.logger.Println("binance ws: connection closed normally")
				return
			}

			// Check for context errors
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				select {
				case <-p.ctx.Done():
					return
				default:
					p.logger.Printf("binance ws: read timeout, reconnecting")
					p.triggerReconnect()
					continue
				}
			}

			// Other errors trigger reconnect
			p.logger.Printf("binance ws: read error: %v", err)
			select {
			case errs <- fmt.Errorf("read message: %w", err):
			default:
			}
			p.triggerReconnect()
			continue
		}

		// Handle text messages (JSON frames)
		if msgType == websocket.MessageText {
			select {
			case frames <- data:
			case <-p.ctx.Done():
				return
			default:
				p.logger.Println("binance ws: frame channel full, dropping message")
			}
			p.resetPongTimer()
		}
		// Binary messages are ignored (we use JSON streams)
	}
}

func (p *WSProvider) resetPongTimer() {
	p.pongMu.Lock()
	defer p.pongMu.Unlock()

	if p.pongTimer == nil {
		p.pongTimer = time.AfterFunc(pongTimeout, p.handlePongTimeout)
		return
	}
	if !p.pongTimer.Stop() {
		select {
		case <-p.pongTimer.C:
		default:
		}
	}
	p.pongTimer.Reset(pongTimeout)
}

func (p *WSProvider) stopPongTimer() {
	p.pongMu.Lock()
	defer p.pongMu.Unlock()

	if p.pongTimer != nil {
		if !p.pongTimer.Stop() {
			select {
			case <-p.pongTimer.C:
			default:
			}
		}
		p.pongTimer = nil
	}
}

func (p *WSProvider) handlePongTimeout() {
	p.logger.Printf("binance ws: pong timeout after %v", pongTimeout)
	p.triggerReconnect()
}

// reconnectLoop handles automatic reconnection.
func (p *WSProvider) reconnectLoop(topics []string) {
	backoff := reconnectDelay

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.reconnectC:
			p.reconnectCount++
			if p.reconnectCount > p.cfg.MaxReconnects {
				p.logger.Printf("binance ws: max reconnect attempts reached (%d)", p.cfg.MaxReconnects)
				continue
			}

			// Record reconnection attempt
			if p.reconnectCounter != nil {
				p.reconnectCounter.Add(p.ctx, 1, metric.WithAttributes(
					attribute.String("environment", telemetry.Environment()),
					attribute.String("provider", "binance"),
					attribute.String("reason", "connection_lost")))
			}

			p.logger.Printf("binance ws: reconnecting (attempt %d/%d) after %v",
				p.reconnectCount, p.cfg.MaxReconnects, backoff)

			time.Sleep(backoff)

			// Exponential backoff
			backoff *= 2
			if backoff > maxReconnectDel {
				backoff = maxReconnectDel
			}

			p.disconnect()
			if err := p.connect(); err != nil {
				p.logger.Printf("binance ws: reconnect failed: %v", err)
				p.triggerReconnect()
				continue
			}

			// Resubscribe to all streams
			if err := p.subscribeTo(topics); err != nil {
				p.logger.Printf("binance ws: resubscribe failed: %v", err)
				p.triggerReconnect()
				continue
			}

			p.logger.Println("binance ws: reconnected successfully")
			backoff = reconnectDelay // Reset backoff on success
		}
	}
}

// connectionTTLMonitor monitors connection age and triggers reconnect before TTL expires.
func (p *WSProvider) connectionTTLMonitor() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.connMu.RLock()
			age := time.Since(p.lastConnect)
			p.connMu.RUnlock()

			if age >= connectionTTL {
				p.logger.Println("binance ws: connection TTL reached, reconnecting")
				// Record TTL-based reconnection
				if p.reconnectCounter != nil {
					p.reconnectCounter.Add(p.ctx, 1, metric.WithAttributes(
						attribute.String("environment", telemetry.Environment()),
						attribute.String("provider", "binance"),
						attribute.String("reason", "ttl_expired")))
				}
				p.triggerReconnect()
			}
		}
	}
}

// triggerReconnect signals the reconnect loop.
func (p *WSProvider) triggerReconnect() {
	select {
	case p.reconnectC <- struct{}{}:
	default:
		// Reconnect already pending
	}
}

// Close gracefully closes the WebSocket connection.
func (p *WSProvider) Close() error {
	p.startedMu.Lock()
	if !p.started {
		p.startedMu.Unlock()
		return nil
	}
	p.started = false
	p.startedMu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}
	p.disconnect()
	return nil
}
