package ws

import (
	"math/big"
	"strconv"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

func parseMillis(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	if v, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return time.UnixMilli(v)
	}
	if f, err := strconv.ParseFloat(ts, 64); err == nil {
		ms := int64(f)
		return time.UnixMilli(ms)
	}
	return time.Time{}
}

func depthLevelsFromPairs(pairs [][]string) []corews.DepthLevel {
	levels := make([]corews.DepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		price, _ := parseDecimalToRat(pair[0])
		qty, _ := parseDecimalToRat(pair[1])
		levels = append(levels, corews.DepthLevel{Price: price, Qty: qty})
	}
	return levels
}

func parseDecimalToRat(s string) (*big.Rat, bool) {
	return core.ParseDecimalToRat(s)
}

func mapOKXStatus(s string) core.OrderStatus {
	switch s {
	case "live", "effective":
		return core.OrderNew
	case "partially-filled":
		return core.OrderPartFilled
	case "filled":
		return core.OrderFilled
	case "canceled":
		return core.OrderCanceled
	case "canceled-amend", "cancel_rejected", "order_failed":
		return core.OrderRejected
	default:
		return core.OrderNew
	}
}
