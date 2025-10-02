package okx

import (
	"strconv"
	"time"

	"github.com/coachpo/meltica/core"
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

func depthLevelsFromPairs(pairs [][]string) []core.DepthLevel {
	levels := make([]core.DepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		price, _ := parseDecimalToRat(pair[0])
		qty, _ := parseDecimalToRat(pair[1])
		levels = append(levels, core.DepthLevel{Price: price, Qty: qty})
	}
	return levels
}
