package layers

import (
	"context"
	"time"
)

// Connection manages the lifecycle of a network transport to an exchange.
//
// Contracts:
//   - Connect must be idempotent; repeated calls after success are no-ops.
//   - Close must be safe to invoke multiple times and release all resources.
//   - Deadline setters affect subsequent Read/Write operations immediately.
//   - Implementations must be safe for concurrent use.
type Connection interface {
	// Connect establishes the underlying transport; implementations should block
	// until the connection is negotiated or the context is cancelled.
	Connect(ctx context.Context) error

	// Close terminates the transport and cleans up associated resources. It must
	// tolerate multiple invocations and return nil once cleanup has completed.
	Close() error

	// IsConnected reports whether the transport is currently active.
	IsConnected() bool

	// SetReadDeadline configures the read deadline; a zero value removes the
	// deadline.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline configures the write deadline; a zero value removes the
	// deadline.
	SetWriteDeadline(t time.Time) error
}

// WSConnection extends Connection with WebSocket-specific primitives.
type WSConnection interface {
	Connection

	// ReadMessage blocks until a frame is available or a configured deadline
	// elapses. It returns the frame type and payload.
	ReadMessage() (messageType int, data []byte, err error)

	// WriteMessage transmits a frame using the provided opcode and payload.
	WriteMessage(messageType int, data []byte) error

	// Ping issues a ping frame and waits for the matching pong within the
	// provided context deadline.
	Ping(ctx context.Context) error

	// OnPong registers a handler invoked whenever a pong frame is received. The
	// handler must be safe for concurrent execution.
	OnPong(handler func())
}

// RESTConnection extends Connection for HTTP-based transports.
type RESTConnection interface {
	Connection

	// Do executes the prepared HTTP request and returns the exchange response.
	Do(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)

	// SetRateLimit configures the maximum number of requests permitted per
	// second. Implementations should smooth bursts to honour the ceiling.
	SetRateLimit(requestsPerSecond int)
}
