package routing

import (
	"math/big"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
)

func parseDecimal(v string) *big.Rat {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	var r big.Rat
	if _, ok := r.SetString(v); !ok {
		return nil
	}
	return &r
}

func parseTime(val string) time.Time {
	if val == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func mapStatus(status, reason string) core.OrderStatus {
	switch strings.ToLower(status) {
	case "received", "open", "pending", "active":
		return core.OrderNew
	case "done", "settled":
		switch strings.ToLower(reason) {
		case "canceled":
			return core.OrderCanceled
		case "rejected":
			return core.OrderRejected
		default:
			return core.OrderFilled
		}
	default:
		return core.OrderNew
	}
}
