package binance

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

// orderBookSnapshotService fetches REST depth snapshots for symbols.
type orderBookSnapshotService struct {
	router  routingrest.RESTDispatcher
	symbols *symbolService
}

func newOrderBookSnapshotService(router routingrest.RESTDispatcher, symbols *symbolService) *orderBookSnapshotService {
	return &orderBookSnapshotService{router: router, symbols: symbols}
}

func (s *orderBookSnapshotService) Snapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	if s.router == nil {
		return corestreams.BookEvent{}, 0, internal.Invalid("depth snapshot: rest router unavailable")
	}
	native, err := s.symbols.nativeForMarkets(ctx, symbol, core.MarketSpot)
	if err != nil {
		return corestreams.BookEvent{}, 0, err
	}
	params := map[string]string{"symbol": native, "limit": fmt.Sprintf("%d", limit)}
	var resp struct {
		LastUpdateID int64           `json:"lastUpdateId"`
		Bids         [][]interface{} `json:"bids"`
		Asks         [][]interface{} `json:"asks"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/depth", Query: params}
	if err := s.router.Dispatch(ctx, msg, &resp); err != nil {
		return corestreams.BookEvent{}, 0, err
	}
	bids := convertDepthLevels(resp.Bids)
	asks := convertDepthLevels(resp.Asks)
	event := corestreams.BookEvent{Symbol: symbol, Bids: bids, Asks: asks, Time: time.Now()}
	return event, resp.LastUpdateID, nil
}

func convertDepthLevels(pairs [][]interface{}) []core.BookDepthLevel {
	levels := make([]core.BookDepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		var priceStr, qtyStr string
		switch v := pair[0].(type) {
		case string:
			priceStr = v
		default:
			priceStr = fmt.Sprint(v)
		}
		switch v := pair[1].(type) {
		case string:
			qtyStr = v
		default:
			qtyStr = fmt.Sprint(v)
		}
		price, _ := numeric.Parse(priceStr)
		qty, _ := numeric.Parse(qtyStr)
		levels = append(levels, core.BookDepthLevel{Price: price, Qty: qty})
	}
	return levels
}
