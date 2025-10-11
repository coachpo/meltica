package rest

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/layers"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/errs"
)

type layerRESTConnection struct {
	client        *Client
	mu            sync.RWMutex
	rateLimit     int
	readDeadline  time.Time
	writeDeadline time.Time
}

var _ layers.RESTConnection = (*layerRESTConnection)(nil)

// AsLayerInterface exposes the Binance REST client as a layers.RESTConnection.
func (c *Client) AsLayerInterface() layers.RESTConnection {
	if c == nil {
		return nil
	}
	return &layerRESTConnection{client: c}
}

func (l *layerRESTConnection) Connect(ctx context.Context) error {
	if l.client == nil {
		return errs.New("binance", errs.CodeInvalid, errs.WithMessage("rest adapter: missing client"))
	}
	return l.client.Connect(ctx)
}

func (l *layerRESTConnection) Close() error {
	if l.client == nil {
		return nil
	}
	return l.client.Close()
}

func (l *layerRESTConnection) IsConnected() bool { return true }

func (l *layerRESTConnection) SetReadDeadline(t time.Time) error {
	l.mu.Lock()
	l.readDeadline = t
	l.mu.Unlock()
	return nil
}

func (l *layerRESTConnection) SetWriteDeadline(t time.Time) error {
	l.mu.Lock()
	l.writeDeadline = t
	l.mu.Unlock()
	return nil
}

func (l *layerRESTConnection) Do(context.Context, *layers.HTTPRequest) (*layers.HTTPResponse, error) {
	return nil, errs.NotSupported("binance rest adapter: Do not implemented")
}

func (l *layerRESTConnection) SetRateLimit(rps int) {
	l.mu.Lock()
	l.rateLimit = rps
	l.mu.Unlock()
}

// LegacyRESTClient returns the wrapped REST client for migration compatibility.
func (l *layerRESTConnection) LegacyRESTClient() coretransport.RESTClient {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client
}
