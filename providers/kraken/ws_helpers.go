package kraken

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
)

func appendDepthLevels(dst *[]core.DepthLevel, raw any) {
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
		*dst = append(*dst, core.DepthLevel{Price: price, Qty: qty})
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
