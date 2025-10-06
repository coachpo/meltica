package streams

import (
	"context"
	"time"
)

const (
	RouteUnknown         = "unknown"
	RouteTradeUpdate     = "trade_update"
	RouteTickerUpdate    = "ticker_update"
	RouteBookSnapshot    = "book_snapshot"
	RouteDepthDelta      = "depth_delta"
	RouteOrderUpdate     = "order_update"
	RouteBalanceSnapshot = "balance_snapshot"
)

type RoutedMessage struct {
	Topic  string
	Raw    []byte
	At     time.Time
	Route  string
	Parsed any
}

type Router interface {
	SubscribePublic(ctx context.Context, topics ...string) (Subscription, error)
	SubscribePrivate(ctx context.Context) (Subscription, error)
	Close() error
}

type Subscription interface {
	C() <-chan RoutedMessage
	Err() <-chan error
	Close() error
}

type OrderBookSnapshotProvider interface {
	DepthSnapshot(ctx context.Context, symbol string, limit int) (BookEvent, int64, error)
}
