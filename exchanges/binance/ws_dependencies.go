package binance

import (
	"context"
	"strings"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/internal"
)

// wsDependencies implements the routing WSDependencies interface without depending on Exchange.
type wsDependencies struct {
	symbols      *symbolService
	listenKeys   *listenKeyService
	depths       depthSnapshotProvider
	exchangeName core.ExchangeName
}

// depthSnapshotProvider is an interface for fetching book snapshots.
type depthSnapshotProvider interface {
	Snapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error)
}

func newWSDependencies(name core.ExchangeName, symbols *symbolService, listenKeys *listenKeyService, depths depthSnapshotProvider) *wsDependencies {
	return &wsDependencies{symbols: symbols, listenKeys: listenKeys, depths: depths, exchangeName: name}
}

func (d *wsDependencies) CanonicalSymbol(binanceSymbol string) (string, error) {
	if d.symbols == nil {
		return "", internal.Exchange("symbol service unavailable")
	}
	sanitized := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if sanitized == "" {
		return "", internal.Exchange("symbol service unavailable")
	}
	return d.symbols.canonicalForMarkets(context.Background(), sanitized)
}

func (d *wsDependencies) NativeSymbol(canonical string) (string, error) {
	if d.symbols == nil {
		return "", internal.Exchange("symbol service unavailable")
	}
	sanitized := strings.ToUpper(strings.TrimSpace(canonical))
	if sanitized == "" {
		return "", internal.Exchange("symbol service unavailable")
	}
	return d.symbols.nativeForMarkets(context.Background(), sanitized)
}

func (d *wsDependencies) NativeTopic(topic core.Topic) (string, error) {
	if d.exchangeName == "" {
		return "", internal.Exchange("topic translator unavailable")
	}
	return core.NativeTopic(d.exchangeName, topic)
}

func (d *wsDependencies) CreateListenKey(ctx context.Context) (string, error) {
	if d.listenKeys == nil {
		return "", internal.Exchange("listen key service unavailable")
	}
	return d.listenKeys.Create(ctx)
}

func (d *wsDependencies) KeepAliveListenKey(ctx context.Context, key string) error {
	if d.listenKeys == nil {
		return internal.Exchange("listen key service unavailable")
	}
	return d.listenKeys.KeepAlive(ctx, key)
}

func (d *wsDependencies) CloseListenKey(ctx context.Context, key string) error {
	if d.listenKeys == nil {
		return internal.Exchange("listen key service unavailable")
	}
	return d.listenKeys.Close(ctx, key)
}

func (d *wsDependencies) BookDepthSnapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	if d.depths == nil {
		return corestreams.BookEvent{}, 0, internal.Exchange("depth snapshot service unavailable")
	}
	return d.depths.Snapshot(ctx, symbol, limit)
}
