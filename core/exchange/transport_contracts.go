package exchange

import (
	"context"
	"net/http"
	"time"
)

// Connection defines the minimal lifecycle management shared by REST and streaming clients.
// Connect should establish or verify underlying connectivity, honouring context cancellation.
// Close must release any associated resources; repeated calls should be safe.
type Connection interface {
	Connect(ctx context.Context) error
	Close() error
}

// RESTRequest is the normalized payload produced by Level 2 routing for a single HTTP call.
// API identifies an exchange specific surface (e.g. spot, linear futures).
// Query values should already be encoded without pagination defaults.
type RESTRequest struct {
	API    string
	Method string
	Path   string
	Query  map[string]string
	Body   []byte
	Signed bool
	Header http.Header
}

// RESTResponse captures the raw data returned by a RESTClient.
// Status conveys the HTTP status code, Header retains the response headers, and Body contains the raw payload bytes.
// ReceivedAt records when the response body was consumed.
type RESTResponse struct {
	Status     int
	Header     http.Header
	Body       []byte
	ReceivedAt time.Time
}

// RESTClient standardises Level 1 behaviour for stateless request/response workflows.
// DoRequest must execute the HTTP call described by RESTRequest and return the raw RESTResponse.
// HandleResponse should decode or map the RESTResponse into out, returning protocol-level errors.
// HandleError is invoked when DoRequest fails before a response is available and should translate infrastructure faults into canonical errors.
type RESTClient interface {
	Connection
	DoRequest(ctx context.Context, req RESTRequest) (*RESTResponse, error)
	HandleResponse(ctx context.Context, req RESTRequest, resp *RESTResponse, out any) error
	HandleError(ctx context.Context, req RESTRequest, err error) error
}

// StreamScope differentiates public market data streams from private user streams.
type StreamScope string

const (
	StreamScopePublic  StreamScope = "public"
	StreamScopePrivate StreamScope = "private"
)

// StreamTopic identifies a logical websocket topic to be delivered through the streaming client.
// Name conveys the exchange specific stream identifier while Params can include query like depth levels.
type StreamTopic struct {
	Scope  StreamScope
	Name   string
	Params map[string]string
}

// StreamMessage represents an outbound websocket frame initiated by the business layer.
// Payload should be pre-serialized according to exchange expectations; Metadata may contain helper attributes such as idempotency keys.
type StreamMessage struct {
	Payload  []byte
	Metadata map[string]string
}

// StreamSubscription exposes the raw websocket frames and associated errors emitted by the streaming client.
type StreamSubscription interface {
	Messages() <-chan RawMessage
	Errors() <-chan error
	Close() error
}

// StreamClient standardises Level 1 behaviour for subscription based websocket transports.
// Subscribe must establish any required connections and begin delivering frames for the requested topics.
// Unsubscribe should best-effort detach the topics from an existing subscription.
// Publish sends an outbound frame (e.g. placing an order or confirming a subscription) when supported by the exchange.
// HandleError converts transport or protocol failures into canonical errors for higher layers.
type StreamClient interface {
	Connection
	Subscribe(ctx context.Context, topics ...StreamTopic) (StreamSubscription, error)
	Unsubscribe(ctx context.Context, sub StreamSubscription, topics ...StreamTopic) error
	Publish(ctx context.Context, message StreamMessage) error
	HandleError(ctx context.Context, err error) error
}

// RawMessage carries a websocket frame along with the timestamp it was received.
type RawMessage struct {
	Data []byte
	At   time.Time
}
