package binance

import (
	"fmt"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/internal/numeric"
)

func parseDepthLevels(pairs [][]interface{}) []core.BookDepthLevel {
	levels := make([]core.BookDepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		var pStr, qStr string
		switch v := pair[0].(type) {
		case string:
			pStr = v
		default:
			pStr = fmt.Sprint(v)
		}
		switch v := pair[1].(type) {
		case string:
			qStr = v
		default:
			qStr = fmt.Sprint(v)
		}
		price, _ := numeric.Parse(pStr)
		qty, _ := numeric.Parse(qStr)
		levels = append(levels, core.BookDepthLevel{Price: price, Qty: qty})
	}
	return levels
}
