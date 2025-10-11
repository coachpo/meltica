package ws

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/layers"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/errs"
)

type layerWSConnection struct {
	client        *Client
	readDeadline  time.Time
	writeDeadline time.Time
	mu            sync.RWMutex
	pongs         []func()
}

var _ layers.WSConnection = (*layerWSConnection)(nil)

// AsLayerInterface exposes the Binance websocket client as a layers.WSConnection.
func (c *Client) AsLayerInterface() layers.WSConnection {
	if c == nil {
		return nil
	}
	return &layerWSConnection{client: c}
}

func (l *layerWSConnection) Connect(ctx context.Context) error {
	if l.client == nil {
		return errs.New("binance", errs.CodeInvalid, errs.WithMessage("ws adapter: missing client"))
	}
	return l.client.Connect(ctx)
}

func (l *layerWSConnection) Close() error {
	if l.client == nil {
		return nil
	}
	return l.client.Close()
}

func (l *layerWSConnection) IsConnected() bool { return true }

// LegacyStreamClient returns the underlying transport client for legacy call sites.
func (l *layerWSConnection) LegacyStreamClient() coretransport.StreamClient {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client
}

func (l *layerWSConnection) SetReadDeadline(t time.Time) error {
	l.mu.Lock()
	l.readDeadline = t
	l.mu.Unlock()
	return nil
}

func (l *layerWSConnection) SetWriteDeadline(t time.Time) error {
	l.mu.Lock()
	l.writeDeadline = t
	l.mu.Unlock()
	return nil
}

func (l *layerWSConnection) ReadMessage() (int, []byte, error) {
	return 0, nil, errs.NotSupported("binance ws adapter: ReadMessage not implemented")
}

func (l *layerWSConnection) WriteMessage(int, []byte) error {
	return errs.NotSupported("binance ws adapter: WriteMessage not implemented")
}

func (l *layerWSConnection) Ping(context.Context) error {
	return errs.NotSupported("binance ws adapter: Ping not implemented")
}

func (l *layerWSConnection) OnPong(handler func()) {
	if handler == nil {
		return
	}
	l.mu.Lock()
	l.pongs = append(l.pongs, handler)
	l.mu.Unlock()
}
