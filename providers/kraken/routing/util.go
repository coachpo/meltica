package routing

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
)

func appendDepthLevels(dst *[]core.BookDepthLevel, raw any) {
	levels, ok := raw.([]any)
	if !ok {
		return
	}
	for _, entry := range levels {
		priceStr := ""
		qtyStr := ""
		switch lvl := entry.(type) {
		case []any:
			if len(lvl) > 0 {
				priceStr = valueString(lvl[0])
			}
			if len(lvl) > 1 {
				qtyStr = valueString(lvl[1])
			}
		case map[string]any:
			priceStr = valueString(firstPresent(lvl, "price", "px", "bid", "ask"))
			qtyStr = valueString(firstPresent(lvl, "qty", "quantity", "volume", "size"))
		default:
			continue
		}
		if priceStr == "" {
			continue
		}
		price := parseDecimalStr(priceStr)
		qty := parseDecimalStr(qtyStr)
		*dst = append(*dst, core.BookDepthLevel{Price: price, Qty: qty})
	}
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func valueString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func parseISOTime(ts string) time.Time {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.UTC()
		}
	}
	return parseTradesTS(ts)
}

func parseTradesTS(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return time.Time{}
	}
	sec := int64(f)
	ns := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, ns).UTC()
}

func parseDecimal(s string) (*big.Rat, bool) {
	var out big.Rat
	if s == "" {
		return nil, false
	}
	if _, ok := out.SetString(s); !ok {
		return nil, false
	}
	return &out, true
}

func parseDecimalStr(s string) *big.Rat {
	r, _ := parseDecimal(s)
	return r
}

func mapStatus(state string) core.OrderStatus {
	switch state {
	case "pending", "open", "partial":
		return core.OrderNew
	case "filled", "closed":
		return core.OrderFilled
	case "canceled", "cancelled":
		return core.OrderCanceled
	case "rejected", "expired":
		return core.OrderRejected
	default:
		return core.OrderNew
	}
}

// CanonicalFromRequested attempts to match an exchange symbol to a requested topic symbol.
func CanonicalFromRequested(exch string, requested []string) string {
	candidates := []string{exch}
	if strings.Contains(exch, "/") {
		candidates = append(candidates, strings.ReplaceAll(exch, "/", "-"))
	} else if strings.Contains(exch, "-") {
		candidates = append(candidates, strings.ReplaceAll(exch, "-", "/"))
	}
	for _, t := range requested {
		parts := strings.Split(t, ":")
		if len(parts) != 2 {
			continue
		}
		sym := parts[1]
		if sym == "" {
			continue
		}
		for _, cand := range candidates {
			if cand != "" && strings.EqualFold(sym, cand) {
				return sym
			}
		}
		if strings.Contains(sym, "-") {
			alt := strings.ReplaceAll(sym, "-", "/")
			for _, cand := range candidates {
				if cand != "" && strings.EqualFold(alt, cand) {
					return sym
				}
			}
		}
	}
	if strings.Contains(exch, "/") {
		return strings.ReplaceAll(exch, "/", "-")
	}
	return exch
}
